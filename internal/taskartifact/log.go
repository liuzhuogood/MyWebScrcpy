package taskartifact

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const fragmentSize = 16 * 1024

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type LogLine struct {
	TaskID    string `json:"task_id"`
	Seq       int64  `json:"seq"`
	AtMS      int64  `json:"at_ms"`
	Stream    string `json:"stream"`
	Message   string `json:"message"`
	Continued bool   `json:"continued"`
}
type LogQuery struct {
	After, Before       int64
	HasAfter, HasBefore bool
	Limit               int
	Stream, Q           string
}
type LogPage struct {
	Items        []LogLine `json:"items"`
	NextCursor   int64     `json:"next_cursor"`
	HasMore      bool      `json:"has_more"`
	Complete     bool      `json:"complete"`
	Truncated    bool      `json:"truncated"`
	Availability string    `json:"availability"`
	OldestSeq    int64     `json:"oldest_seq"`
	LatestSeq    int64     `json:"latest_seq"`
}
type logIndex struct{ Seq, Offset int64 }
type LogStore struct {
	mu                        sync.Mutex
	dir                       string
	perRun, totalLimit, total int64
	writers                   map[string]*LogCapture
}
type LogCapture struct {
	mu           sync.Mutex
	store        *LogStore
	id           string
	file, index  *os.File
	state        LogState
	closed       bool
	notify       func(LogState)
	streams      []*logWriter
	lastPublish  time.Time
	publishedSeq int64
}
type logWriter struct {
	capture *LogCapture
	stream  string
	mu      sync.Mutex
	pending []byte
}

