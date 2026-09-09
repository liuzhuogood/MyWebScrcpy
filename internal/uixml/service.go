// Package uixml provides a bounded, read-only Android UI XML snapshot service.
package uixml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxBytes = 2 << 20
	defaultTimeout  = 15 * time.Second
)

var (
	ErrMissingSerial = errors.New("missing serial")
	ErrInvalidXML    = errors.New("invalid UI XML")
	ErrTooLarge      = errors.New("UI XML exceeds size limit")
)

// Runner executes one adb invocation. It is injectable so command behavior can
// be tested without a connected device.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DisplaySize is the device coordinate space used by Android UI bounds.
// It is optional because an XML snapshot remains useful if wm size fails.
type DisplaySize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Snapshot struct {
	XML         string       `json:"xml"`
	CapturedAt  time.Time    `json:"captured_at"`
	DisplaySize *DisplaySize `json:"display_size,omitempty"`
}

// Service takes a point-in-time UI hierarchy snapshot. A device can only run
// one dump at a time because the dump path on the device is fixed.
type Service struct {
	run     Runner
	max     int64
	timeout time.Duration
	mu      sync.Mutex
	locks   map[string]*sync.Mutex
}

func New(adbPath string) *Service {
	return NewWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, adbPath, args...).Output()
	})
}

func NewWithRunner(run Runner) *Service {
	return &Service{run: run, max: DefaultMaxBytes, timeout: defaultTimeout, locks: make(map[string]*sync.Mutex)}
}

func (s *Service) lockFor(serial string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[serial] == nil {
		s.locks[serial] = &sync.Mutex{}
	}
	return s.locks[serial]
}

// Fetch returns the raw, validated XML and capture time.
func (s *Service) Fetch(ctx context.Context, serial string) (string, time.Time, error) {
	snapshot, err := s.FetchSnapshot(ctx, serial)
	if err != nil {
		return "", time.Time{}, err
	}
	return snapshot.XML, snapshot.CapturedAt, nil
}

// FetchSnapshot also reads the effective Android display size when available.
// UI Automator can omit system insets from maximum node bounds while the video
// stream is based on the full display coordinate space.
func (s *Service) FetchSnapshot(ctx context.Context, serial string) (Snapshot, error) {
	if strings.TrimSpace(serial) == "" {
		return Snapshot{}, ErrMissingSerial
	}
	lock := s.lockFor(serial)
	lock.Lock()
	defer lock.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if _, err := s.run(ctx, "-s", serial, "shell", "uiautomator", "dump", "/sdcard/window_dump.xml"); err != nil {
		return Snapshot{}, fmt.Errorf("uiautomator dump: %w", err)
	}
	out, err := s.run(ctx, "-s", serial, "exec-out", "cat", "/sdcard/window_dump.xml")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read UI XML: %w", err)
	}
	if int64(len(out)) > s.max {
		return Snapshot{}, ErrTooLarge
	}
	xmlText := string(out)
	if err := validateXML(strings.NewReader(xmlText)); err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{XML: xmlText, CapturedAt: time.Now().UTC()}
	if out, err := s.run(ctx, "-s", serial, "shell", "wm", "size"); err == nil {
		snapshot.DisplaySize = parseDisplaySize(string(out))
	}
	return snapshot, nil
}

var displaySizePattern = regexp.MustCompile(`(?m)^(Physical size|Override size):\s*(\d+)x(\d+)\s*$`)

func parseDisplaySize(output string) *DisplaySize {
	var physical, override *DisplaySize
	for _, match := range displaySizePattern.FindAllStringSubmatch(output, -1) {
		width, _ := strconv.Atoi(match[2])
		height, _ := strconv.Atoi(match[3])
		if width <= 0 || height <= 0 {
			continue
		}
		size := &DisplaySize{Width: width, Height: height}
		if match[1] == "Override size" {
			override = size
		} else {
			physical = size
		}
	}
	if override != nil {
		return override
	}
	return physical
}

func validateXML(r io.Reader) error {
	dec := xml.NewDecoder(r)
	depth, roots := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if roots != 1 || depth != 0 {
				return ErrInvalidXML
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidXML, err)
		}
		switch tok.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(tok.(xml.CharData))) != "" {
				return ErrInvalidXML
			}
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 {
					return ErrInvalidXML
				}
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return ErrInvalidXML
			}
		}
	}
}
