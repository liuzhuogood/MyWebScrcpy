// Package uipage provides a bounded, read-only Android UI Page information service.
package uipage

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
)

var (
	ErrMissingSerial    = errors.New("missing serial")
	ErrPageInfoNotFound = errors.New("unable to resolve current focused page")
)

// Runner executes one adb invocation. It is injectable so command behavior can
// be tested without a connected device.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DisplayInfo holds display dimensions, density and rotation angle.
type DisplayInfo struct {
	Width      int `json:"width"`
	Height     int `json:"height"`
	DensityDpi int `json:"density_dpi,omitempty"`
	Rotation   int `json:"rotation"` // 0, 90, 180, 270
}

// PageInfo represents the foreground application and window metadata.
type PageInfo struct {
	PackageName string       `json:"package_name"`
	Activity    string       `json:"activity"`
	Component   string       `json:"component"`
	WindowName  string       `json:"window_name"`
	Display     *DisplayInfo `json:"display,omitempty"`
	FetchedAt   time.Time    `json:"fetched_at"`
}

// Service queries Android system services via adb to resolve the current foreground page and display metrics.
type Service struct {
	run     Runner
	timeout time.Duration
	mu      sync.Mutex
	locks   map[string]*sync.Mutex
}

// New creates a Service using real adb binary.
func New(adbPath string) *Service {
	return NewWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, adbPath, args...).Output()
	})
}

// NewWithRunner creates a Service with a custom adb runner (ideal for unit testing).
func NewWithRunner(run Runner) *Service {
	return &Service{
		run:     run,
		timeout: defaultTimeout,
		locks:   make(map[string]*sync.Mutex),
	}
}

func (s *Service) lockFor(serial string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[serial] == nil {
		s.locks[serial] = &sync.Mutex{}
	}
	return s.locks[serial]
}

// FetchPageInfo queries adb commands to extract foreground page info and display metrics.
func (s *Service) FetchPageInfo(ctx context.Context, serial string) (*PageInfo, error) {
	if strings.TrimSpace(serial) == "" {
		return nil, ErrMissingSerial
	}
	lock := s.lockFor(serial)
	lock.Lock()
	defer lock.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	page := &PageInfo{
		FetchedAt: time.Now().UTC(),
	}

	// 1. Resolve foreground window & activity
	// Priority 1: dumpsys window (mCurrentFocus / mFocusedApp)
	resolved := false
	if winOut, err := s.run(ctx, "-s", serial, "shell", "dumpsys", "window"); err == nil {
		if p, ok := parseWindowDump(string(winOut)); ok {
			page.PackageName = p.PackageName
			page.Activity = p.Activity
			page.Component = p.Component
			page.WindowName = p.WindowName
			resolved = true
		}
	}

	// Priority 2: Fallback to dumpsys activity activities (mResumedActivity)
	if !resolved {
		if actOut, err := s.run(ctx, "-s", serial, "shell", "dumpsys", "activity", "activities"); err == nil {
			if p, ok := parseActivityDump(string(actOut)); ok {
				page.PackageName = p.PackageName
				page.Activity = p.Activity
				page.Component = p.Component
				page.WindowName = p.WindowName
				resolved = true
			}
		}
	}

	if !resolved {
		return nil, ErrPageInfoNotFound
	}

	// 2. Resolve Display Info (size, density, rotation)
	display := &DisplayInfo{}
	hasDisplay := false

	// 2.1 wm size
	if sizeOut, err := s.run(ctx, "-s", serial, "shell", "wm", "size"); err == nil {
		if w, h, ok := parseWmSize(string(sizeOut)); ok {
			display.Width = w
			display.Height = h
			hasDisplay = true
		}
	}

	// 2.2 wm density
	if densityOut, err := s.run(ctx, "-s", serial, "shell", "wm", "density"); err == nil {
		if dpi, ok := parseWmDensity(string(densityOut)); ok {
			display.DensityDpi = dpi
			hasDisplay = true
		}
	}

	// 2.3 Rotation: check dumpsys input -> dumpsys window displays
	rotResolved := false
	if inputOut, err := s.run(ctx, "-s", serial, "shell", "dumpsys", "input"); err == nil {
		if rot, ok := parseSurfaceOrientation(string(inputOut)); ok {
			display.Rotation = rot
			rotResolved = true
			hasDisplay = true
		}
	}
	if !rotResolved {
		if winDispOut, err := s.run(ctx, "-s", serial, "shell", "dumpsys", "window", "displays"); err == nil {
			if rot, ok := parseWindowDisplaysRotation(string(winDispOut)); ok {
				display.Rotation = rot
				hasDisplay = true
			}
		}
	}

	if hasDisplay {
		page.Display = display
	}

	return page, nil
}

