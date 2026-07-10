package scrcpy

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"os/exec"
)

// ServerConfig 是启动一个 scrcpy server 实例所需的参数。
type ServerConfig struct {
	Serial    string // adb 设备序列号，如 10.0.0.104:5555
	MaxSize   int    // 最大边长像素，0=不变换
	BitRate   int    // 视频码率 bps
	MaxFPS    int    // 最大帧率，0=不限
	Codec     string // h264 / h265 / av1
	Control   bool   // 是否开启控制通道
}

// DefaultConfig 返回适合 web 投屏 + 操作的默认参数。
func DefaultConfig(serial string) ServerConfig {
	return ServerConfig{
		Serial:  serial,
		MaxSize: 1024,
		BitRate: 2_000_000,
		MaxFPS:  15,
		Codec:   "h264",
		Control: true,
	}
}

// Server 代表一个正在运行（或将要运行）的 scrcpy server 实例。
type Server struct {
	cfg        ServerConfig
	scid       uint32   // 31 位随机 id
	scidHex    string   // 8 位十六进制
	socketName string   // localabstract socket 名: scrcpy_<hex>
	forwardSpec string  // adb forward 的 remote spec
	localPort  int      // adb forward 的本地端口
	adbPath    string
	cmd        *exec.Cmd // app_process 后台进程
}

// NewServer 创建一个 server 实例（不启动）。
// localPort 指定 adb forward 的本地端口，每个 session 必须用不同端口。
func NewServer(cfg ServerConfig, adbPath string, localPort int) *Server {
	s := &Server{cfg: cfg, adbPath: adbPath, localPort: localPort}
	s.generateSCID()
	return s
}

func (s *Server) generateSCID() {
	var b [4]byte
	rand.Read(b[:])
	s.scid = binary.BigEndian.Uint32(b[:]) & 0x7fffffff // 31 位
	s.scidHex = fmt.Sprintf("%08x", s.scid)
	s.socketName = "scrcpy_" + s.scidHex
	s.forwardSpec = "localabstract:" + s.socketName
}

// SCIDHex 返回 scid 的 8 位十六进制字符串。
func (s *Server) SCIDHex() string { return s.scidHex }

// Push 将 server.jar 推送到设备。
func (s *Server) Push(jarPath string) error {
	cmd := exec.Command(s.adbPath, "-s", s.cfg.Serial, "push", jarPath, DeviceServerPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("push server jar: %w: %s", err, out)
	}
	// 验证设备端 jar 存在
	verify := exec.Command(s.adbPath, "-s", s.cfg.Serial, "shell",
		"ls -la "+DeviceServerPath)
	vout, verr := verify.CombinedOutput()
	if verr != nil {
		return fmt.Errorf("verify jar: %w: %s", verr, vout)
	}
	log.Printf("[scrcpy] push 完成, 设备端: %s", string(vout))
	return nil
}

// Forward 建立 adb forward 隧道: 本地端口 -> 设备 localabstract socket。
func (s *Server) Forward() error {
	local := fmt.Sprintf("tcp:%d", s.localPort)
	cmd := exec.Command(s.adbPath, "-s", s.cfg.Serial, "forward", local, s.forwardSpec)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb forward: %w: %s", err, out)
	}
	return nil
}

// RemoveForward 移除 forward 隧道。
func (s *Server) RemoveForward() {
	exec.Command(s.adbPath, "-s", s.cfg.Serial, "forward", "--remove",
		fmt.Sprintf("tcp:%d", s.localPort)).Run()
}

// Start 在设备上后台启动 scrcpy server 进程 (app_process)。
// 返回的 cmd 是 adb shell 子进程；进程退出通常意味着 server 结束。
func (s *Server) Start() error {
	controlStr := "false"
	if s.cfg.Control {
		controlStr = "true"
	}

	args := []string{
		"-s", s.cfg.Serial, "shell",
		"CLASSPATH=" + DeviceServerPath,
		"app_process", "/", "com.genymobile.scrcpy.Server", ServerVersion,
		"scid=" + s.scidHex,
		"log_level=warn",
		"video=true",
		"audio=false",
		"tunnel_forward=true",
		"control=" + controlStr,
		"video_codec=" + s.cfg.Codec,
		"send_dummy_byte=true",
		"send_device_meta=false",
		"send_stream_meta=true",
		"send_frame_meta=true",
		fmt.Sprintf("max_size=%d", s.cfg.MaxSize),
		fmt.Sprintf("video_bit_rate=%d", s.cfg.BitRate),
		fmt.Sprintf("max_fps=%d", s.cfg.MaxFPS),
	}

	s.cmd = exec.Command(s.adbPath, args...)
	// 把 server 的 stdout/stderr 都收集起来，便于排查
	lw := logWriter{prefix: "[scrcpy:" + s.cfg.Serial + "] "}
	s.cmd.Stdout = lw
	s.cmd.Stderr = lw

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("start app_process: %w", err)
	}

	// 后台监控 server 进程退出，打印退出码
	go func() {
		err := s.cmd.Wait()
		log.Printf("[scrcpy:%s] server 进程退出: %v", s.cfg.Serial, err)
	}()
	return nil
}

// Stop 终止 server 进程。
func (s *Server) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}

// Wait 等待 server 进程退出。
func (s *Server) Wait() error {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Wait()
}

// LocalPort 返回 forward 的本地端口。
func (s *Server) LocalPort() int { return s.localPort }

// logWriter 把 server 进程的输出写到标准 log，带前缀。
type logWriter struct{ prefix string }

func (w logWriter) Write(p []byte) (int, error) {
	log.Printf("%s%s", w.prefix, string(p))
	return len(p), nil
}