func NewLogStore(dir string, perRun, total int64) (*LogStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &LogStore{dir: dir, perRun: perRun, totalLimit: total, writers: map[string]*LogCapture{}}
	files, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".jsonl") {
			if i, e := f.Info(); e == nil {
				s.total += i.Size()
			}
		}
	}
	return s, nil
}
func (s *LogStore) path(id, ext string) (string, error) {
	if !safeID.MatchString(id) {
		return "", fmt.Errorf("无效任务 ID")
	}
	return filepath.Join(s.dir, id+ext), nil
}
func (s *LogStore) Create(id string, notify func(LogState)) (*LogCapture, error) {
	path, err := s.path(id, ".jsonl")
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	idx, err := os.OpenFile(path+".idx", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	w := &LogCapture{store: s, id: id, file: f, index: idx, notify: notify, state: LogState{Availability: "available", CaptureState: "waiting"}}
	s.mu.Lock()
	s.writers[id] = w
	s.mu.Unlock()
	w.publish()
	if w.state.CaptureState == "degraded" {
		w.Close()
		os.Remove(path)
		os.Remove(path + ".idx")
		return nil, fmt.Errorf("初始化日志状态失败")
	}
	return w, nil
}
func (w *LogCapture) Writer(stream string) io.Writer {
	writer := &logWriter{capture: w, stream: stream}
	w.mu.Lock()
	w.streams = append(w.streams, writer)
	w.mu.Unlock()
	return writer
}

// Write fragments every received chunk, including unterminated lines; no unbounded line buffer.
func (w *logWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		p = append(w.pending, p...)
		w.pending = nil
	}
	// Keep at most one incomplete UTF-8 rune between OS pipe reads.
	endValid := len(p)
	for endValid > 0 && len(p)-endValid < 3 && !utf8.RuneStart(p[endValid-1]) {
		endValid--
	}
	if endValid > 0 {
		endValid--
	}
	if endValid < len(p) && !utf8.FullRune(p[endValid:]) {
		w.pending = append([]byte(nil), p[endValid:]...)
		p = p[:endValid]
	}
	for len(p) > 0 {
		end := bytes.IndexByte(p, '\n')
		newline := end >= 0
		if end < 0 {
			end = len(p)
		}
		continued := !newline
		if end > fragmentSize {
			end = fragmentSize
			for end > 0 && !utf8.RuneStart(p[end]) {
				end--
			}
			continued = true
			newline = false
		}
		if end == 0 && !newline {
			end = 1
		}
		w.capture.append(w.stream, strings.ToValidUTF8(string(bytes.TrimSuffix(p[:end], []byte{'\r'})), "�"), continued)
		p = p[end:]
		if newline {
			p = p[1:]
		}
	}
	return n, nil // Disk failures are evidence degradation, never a process pipe failure.
}
func (w *LogCapture) append(stream, message string, continued bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.state.CaptureState == "degraded" || w.state.Truncated {
		return
	}
	line := LogLine{TaskID: w.id, Seq: w.state.LastSeq + 1, AtMS: time.Now().UnixMilli(), Stream: stream, Message: message, Continued: continued}
	data, err := json.Marshal(line)
	if err != nil {
		return
	}
	data = append(data, '\n')
	w.store.mu.Lock()
	allowed := w.state.Bytes+int64(len(data)) <= w.store.perRun && w.store.total+int64(len(data)) <= w.store.totalLimit
	if allowed {
		w.store.total += int64(len(data))
	}
	w.store.mu.Unlock()
	if !allowed {
		if !w.state.Truncated {
			w.state.Truncated = true
			w.publish()
		}
		return
	}
	if (line.Seq-1)%128 == 0 {
		b, _ := json.Marshal(logIndex{Seq: line.Seq, Offset: w.state.Bytes})
		if _, err = w.index.Write(append(b, '\n')); err != nil {
			w.store.mu.Lock()
			w.store.total -= int64(len(data))
			w.store.mu.Unlock()
			w.degrade(err)
			return
		}
	}
	count, err := w.file.Write(data)
	if err == nil && count != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.file.Truncate(w.state.Bytes)
		w.store.mu.Lock()
		w.store.total -= int64(len(data))
		w.store.mu.Unlock()
		w.degrade(err)
		return
	}
	w.state.Bytes += int64(count)
	w.state.LastSeq = line.Seq
	w.state.CaptureState = "running"
	if stream == "system" || w.state.LastSeq-w.publishedSeq >= 128 || time.Since(w.lastPublish) >= 200*time.Millisecond {
		w.publish()
	}
}
func (w *LogCapture) Event(message string) { w.append("system", message, false) }
func (w *LogCapture) degrade(err error) {
	w.state.CaptureState = "degraded"
	w.state.ErrorMsg = "日志保存失败: " + err.Error()
	w.publish()
}
func (w *LogCapture) publish() {
	w.lastPublish = time.Now()
	w.publishedSeq = w.state.LastSeq
	b, _ := json.Marshal(w.state)
	path, _ := w.store.path(w.id, ".state.json")
	if err := writeLogState(path, b); err != nil {
		w.state.CaptureState = "degraded"
		w.state.ErrorMsg = "日志状态保存失败"
	}
	if w.notify != nil {
		w.notify(w.state)
	}
}
func (w *LogCapture) Close() {
	w.mu.Lock()
	streams := append([]*logWriter(nil), w.streams...)
	w.mu.Unlock()
	for _, stream := range streams {
		stream.mu.Lock()
		if len(stream.pending) > 0 {
			w.append(stream.stream, strings.ToValidUTF8(string(stream.pending), "�"), true)
			stream.pending = nil
		}
		stream.mu.Unlock()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	if err := w.file.Sync(); err != nil {
		w.degrade(err)
	}
	w.file.Close()
	w.index.Close()
	if w.state.CaptureState != "degraded" {
		w.state.CaptureState = "completed"
	}
	expires := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	w.state.ExpiresAt = &expires
	w.publish()
	w.store.mu.Lock()
	delete(w.store.writers, w.id)
	w.store.mu.Unlock()
}
func (s *LogStore) Status(id string) (LogState, error) {
	path, e := s.path(id, ".state.json")
	if e != nil {
		return LogState{}, e
	}
	s.mu.Lock()
	w := s.writers[id]
	s.mu.Unlock()
	if w != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.state, nil
	}
	data, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return LogState{Availability: "unavailable", CaptureState: "completed"}, nil
	}
	if e != nil {
		return LogState{}, e
	}
	var state LogState
	e = json.Unmarshal(data, &state)
	if e != nil {
		return state, e
	}
	if state.ExpiresAt != nil && time.Now().UnixMilli() >= *state.ExpiresAt {
		state.Availability = "expired"
	}
	return state, nil
}
func (s *LogStore) Read(id string, q LogQuery) (LogPage, error) {
	state, e := s.Status(id)
	page := LogPage{Items: []LogLine{}, Availability: state.Availability, Truncated: state.Truncated, Complete: state.CaptureState == "completed" || state.CaptureState == "degraded", LatestSeq: state.LastSeq}
	if q.HasAfter && q.HasBefore {
		return page, fmt.Errorf("after 和 before 不能同时使用")
	}
	if q.Limit == 0 {
		q.Limit = 200
	}
	if q.Limit < 1 || q.Limit > 500 {
		return page, fmt.Errorf("limit 必须为 1 到 500")
	}
	if q.Stream != "" && q.Stream != "all" && q.Stream != "stdout" && q.Stream != "stderr" && q.Stream != "system" {
		return page, fmt.Errorf("无效日志来源")
	}
	if utf8.RuneCountInString(q.Q) > 200 {
		return page, fmt.Errorf("搜索最多 200 字")
	}
	if e != nil || state.Availability != "available" {
		return page, e
	}
	lower, upper := int64(1), state.LastSeq
	backward := !q.HasAfter
	if q.HasAfter && q.After >= state.LastSeq {
		page.NextCursor = q.After
		return page, nil
	}
	if q.HasAfter {
		lower = q.After + 1
		page.NextCursor = q.After
	}
	if q.HasBefore {
		upper = min(upper, q.Before-1)
		page.NextCursor = q.Before
	}
	// A bounded scan advances even if every row was filtered out.
	if backward {
		lower = max(lower, upper-1999)
	} else {
		upper = min(upper, lower+1999)
	}
	path, _ := s.path(id, ".jsonl")
	f, e := os.Open(path)
	if e != nil {
		return page, e
	}
	defer f.Close()
	indexes, _ := os.ReadFile(path + ".idx")
	offset := int64(0)
	for _, b := range bytes.Split(indexes, []byte{'\n'}) {
		var idx logIndex
		if json.Unmarshal(b, &idx) == nil && idx.Seq <= lower {
			offset = idx.Offset
		}
	}
	f.Seek(offset, io.SeekStart)
	scanner := bufio.NewScanner(io.LimitReader(f, max(0, state.Bytes-offset)))
	scanner.Buffer(make([]byte, 64*1024), 128*1024)
	scanned := []LogLine{}
	for scanner.Scan() {
		var line LogLine
		if e = json.Unmarshal(scanner.Bytes(), &line); e != nil {
			return page, fmt.Errorf("日志损坏: %w", e)
		}
		if line.Seq < lower {
			continue
		}
		if line.Seq > upper {
			break
		}
		scanned = append(scanned, line)
	}
	if e = scanner.Err(); e != nil {
		return page, e
	}
	if backward {
		sort.Slice(scanned, func(i, j int) bool { return scanned[i].Seq > scanned[j].Seq })
	}
	for _, line := range scanned {
		page.NextCursor = line.Seq
		if (q.Stream == "" || q.Stream == "all" || q.Stream == line.Stream) && strings.Contains(strings.ToLower(line.Message), strings.ToLower(q.Q)) {
			page.Items = append(page.Items, line)
			if len(page.Items) >= q.Limit {
				break
			}
		}
	}
	if backward {
		page.HasMore = page.NextCursor > 1 && len(scanned) > 0
		sort.Slice(page.Items, func(i, j int) bool { return page.Items[i].Seq < page.Items[j].Seq })
	} else {
		page.HasMore = page.NextCursor < state.LastSeq
	}
	if len(page.Items) > 0 {
		page.OldestSeq = page.Items[0].Seq
	}
	return page, nil
}
func (s *LogStore) Open(id string) (*os.File, error) {
	state, e := s.Status(id)
	if e != nil {
		return nil, e
	}
	if state.Availability != "available" {
		return nil, fmt.Errorf("日志%s", state.Availability)
	}
	path, _ := s.path(id, ".jsonl")
	return os.Open(path)
}

