package ws

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

// TestRealDeviceSessionStartupPerf measures real-device session startup and
// first-keyframe latency when SCRCPY_BENCH_SERIAL is provided.
func TestRealDeviceSessionStartupPerf(t *testing.T) {
	serial := os.Getenv("SCRCPY_BENCH_SERIAL")
	if serial == "" {
		t.Skip("skipping real-device perf test: set SCRCPY_BENCH_SERIAL to run")
	}

	adbPath := os.Getenv("ADB")
	if adbPath == "" {
		adbPath = "/Users/liuzhuo/Library/Android/sdk/platform-tools/adb"
		if _, err := os.Stat(adbPath); err != nil {
			adbPath = "adb"
		}
	}

	jarPath := filepath.Clean("../../assets/scrcpy-server")
	if _, err := os.Stat(jarPath); err != nil {
		t.Fatalf("locate scrcpy-server jar: %v", err)
	}

	h := NewHub(adbPath, jarPath)
	h.nextPort = 38183

	measureColdStart := func(label string) (acquireDur, firstKeyDur, totalDur time.Duration) {
		t0 := time.Now()
		ms, meta, release, err := h.acquireSession(serial)
		if err != nil {
			t.Fatalf("%s acquireSession failed: %v", label, err)
		}
		acquireDur = time.Since(t0)

		ch, unsub := h.subscribeShared(ms)
		gotConfig := false
		gotKey := false
		timeout := time.After(8 * time.Second)
		for !gotConfig || !gotKey {
			select {
			case pkt, ok := <-ch:
				if !ok {
					t.Fatalf("%s shared video channel closed early", label)
				}
				if len(pkt.data) > 0 {
					kind := scrcpy.FrameKind(pkt.data[0])
					if kind == scrcpy.FrameConfig {
						gotConfig = true
					}
					if kind == scrcpy.FrameKey {
						gotKey = true
					}
				}
			case <-timeout:
				t.Fatalf("%s timed out waiting for config/key frame (config=%v key=%v)", label, gotConfig, gotKey)
			}
		}
		totalDur = time.Since(t0)
		firstKeyDur = totalDur - acquireDur
		unsub()
		release()

		t.Logf("[PERF] %s: acquire=%v, keyframeWait=%v, total=%v (size=%dx%d codec=%s)",
			label, acquireDur.Round(time.Millisecond), firstKeyDur.Round(time.Millisecond), totalDur.Round(time.Millisecond),
			meta.Width, meta.Height, meta.Codec)
		return acquireDur, firstKeyDur, totalDur
	}

	forceCloseActiveSession := func() {
		h.sessionMu.Lock()
		ms := h.sessions[serial]
		delete(h.sessions, serial)
		h.sessionMu.Unlock()
		if ms != nil {
			ms.mu.Lock()
			if ms.closeTimer != nil {
				ms.closeTimer.Stop()
				ms.closeTimer = nil
			}
			ms.mu.Unlock()
			ms.actions.Close()
			ms.sess.close()
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Cold Start #1 (first time Hub connects to this device)
	c1Acq, _, c1Tot := measureColdStart("ColdStart#1 (initial)")
	forceCloseActiveSession()

	// Cold Start #2 (second cold start in same Hub process)
	c2Acq, _, c2Tot := measureColdStart("ColdStart#2 (repeat cold)")
	forceCloseActiveSession()

	// Cold Start #3 (third cold start in same Hub process)
	c3Acq, _, c3Tot := measureColdStart("ColdStart#3 (repeat cold)")

	// Hot Reuse (within idle keepalive window, session still alive)
	hAcq, _, hTot := measureColdStart("HotReuse (within keepalive)")
	forceCloseActiveSession()

	avgRepeatCold := (c2Tot + c3Tot) / 2
	t.Logf("[PERF SUMMARY] ColdStart#1=%v (acquire=%v) | RepeatColdAvg=%v (acquire=%v) | HotReuse=%v (acquire=%v)",
		c1Tot.Round(time.Millisecond), c1Acq.Round(time.Millisecond),
		avgRepeatCold.Round(time.Millisecond), ((c2Acq + c3Acq) / 2).Round(time.Millisecond),
		hTot.Round(100*time.Microsecond), hAcq.Round(100*time.Microsecond))
}

func TestResolveDeviceEnvSkipsPushWhenRemoteJarMatchesAndCaches(t *testing.T) {
	tmpDir := t.TempDir()
	jarFile := filepath.Join(tmpDir, "scrcpy-server.jar")
	jarBytes := []byte("fake-scrcpy-server-jar-content-1234567890")
	if err := os.WriteFile(jarFile, jarBytes, 0o644); err != nil {
		t.Fatalf("write fake jar: %v", err)
	}

	countFile := filepath.Join(tmpDir, "adb_calls.txt")
	adbScript := filepath.Join(tmpDir, "fake-adb.sh")
	scriptContent := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + countFile + "\"\n" +
		"echo 33\n" +
		"echo 41\n"
	if err := os.WriteFile(adbScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("write fake adb: %v", err)
	}

	h := NewHub(adbScript, jarFile)
	sdk, needPush := h.resolveDeviceEnv("dev-1", false)
	if sdk != 33 || needPush {
		t.Fatalf("first resolveDeviceEnv = (sdk=%d, needPush=%v), want (33, false)", sdk, needPush)
	}
	h.markJarReady("dev-1", sdk)

	// Second call must hit in-memory cache without invoking adb at all.
	sdk2, needPush2 := h.resolveDeviceEnv("dev-1", false)
	if sdk2 != 33 || needPush2 {
		t.Fatalf("cached resolveDeviceEnv = (sdk=%d, needPush=%v), want (33, false)", sdk2, needPush2)
	}
	callsBytes, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatalf("read adb calls: %v", err)
	}
	lines := 0
	for _, b := range callsBytes {
		if b == '\n' {
			lines++
		}
	}
	if lines != 1 {
		t.Fatalf("expected exactly 1 adb invocation after cached call, got %d (%q)", lines, string(callsBytes))
	}

	// After invalidation or forcePush, needPush must be true when forced.
	_, needPushForced := h.resolveDeviceEnv("dev-1", true)
	if !needPushForced {
		t.Fatal("expected forcePush=true to require push")
	}
}

func TestAcquireSessionPerSerialConcurrencyDoesNotBlockOtherDevices(t *testing.T) {
	h := NewHub("adb", "")
	h.idleTTL = 20 * time.Millisecond

	slowStarted := make(chan struct{})
	slowRelease := make(chan struct{})
	var slowCalls int

	h.startSessionFn = func(serial string) (*session, *handshakeMeta, error) {
		if serial == "device-slow" {
			slowCalls++
			select {
			case <-slowStarted:
			default:
				close(slowStarted)
			}
			<-slowRelease
			return &session{}, &handshakeMeta{Type: "meta", Serial: serial, SessionID: "s-slow", Codec: "h264", Width: 448, Height: 1024}, nil
		}
		return &session{}, &handshakeMeta{Type: "meta", Serial: serial, SessionID: "s-fast", Codec: "h264", Width: 448, Height: 1024}, nil
	}

	// Launch two concurrent callers for device-slow.
	doneSlow1 := make(chan func(), 1)
	doneSlow2 := make(chan func(), 1)
	go func() {
		_, _, rel, err := h.acquireSession("device-slow")
		if err != nil {
			t.Errorf("slow1 err: %v", err)
		}
		doneSlow1 <- rel
	}()
	<-slowStarted
	go func() {
		_, _, rel, err := h.acquireSession("device-slow")
		if err != nil {
			t.Errorf("slow2 err: %v", err)
		}
		doneSlow2 <- rel
	}()

	// While device-slow is blocked inside startSession, device-fast must complete immediately.
	t0 := time.Now()
	_, fastMeta, fastRel, err := h.acquireSession("device-fast")
	fastDur := time.Since(t0)
	if err != nil {
		t.Fatalf("device-fast acquire failed: %v", err)
	}
	defer fastRel()
	if fastMeta.SessionID != "s-fast" {
		t.Fatalf("unexpected fast meta: %+v", fastMeta)
	}
	if fastDur > 100*time.Millisecond {
		t.Fatalf("device-fast was blocked by device-slow (%v)", fastDur)
	}

	// Unblock device-slow and verify singleflight shared the session across both waiters.
	close(slowRelease)
	rel1 := <-doneSlow1
	rel2 := <-doneSlow2
	if slowCalls != 1 {
		t.Fatalf("expected 1 startSession call for device-slow, got %d", slowCalls)
	}
	h.sessionMu.Lock()
	msSlow := h.sessions["device-slow"]
	h.sessionMu.Unlock()
	if msSlow == nil {
		t.Fatal("device-slow session missing")
	}
	msSlow.mu.Lock()
	refs := msSlow.refs
	msSlow.mu.Unlock()
	if refs != 2 {
		t.Fatalf("expected refs=2 for device-slow, got %d", refs)
	}
	rel1()
	rel2()
}
