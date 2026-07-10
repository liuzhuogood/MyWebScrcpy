package scrcpy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Frame 表示从 video socket 读出的一个单元。
type Frame struct {
	Kind    FrameKind // config / key / delta / session
	PTS     uint64    // 微秒 (config/session 时为 0)
	Payload []byte    // 媒体数据 (session 包时为 nil)
	// 仅 session 包填充：
	Width  uint32
	Height uint32
}

type FrameKind uint8

const (
	FrameConfig  FrameKind = 0
	FrameKey     FrameKind = 1
	FrameDelta   FrameKind = 2
	FrameSession FrameKind = 3
)

// Connection 管理到 scrcpy server 的两条 socket 连接。
type Connection struct {
	videoConn  net.Conn
	ctrlConn   net.Conn
	hasControl bool
	width      uint32
	height     uint32
	codecID    uint32
}

// Dial 连接到 scrcpy server。需要先完成 Forward。
// server 已经 Start。连接顺序固定：先 video，再 control（若开启）。
// 因为 forward 模式下设备端 accept 顺序也是 video 优先。
func Dial(localPort int, hasControl bool, timeout time.Duration) (*Connection, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)

	// 连 video socket
	videoConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial video socket: %w", err)
	}

	c := &Connection{videoConn: videoConn, hasControl: hasControl}

	// 阶段 1: 读 dummy byte (forward 模式，设备在第一个 accept 后立即发)
	if err := c.readDummyByte(); err != nil {
		videoConn.Close()
		return nil, err
	}

	// 阶段 2: 连 control socket。
	// 关键：server 在 accept 了 video + control 两个 socket 后才开始发 codec id。
	// 所以必须在读完 dummy byte 后、读 codec id 之前连上 control。
	if hasControl {
		ctrlConn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			videoConn.Close()
			return nil, fmt.Errorf("dial control socket: %w", err)
		}
		c.ctrlConn = ctrlConn
	}

	// 阶段 3: 读 codec id + session packet (此时 server 已收到两个 socket，开始发数据)
	if err := c.readStreamHeader(); err != nil {
		if c.ctrlConn != nil {
			c.ctrlConn.Close()
		}
		videoConn.Close()
		return nil, err
	}

	return c, nil
}

// readDummyByte 读取 forward 模式的 dummy byte (1 字节 0x00)。
func (c *Connection) readDummyByte() error {
	c.videoConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer c.videoConn.SetReadDeadline(time.Time{})

	dummy := make([]byte, 1)
	if _, err := io.ReadFull(c.videoConn, dummy); err != nil {
		return fmt.Errorf("read dummy byte: %w", err)
	}
	return nil
}

// readStreamHeader 读取 codec id (4B) + session packet (12B)。
// 必须在 control socket 连接后调用。
func (c *Connection) readStreamHeader() error {
	c.videoConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer c.videoConn.SetReadDeadline(time.Time{})

	// codec id (4 字节 u32 BE)
	var codecBuf [4]byte
	if _, err := io.ReadFull(c.videoConn, codecBuf[:]); err != nil {
		return fmt.Errorf("read codec id: %w", err)
	}
	c.codecID = binary.BigEndian.Uint32(codecBuf[:])
	if c.codecID == 0 {
		return errors.New("device disabled video stream (codec id=0)")
	}
	if c.codecID == 1 {
		return errors.New("device configuration error (codec id=1)")
	}

	// session packet (12 字节)
	hdr := make([]byte, PacketHeaderSize)
	if _, err := io.ReadFull(c.videoConn, hdr); err != nil {
		return fmt.Errorf("read session packet: %w", err)
	}
	return c.parseSessionHeader(hdr)
}

// parseSessionHeader 解析 session packet 头。
// 12 字节头整体作为 pts+flags(u64) + size(u32)，session 包时 u64 的 bit63=1，
// 且后面 8 字节实际是 width(u32)+height(u32) 而非 payload size。
func (c *Connection) parseSessionHeader(hdr []byte) error {
	ptsFlags := binary.BigEndian.Uint64(hdr[0:8])
	if ptsFlags&FlagSessionPacket == 0 {
		// 不是 session packet，协议异常
		return fmt.Errorf("expected session packet, got flags=0x%x", ptsFlags)
	}
	c.width = binary.BigEndian.Uint32(hdr[4:8])
	c.height = binary.BigEndian.Uint32(hdr[8:12])
	return nil
}

// CodecID 返回握手得到的 codec id。
func (c *Connection) CodecID() uint32 { return c.codecID }

// Size 返回视频分辨率。
func (c *Connection) Size() (uint32, uint32) { return c.width, c.height }

// SetSize 更新分辨率（处理中途的 session packet 旋转）。
func (c *Connection) SetSize(w, h uint32) { c.width, c.height = w, h }

// ReadFrame 从 video socket 读取下一帧。阻塞直到有帧或连接断开。
// 设置 read deadline 超时，用于检测旋转等导致的编码器重启卡死。
func (c *Connection) ReadFrame() (*Frame, error) {
	hdr := make([]byte, PacketHeaderSize)
	for {
		// 每次 read 设 10 秒超时，防止旋转编码器重启时永久阻塞
		c.videoConn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, err := io.ReadFull(c.videoConn, hdr)
		if err != nil {
			return nil, err
		}
		ptsFlags := binary.BigEndian.Uint64(hdr[0:8])
		pktLen := binary.BigEndian.Uint32(hdr[8:12])

		// session packet (旋转/尺寸变化)，无 payload
		if ptsFlags&FlagSessionPacket != 0 {
			w := binary.BigEndian.Uint32(hdr[4:8])
			h := binary.BigEndian.Uint32(hdr[8:12])
			c.width, c.height = w, h
			return &Frame{Kind: FrameSession, Width: w, Height: h}, nil
		}

		if pktLen == 0 {
			return nil, errors.New("invalid packet size 0")
		}
		payload := make([]byte, pktLen)
		if _, err := io.ReadFull(c.videoConn, payload); err != nil {
			return nil, err
		}

		f := &Frame{Payload: payload}
		switch {
		case ptsFlags&FlagConfigPacket != 0:
			f.Kind = FrameConfig
			f.PTS = 0
		default:
			f.PTS = ptsFlags & PTSMask
			if ptsFlags&FlagKeyFrame != 0 {
				f.Kind = FrameKey
			} else {
				f.Kind = FrameDelta
			}
		}
		return f, nil
	}
}

// WriteControl 向 control socket 写入控制消息（已打包的字节）。
func (c *Connection) WriteControl(b []byte) error {
	if c.ctrlConn == nil {
		return errors.New("control socket not available")
	}
	_, err := c.ctrlConn.Write(b)
	return err
}

// Close 关闭两条连接。
func (c *Connection) Close() {
	if c.videoConn != nil {
		c.videoConn.Close()
	}
	if c.ctrlConn != nil {
		c.ctrlConn.Close()
	}
}
