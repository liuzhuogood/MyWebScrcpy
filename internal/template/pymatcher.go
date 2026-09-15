package template

import (
	"bufio"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

//go:embed pymatcher.py
var pyMatcherScript []byte

// PythonMatchResult mirrors the JSON emitted by pymatcher.py. Coordinates are
// normalized (0..1), matching the existing MatchResult shape.
type PythonMatchResult struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

type pyResultMessage struct {
	DeviceID string              `json:"device_id"`
	FrameID  uint64              `json:"frame_id"`
	Matches  []PythonMatchResult `json:"matches"`
}

// frameMsg is one encoded scrcpy frame queued for the Python subprocess.
type frameMsg struct {
	serial  string
	kind    uint8
	frameID uint64
	payload []byte
}

// PyMatcher manages a long-lived Python subprocess that decodes H264 (incl.
// CABAC) with PyAV and runs OpenCV template matching against the on-disk
// template directory. Go forwards raw scrcpy frames over stdin and consumes
// JSON results from stdout.
//
// Frame submission is asynchronous and latest-frame-only: SubmitFrame never
// blocks the video fan-out, so a slow Python matcher cannot stall screen
// mirroring. When the queue fills, the oldest pending frame is dropped in
// favour of the newest.
type PyMatcher struct {
	templatesDir string
	onResult     func(serial string, frameID uint64, matches []MatchResult)

	sendCh chan frameMsg
	wg     sync.WaitGroup

	cmd    *exec.Cmd
	stdin  *os.File
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

// NewPyMatcher creates a matcher that will spawn a Python subprocess on Start.
func NewPyMatcher(templatesDir string, onResult func(serial string, frameID uint64, matches []MatchResult)) *PyMatcher {
	return &PyMatcher{
		templatesDir: templatesDir,
		onResult:     onResult,
		done:         make(chan struct{}),
	}
}

// Start spawns the Python subprocess and begins reading results. It returns
// an error if the script cannot be written or the process fails to launch.
func (p *PyMatcher) Start() error {
	scriptPath := p.writeScript()
	if scriptPath == "" {
		return errors.New("pymatcher: failed to write python script")
	}
	pythonBin := os.Getenv("MYWEBSCRCPY_PYTHON")
	if pythonBin == "" {
		pythonBin = "python3"
	}
	cmd := exec.Command(pythonBin, scriptPath, "--templates-dir", p.templatesDir)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("pymatcher: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pymatcher: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pymatcher: start python: %w", err)
	}
	p.cmd = cmd
	p.stdin = stdin.(*os.File)
	p.sendCh = make(chan frameMsg, 32)
	p.wg.Add(1)
	go p.writerLoop()
	go p.readLoop(stdout.(*os.File))
	go func() {
		<-p.done
		_ = cmd.Wait()
	}()
	log.Printf("[pymatcher] python 匹配进程已启动: %s", scriptPath)
	return nil
}

// writeScript writes the embedded python script to a temp file.
func (p *PyMatcher) writeScript() string {
	dir := os.TempDir()
	path := dir + "/mywebscrcpy_pymatcher.py"
	if err := os.WriteFile(path, pyMatcherScript, 0o755); err != nil {
		log.Printf("[pymatcher] 写脚本失败: %v", err)
		return ""
	}
	return path
}

// SubmitFrame queues one encoded scrcpy frame for the Python process. It never
// blocks the caller: config frames are force-sent (short timeout) so the
// decoder can initialize; ordinary frames use latest-frame semantics and drop
// the oldest queued frame when full.
func (p *PyMatcher) SubmitFrame(serial string, kind uint8, frameID uint64, payload []byte) {
	p.mu.Lock()
	if p.closed || p.stdin == nil {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	msg := frameMsg{serial: serial, kind: kind, frameID: frameID, payload: payload}
	if kind == 0 {
		// config frame must reach the decoder; wait briefly if needed.
		select {
		case p.sendCh <- msg:
		case <-time.After(100 * time.Millisecond):
		}
		return
	}
	// latest-frame: when full, drop the oldest pending frame.
	select {
	case p.sendCh <- msg:
	default:
		select {
		case <-p.sendCh:
		default:
		}
		select {
		case p.sendCh <- msg:
		default:
		}
	}
}

// writerLoop serializes queued frames onto the Python stdin pipe. A slow Python
// side may block here, but that never blocks ConsumeVideoFrame.
func (p *PyMatcher) writerLoop() {
	defer p.wg.Done()
	for {
		select {
		case msg := <-p.sendCh:
			p.writeFrame(msg)
		case <-p.done:
			return
		}
	}
}

func (p *PyMatcher) writeFrame(msg frameMsg) {
	serBytes := []byte(msg.serial)
	body := make([]byte, 4+len(serBytes)+1+8+len(msg.payload))
	binary.BigEndian.PutUint32(body[0:4], uint32(len(serBytes)))
	copy(body[4:], serBytes)
	off := 4 + len(serBytes)
	body[off] = msg.kind
	binary.BigEndian.PutUint64(body[off+1:off+9], msg.frameID)
	copy(body[off+9:], msg.payload)

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(body)))

	buf := make([]byte, 0, len(header)+len(body))
	buf = append(buf, header...)
	buf = append(buf, body...)
	if _, err := p.stdin.Write(buf); err != nil {
		log.Printf("[pymatcher] 写帧失败 serial=%s: %v", msg.serial, err)
	}
}

// readLoop consumes JSON result lines from the Python subprocess.
func (p *PyMatcher) readLoop(stdout *os.File) {
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg pyResultMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("[pymatcher] 解析结果失败: %v", err)
			continue
		}
		matches := make([]MatchResult, 0, len(msg.Matches))
		for _, m := range msg.Matches {
			matches = append(matches, MatchResult{
				Name:  m.Name,
				Score: m.Score,
				X:     m.X,
				Y:     m.Y,
				W:     m.W,
				H:     m.H,
			})
		}
		if p.onResult != nil {
			p.onResult(msg.DeviceID, msg.FrameID, matches)
		}
	}
	log.Printf("[pymatcher] 结果读取结束")
}

// Close terminates the Python subprocess.
func (p *PyMatcher) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	p.mu.Unlock()
	close(p.done)
	p.wg.Wait()
}