// Cleanup only removes expired completed logs; metadata remains to distinguish expired from legacy.
func (s *LogStore) Cleanup() {
	files, _ := os.ReadDir(s.dir)
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".state.json") {
			continue
		}
		id := strings.TrimSuffix(f.Name(), ".state.json")
		state, e := s.Status(id)
		if e != nil || state.Availability != "expired" {
			continue
		}
		path, _ := s.path(id, ".jsonl")
		if info, e := os.Stat(path); e == nil {
			if os.Remove(path) == nil {
				s.mu.Lock()
				s.total -= info.Size()
				s.mu.Unlock()
			}
		}
		os.Remove(path + ".idx")
	}
}

func (s *LogStore) Recover(id string) {
	path, e := s.path(id, ".state.json")
	if e != nil {
		return
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return
	}
	var st LogState
	if json.Unmarshal(data, &st) != nil {
		return
	}
	if st.CaptureState == "running" || st.CaptureState == "waiting" {
		filePath, _ := s.path(id, ".jsonl")
		if file, e := os.OpenFile(filePath, os.O_WRONLY, 0600); e == nil {
			file.Truncate(st.Bytes)
			file.Close()
		}
		st.CaptureState = "degraded"
		st.ErrorMsg = "服务重启，日志可能不完整"
		expires := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
		st.ExpiresAt = &expires
		b, _ := json.Marshal(st)
		writeLogState(path, b)
	}
}

func writeLogState(path string, data []byte) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, data, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
