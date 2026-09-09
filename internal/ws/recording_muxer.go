package ws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/mp4"
	"mywebscrcpy/internal/scrcpy"
)

const recordingTimescale = uint64(1_000_000) // scrcpy PTS uses microseconds.

type pendingSample struct {
	pts     uint64
	arrived time.Time
	data    []byte
	sync    bool
}

type mp4RecordingMuxer struct {
	file            *os.File
	config          []byte
	initialized     bool
	trackID         uint32
	audioTrackID    uint32
	needsAudio      bool
	audioConfig     []byte
	audioTimescale  uint64
	audioPending    *pendingSample
	audioDecodeTime uint64
	fragment        *mp4.Fragment
	sequence        uint32
	decodeTime      uint64
	fragmentDur     uint64
	pending         *pendingSample
}

func newMP4RecordingMuxer(path string, needsAudio bool) (*mp4RecordingMuxer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	return &mp4RecordingMuxer{file: f, needsAudio: needsAudio}, nil
}

func (m *mp4RecordingMuxer) Write(kind scrcpy.FrameKind, pts uint64, payload []byte, now time.Time) error {
	switch kind {
	case scrcpy.FrameConfig:
		if m.initialized {
			return errors.New("video encoder configuration changed during recording")
		}
		m.config = append([]byte(nil), payload...)
		return nil
	case scrcpy.FrameKey, scrcpy.FrameDelta:
		if !m.initialized {
			if kind != scrcpy.FrameKey {
				return nil
			}
			if m.needsAudio && len(m.audioConfig) == 0 {
				return nil
			}
			if err := m.initialize(); err != nil {
				return err
			}
		}
		data, err := annexBToAVCC(payload)
		if err != nil {
			return fmt.Errorf("invalid h264 access unit: %w", err)
		}
		if m.pending != nil {
			duration := pts - m.pending.pts
			if pts <= m.pending.pts {
				duration = 1
			}
			if err := m.addPending(duration); err != nil {
				return err
			}
		}
		m.pending = &pendingSample{pts: pts, arrived: now, data: data, sync: kind == scrcpy.FrameKey}
		return nil
	default:
		return nil
	}
}

// WriteAudio accepts AAC config/media packets from the shared scrcpy audio
// stream. Packets before the video keyframe are intentionally discarded: the
// recording starts at a decodable video boundary rather than producing a file
// with an arbitrary leading audio-only segment.
func (m *mp4RecordingMuxer) WriteAudio(config bool, pts uint64, payload []byte, now time.Time) error {
	if !m.needsAudio {
		return nil
	}
	if config {
		if m.initialized {
			return errors.New("audio encoder configuration changed during recording")
		}
		m.audioConfig = append([]byte(nil), payload...)
		return nil
	}
	if !m.initialized {
		return nil
	}
	if m.audioPending != nil {
		duration := pts - m.audioPending.pts
		if pts <= m.audioPending.pts {
			duration = 1
		}
		if err := m.addAudioPending(duration); err != nil {
			return err
		}
	}
	m.audioPending = &pendingSample{pts: pts, arrived: now, data: append([]byte(nil), payload...)}
	return nil
}

func (m *mp4RecordingMuxer) initialize() error {
	sps, pps, err := h264ParameterSets(m.config)
	if err != nil {
		return err
	}
	init := mp4.CreateEmptyInit()
	trak := init.AddEmptyTrack(uint32(recordingTimescale), "video", "und")
	if err := trak.SetAVCDescriptor("avc1", sps, pps, true); err != nil {
		return fmt.Errorf("create h264 descriptor: %w", err)
	}
	m.trackID = trak.Tkhd.TrackID
	if m.needsAudio {
		asc, err := aac.DecodeAudioSpecificConfig(bytes.NewReader(m.audioConfig))
		if err != nil {
			return fmt.Errorf("parse aac config: %w", err)
		}
		if asc.ChannelConfiguration != 2 {
			return fmt.Errorf("unsupported aac channel configuration: %d", asc.ChannelConfiguration)
		}
		audioTrak := init.AddEmptyTrack(uint32(asc.SamplingFrequency), "audio", "und")
		if err := audioTrak.SetAACDescriptor(asc.ObjectType, asc.SamplingFrequency); err != nil {
			return fmt.Errorf("create aac descriptor: %w", err)
		}
		m.audioTrackID = audioTrak.Tkhd.TrackID
		m.audioTimescale = uint64(asc.SamplingFrequency)
	}
	if err := init.Encode(m.file); err != nil {
		return fmt.Errorf("write mp4 header: %w", err)
	}
	m.initialized = true
	return nil
}

func (m *mp4RecordingMuxer) addPending(duration uint64) error {
	if duration == 0 {
		duration = 1
	}
	if duration > uint64(^uint32(0)) {
		return errors.New("h264 frame duration is too large")
	}
	if err := m.ensureFragment(); err != nil {
		return err
	}
	flags := mp4.NonSyncSampleFlags
	if m.pending.sync {
		flags = mp4.SyncSampleFlags
	}
	if err := m.fragment.AddFullSampleToTrack(mp4.FullSample{
		Sample:     mp4.Sample{Flags: flags, Dur: uint32(duration), Size: uint32(len(m.pending.data))},
		DecodeTime: m.decodeTime,
		Data:       m.pending.data,
	}, m.trackID); err != nil {
		return err
	}
	m.decodeTime += duration
	m.fragmentDur += duration
	if m.fragmentDur >= recordingTimescale {
		return m.flushFragment()
	}
	return nil
}