// Regex patterns for window and activity parsing
var (
	// mCurrentFocus=Window{a833e82 u0 com.uutalk.im/com.jiangxia.im.MainActivity}
	// or Window{... u0 com.example/.MainActivity}
	reCurrentFocus = regexp.MustCompile(`(?m)mCurrentFocus=(?:null|Window\{[0-9a-fA-F]+\s+[^\s}]+\s+([^\s}]+)\})`)
	// mFocusedApp=AppWindowToken{... token=Token{... ActivityRecord{... u0 com.uutalk.im/com.jiangxia.im.MainActivity ...}}}
	// or ActivityRecord{... u0 com.uutalk.im/.MainActivity ...}
	reFocusedApp = regexp.MustCompile(`(?m)mFocusedApp=.*?(?:ActivityRecord\{[0-9a-fA-F]+\s+[^\s}]+\s+([^\s}]+)|\s([a-zA-Z0-9._]+/[a-zA-Z0-9._$]+))`)

	// dumpsys activity activities:
	// mResumedActivity: ActivityRecord{45a165b u0 com.uutalk.im/com.jiangxia.im.MainActivity t123}
	// topResumedActivity=ActivityRecord{...}
	reResumedActivity = regexp.MustCompile(`(?m)(?:mResumedActivity|topResumedActivity|mFocusedActivity|mLastResumedActivity):\s*ActivityRecord\{[0-9a-fA-F]+\s+[^\s}]+\s+([^\s}]+)`)

	// wm size
	reWmSize = regexp.MustCompile(`(?m)^(Physical size|Override size):\s*(\d+)x(\d+)\s*$`)

	// wm density
	// Physical density: 420
	// Override density: 480
	reWmDensity = regexp.MustCompile(`(?m)^(Physical density|Override density):\s*(\d+)\s*$`)

	// dumpsys input
	// SurfaceOrientation: 0
	reSurfaceOrientation = regexp.MustCompile(`(?m)SurfaceOrientation:\s*([0-3])`)

	// dumpsys window displays
	reDisplayRotation = regexp.MustCompile(`(?m)(?:mCurrentRotation|rotation|mRotation|orientation)=(\d)`)
)

// normalizeComponent takes "pkg/act" or "pkg/.Sub" and normalizes it to full component, package, and activity.
func normalizeComponent(rawComp string) (pkg string, act string, fullComp string, ok bool) {
	rawComp = strings.TrimSpace(rawComp)
	if rawComp == "" || rawComp == "null" {
		return "", "", "", false
	}
	parts := strings.SplitN(rawComp, "/", 2)
	if len(parts) != 2 {
		return "", "", "", false
	}
	pkg = strings.TrimSpace(parts[0])
	act = strings.TrimSpace(parts[1])
	if pkg == "" || act == "" {
		return "", "", "", false
	}
	if strings.HasPrefix(act, ".") {
		act = pkg + act
	}
	fullComp = pkg + "/" + act
	return pkg, act, fullComp, true
}

func parseWindowDump(output string) (*PageInfo, bool) {
	// Try mCurrentFocus first
	for _, match := range reCurrentFocus.FindAllStringSubmatch(output, -1) {
		if len(match) > 1 && match[1] != "" {
			if pkg, act, comp, ok := normalizeComponent(match[1]); ok {
				winName := match[0]
				if idx := strings.Index(match[0], "Window{"); idx != -1 {
					winName = match[0][idx:]
				}
				return &PageInfo{
					PackageName: pkg,
					Activity:    act,
					Component:   comp,
					WindowName:  winName,
				}, true
			}
		}
	}

	// Try mFocusedApp second
	for _, match := range reFocusedApp.FindAllStringSubmatch(output, -1) {
		raw := ""
		if len(match) > 1 && match[1] != "" {
			raw = match[1]
		} else if len(match) > 2 && match[2] != "" {
			raw = match[2]
		}
		if raw != "" {
			if pkg, act, comp, ok := normalizeComponent(raw); ok {
				return &PageInfo{
					PackageName: pkg,
					Activity:    act,
					Component:   comp,
					WindowName:  match[0],
				}, true
			}
		}
	}

	return nil, false
}

func parseActivityDump(output string) (*PageInfo, bool) {
	for _, match := range reResumedActivity.FindAllStringSubmatch(output, -1) {
		if len(match) > 1 && match[1] != "" {
			if pkg, act, comp, ok := normalizeComponent(match[1]); ok {
				return &PageInfo{
					PackageName: pkg,
					Activity:    act,
					Component:   comp,
					WindowName:  match[0],
				}, true
			}
		}
	}
	return nil, false
}

func parseWmSize(output string) (width int, height int, ok bool) {
	var physicalW, physicalH int
	var overrideW, overrideH int
	hasPhysical, hasOverride := false, false

	for _, match := range reWmSize.FindAllStringSubmatch(output, -1) {
		w, _ := strconv.Atoi(match[2])
		h, _ := strconv.Atoi(match[3])
		if w <= 0 || h <= 0 {
			continue
		}
		if match[1] == "Override size" {
			overrideW, overrideH = w, h
			hasOverride = true
		} else {
			physicalW, physicalH = w, h
			hasPhysical = true
		}
	}

	if hasOverride {
		return overrideW, overrideH, true
	}
	if hasPhysical {
		return physicalW, physicalH, true
	}
	return 0, 0, false
}

func parseWmDensity(output string) (dpi int, ok bool) {
	var physicalDpi, overrideDpi int
	hasPhysical, hasOverride := false, false

	for _, match := range reWmDensity.FindAllStringSubmatch(output, -1) {
		val, _ := strconv.Atoi(match[2])
		if val <= 0 {
			continue
		}
		if match[1] == "Override density" {
			overrideDpi = val
			hasOverride = true
		} else {
			physicalDpi = val
			hasPhysical = true
		}
	}

	if hasOverride {
		return overrideDpi, true
	}
	if hasPhysical {
		return physicalDpi, true
	}
	return 0, false
}

func parseSurfaceOrientation(output string) (rotation int, ok bool) {
	match := reSurfaceOrientation.FindStringSubmatch(output)
	if len(match) > 1 {
		val, err := strconv.Atoi(match[1])
		if err == nil {
			switch val {
			case 0:
				return 0, true
			case 1:
				return 90, true
			case 2:
				return 180, true
			case 3:
				return 270, true
			}
		}
	}
	return 0, false
}

func parseWindowDisplaysRotation(output string) (rotation int, ok bool) {
	matches := reDisplayRotation.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) > 1 {
			val, err := strconv.Atoi(match[1])
			if err == nil {
				switch val {
				case 0:
					return 0, true
				case 1:
					return 90, true
				case 2:
					return 180, true
				case 3:
					return 270, true
				}
			}
		}
	}
	return 0, false
}
