package ws

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/visionframes"
)

const (
	defaultStreamLeaseTTL = 15 * time.Minute
)

type streamLease struct {
	serial    string
	cancel    func()
	stop      chan struct{}
	stopped   chan struct{}
	expiresAt time.Time
	timer     *time.Timer
}

// StreamKeeper 维护设备 scrcpy 视频流的服务端自主保活与解码（默认保活 15 分钟，请求自动刷新）。
type StreamKeeper struct {
	hub    *Hub
	frames *visionframes.Store
	mu     sync.Mutex
	leases map[string]*streamLease
	closed bool
}

// NewStreamKeeper 创建流保活管理器。
func NewStreamKeeper(hub *Hub, frames *visionframes.Store) *StreamKeeper {
	return &StreamKeeper{
		hub:    hub,
		frames: frames,
		leases: make(map[string]*streamLease),
	}
}

// Touch 激活或延长指定设备的视频流保活租约（默认 15 分钟）。
func (k *StreamKeeper) Touch(serial string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = defaultStreamLeaseTTL
	}
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return fmt.Errorf("StreamKeeper 已关闭")
	}
	if lease, exists := k.leases[serial]; exists {
		select {
		case <-lease.stopped:
			// 旧租约底层流已结束，清理后重新启动
			if lease.timer != nil {
				lease.timer.Stop()
			}
			delete(k.leases, serial)
		default:
			lease.expiresAt = time.Now().Add(ttl)
			if lease.timer != nil {
				lease.timer.Stop()
				lease.timer.Reset(ttl)
			}
			k.mu.Unlock()
			return nil
		}
	}
	k.mu.Unlock()

	// 首次启动服务端设备流：在锁外执行网络连接和 scrcpy 握手
	if strings.Contains(serial, ":") && k.hub != nil && k.hub.adbPath != "" {
		connCtx, connCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = dependencies.CommandContext(connCtx, k.hub.adbPath, "connect", serial).Run()
		connCancel()
	}

	ms, _, cancel, err := k.hub.acquireSession(serial)
	if err != nil {
		return fmt.Errorf("启动服务端视频流失败: %w", err)
	}

	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		cancel()
		return fmt.Errorf("StreamKeeper 已关闭")
	}
	if lease, exists := k.leases[serial]; exists {
		select {
		case <-lease.stopped:
			delete(k.leases, serial)
		default:
			lease.expiresAt = time.Now().Add(ttl)
			if lease.timer != nil {
				lease.timer.Stop()
				lease.timer.Reset(ttl)
			}
			k.mu.Unlock()
			cancel()
			return nil
		}
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	l := &streamLease{
		serial:    serial,
		cancel:    cancel,
		stop:      stop,
		stopped:   stopped,
		expiresAt: time.Now().Add(ttl),
	}
	l.timer = time.AfterFunc(ttl, func() {
		k.expire(serial)
	})
	k.leases[serial] = l
	k.mu.Unlock()

	log.Printf("[StreamKeeper] 启动视频流自主保活 serial=%s ttl=%v", serial, ttl)
	go k.runStream(serial, ms, l)
	return nil
}

func (k *StreamKeeper) expire(serial string) {
	k.mu.Lock()
	l, exists := k.leases[serial]
	if !exists {
		k.mu.Unlock()
		return
	}
	if time.Now().Before(l.expiresAt) {
		remaining := time.Until(l.expiresAt)
		l.timer = time.AfterFunc(remaining, func() {
			k.expire(serial)
		})
		k.mu.Unlock()
		return
	}
	delete(k.leases, serial)
	k.mu.Unlock()

	log.Printf("[StreamKeeper] 视频流保活到期释放 serial=%s", serial)
	close(l.stop)
	l.cancel()
}