func (m *mp4RecordingMuxer) addAudioPending(duration uint64) error {
	if m.audioTimescale == 0 {
		return errors.New("audio timescale unavailable")
	}
	// Audio PTS are in microseconds while the AAC track timescale is samples/s.
	duration = duration * m.audioTimescale / recordingTimescale
	if duration == 0 {
		duration = 1
	}
	if duration > uint64(^uint32(0)) {
		return errors.New("aac frame duration is too large")
	}
	if err := m.ensureFragment(); err != nil {
		return err
	}
	if err := m.fragment.AddFullSampleToTrack(mp4.FullSample{Sample: mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: uint32(duration), Size: uint32(len(m.audioPending.data))}, DecodeTime: m.audioDecodeTime, Data: m.audioPending.data}, m.audioTrackID); err != nil {
		return err
	}
	m.audioDecodeTime += duration
	return nil
}

func (m *mp4RecordingMuxer) ensureFragment() error {
	if m.fragment != nil {
		return nil
	}
	m.sequence++
	trackIDs := []uint32{m.trackID}
	if m.needsAudio {
		trackIDs = append(trackIDs, m.audioTrackID)
	}
	fragment, err := mp4.CreateMultiTrackFragment(m.sequence, trackIDs)
	if err != nil {
		return err
	}
	m.fragment = fragment
	return nil
}

// rebasePendingPTS aligns a replayed bootstrap sample with the first live
// frame. The sample remains in the recording, but it only occupies the wall
// time spent waiting for that live frame.
func (m *mp4RecordingMuxer) rebasePendingPTS(pts uint64, now time.Time) {
	if m.pending == nil {
		return
	}
	elapsed := now.Sub(m.pending.arrived).Microseconds()
	if elapsed < 1 {
		elapsed = 1
	}
	if uint64(elapsed) >= pts {
		m.pending.pts = 0
		return
	}
	m.pending.pts = pts - uint64(elapsed)
}

func (m *mp4RecordingMuxer) flushFragment() error {
	if m.fragment == nil {
		return nil
	}
	if err := m.fragment.Encode(m.file); err != nil {
		return fmt.Errorf("write mp4 fragment: %w", err)
	}
	if err := m.file.Sync(); err != nil {
		return err
	}
	m.fragment = nil
	m.fragmentDur = 0
	return nil
}

func (m *mp4RecordingMuxer) Close(now time.Time) error {
	if m.file == nil {
		return nil
	}
	defer func() { _ = m.file.Close(); m.file = nil }()
	if !m.initialized || m.pending == nil {
		return errors.New("no decodable key frame received")
	}
	if m.needsAudio && m.audioPending == nil {
		return errors.New("no aac audio frame received")
	}
	duration := uint64(now.Sub(m.pending.arrived).Microseconds())
	if duration == 0 {
		duration = 1
	}
	if err := m.addPending(duration); err != nil {
		return err
	}
	if m.needsAudio {
		audioDuration := uint64(now.Sub(m.audioPending.arrived).Microseconds())
		if audioDuration == 0 {
			audioDuration = 1
		}
		if err := m.addAudioPending(audioDuration); err != nil {
			return err
		}
	}
	return m.flushFragment()
}

func h264ParameterSets(config []byte) ([][]byte, [][]byte, error) {
	var nalus [][]byte
	if isAnnexB(config) {
		nalus = splitAnnexBNALUs(config)
	} else {
		var err error
		nalus, err = avccNALUs(config)
		if err != nil {
			return nil, nil, err
		}
	}
	var sps, pps [][]byte
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		switch nalu[0] & 0x1f {
		case 7:
			sps = append(sps, append([]byte(nil), nalu...))
		case 8:
			pps = append(pps, append([]byte(nil), nalu...))
		}
	}
	if len(sps) == 0 || len(pps) == 0 {
		return nil, nil, errors.New("h264 config lacks SPS or PPS")
	}
	return sps, pps, nil
}

func annexBToAVCC(data []byte) ([]byte, error) {
	if !isAnnexB(data) {
		if _, err := avccNALUs(data); err != nil {
			return nil, err
		}
		return append([]byte(nil), data...), nil
	}
	nalus := splitAnnexBNALUs(data)
	if len(nalus) == 0 {
		return nil, errors.New("no h264 nal units")
	}
	result := make([]byte, 0, len(data))
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(nalu)))
		result = append(result, length[:]...)
		result = append(result, nalu...)
	}
	return result, nil
}

func avccNALUs(data []byte) ([][]byte, error) {
	var result [][]byte
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, errors.New("truncated nal length")
		}
		length := int(binary.BigEndian.Uint32(data[:4]))
		data = data[4:]
		if length <= 0 || length > len(data) {
			return nil, errors.New("invalid nal length")
		}
		result = append(result, data[:length])
		data = data[length:]
	}
	return result, nil
}

func splitAnnexBNALUs(data []byte) [][]byte {
	var result [][]byte
	start := -1
	for i := 0; i+3 < len(data); i++ {
		prefix := 0
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			prefix = 3
		} else if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			prefix = 4
		}
		if prefix == 0 {
			continue
		}
		if start >= 0 && start < i {
			result = append(result, data[start:i])
		}
		start = i + prefix
		i += prefix - 1
	}
	if start >= 0 && start < len(data) {
		result = append(result, data[start:])
	}
	return result
}
