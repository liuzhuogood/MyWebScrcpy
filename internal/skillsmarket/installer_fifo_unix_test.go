//go:build !windows && !plan9 && !js && !wasip1

package skillsmarket

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestFIFOQueryDoesNotBlockAndRejects(t *testing.T) {
	testHome := t.TempDir()
	mgr, err := NewManager(testHome)
	if err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(GetTargetSkillsPath(testHome, ToolCodex), string(SkillDeviceBase))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	fifoPath := filepath.Join(targetDir, ".mywebscrcpy-manifest.json")
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Skip("FIFO creation not supported on this environment")
	}

	done := make(chan TargetStatus, 1)
	go func() {
		done <- mgr.CheckSkillTargetStatus(ToolCodex, SkillDeviceBase)
	}()

	select {
	case st := <-done:
		if st.Installed || st.Status == "installed" {
			t.Fatalf("expected FIFO to be rejected as conflict, got %+v", st)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("query blocked indefinitely on FIFO!")
	}
}