// IsActive 查询设备是否处于保活租约中且底层流未退出。
func (k *StreamKeeper) IsActive(serial string) bool {
	if k == nil {
		return false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	l, exists := k.leases[serial]
	if !exists {
		return false
	}
	select {
	case <-l.stopped:
		return false
	default:
		return true
	}
}

// ActiveSerials 返回当前所有处于保活期内的设备序列号列表。
func (k *StreamKeeper) ActiveSerials() []string {
	if k == nil {
		return nil
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	serials := make([]string, 0, len(k.leases))
	for s := range k.leases {
		serials = append(serials, s)
	}
	return serials
}

// EnsureFrame 确保设备视频流保活并在 maxWait 内返回最新帧。
func (k *StreamKeeper) EnsureFrame(ctx context.Context, serial string, maxWait time.Duration) (visionframes.Frame, error) {
	if err := k.Touch(serial, defaultStreamLeaseTTL); err != nil {
		return visionframes.Frame{}, err
	}
	if k.frames == nil {
		return visionframes.Frame{}, fmt.Errorf("frames store 未初始化")
	}

	// 1. 若 5 秒内已有最新帧，立即返回
	if f, ok := k.frames.Latest(serial, 5*time.Second); ok {
		return f, nil
	}

	// 2. 轮询等待新帧到达
	ticker := time.NewTicker(60 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(maxWait)

	for {
		select {
		case <-ctx.Done():
			return visionframes.Frame{}, ctx.Err()
		case <-ticker.C:
			if f, ok := k.frames.Latest(serial, 5*time.Second); ok {
				return f, nil
			}
			if time.Now().After(deadline) {
				return visionframes.Frame{}, fmt.Errorf("等待视频流画面帧超时")
			}
		}
	}
}

func (k *StreamKeeper) runStream(serial string, ms *managedSession, l *streamLease) {
	defer func() {
		close(l.stopped)
		k.mu.Lock()
		if cur, exists := k.leases[serial]; exists && cur == l {
			if l.timer != nil {
				l.timer.Stop()
			}
			delete(k.leases, serial)
			log.Printf("[StreamKeeper] 视频流底层连接结束，已清理保活租约 serial=%s", serial)
		}
		k.mu.Unlock()
	}()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("[StreamKeeper] 未找到系统 ffmpeg，将由匹配服务 worker 或客户端补充推帧 serial=%s", serial)
		<-l.stop
		return
	}
	k.runFFmpeg(serial, ms, l, ffmpegPath)
}

func (k *StreamKeeper) runFFmpeg(serial string, ms *managedSession, l *streamLease, ffmpegPath string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-flags", "low_delay",
		"-fflags", "nobuffer",
		"-threads", "1",
		"-f", "h264",
		"-i", "pipe:0",
		"-vf", "fps=2",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[StreamKeeper] ffmpeg stdin pipe error: %v", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		log.Printf("[StreamKeeper] ffmpeg stdout pipe error: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		log.Printf("[StreamKeeper] ffmpeg 启动失败: %v", err)
		return
	}

	// 异步读取 stdout，按 JPEG 头尾特征切分单帧
	go func() {
		defer stdout.Close()
		var cur []byte
		b := make([]byte, 16*1024)
		for {
			n, err := stdout.Read(b)
			if n > 0 {
				cur = append(cur, b[:n]...)
				for {
					soi := bytes.Index(cur, []byte{0xFF, 0xD8})
					if soi < 0 {
						if len(cur) > 1 {
							cur = cur[len(cur)-1:]
						}
						break
					}
					if soi > 0 {
						cur = cur[soi:]
					}
					eoi := bytes.Index(cur[2:], []byte{0xFF, 0xD9})
					if eoi < 0 {
						break
					}
					end := 2 + eoi + 2
					jpegBytes := make([]byte, end)
					copy(jpegBytes, cur[:end])
					cur = cur[end:]
					if k.frames != nil {
						_, _ = k.frames.Put(serial, jpegBytes)
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	ch, unsubscribe := k.hub.subscribeShared(ms)
	defer unsubscribe()
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	var lastSent time.Time
	for {
		select {
		case <-l.stop:
			return
		case frame, ok := <-ch:
			if !ok {
				return
			}
			if len(frame.data) < 9 {
				continue
			}
			kind := frame.data[0]
			now := time.Now()
			if !shouldEmitVisionFrame(kind, 5, lastSent, now) {
				continue
			}
			lastSent = now
			annexB, err := avccToAnnexB(frame.data[9:])
			if err != nil || len(annexB) == 0 {
				continue
			}
			if _, err := stdin.Write(annexB); err != nil {
				return
			}
		}
	}
}

// Close 停止并清理所有保活租约。
func (k *StreamKeeper) Close() {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return
	}
	k.closed = true
	allLeases := make([]*streamLease, 0, len(k.leases))
	for _, l := range k.leases {
		allLeases = append(allLeases, l)
	}
	k.leases = make(map[string]*streamLease)
	k.mu.Unlock()

	for _, l := range allLeases {
		if l.timer != nil {
			l.timer.Stop()
		}
		close(l.stop)
		l.cancel()
	}
}
