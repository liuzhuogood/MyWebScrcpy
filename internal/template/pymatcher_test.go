package template

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestPyMatcherHandshake verifies the Go<->Python protocol: SubmitFrame writes
// a control frame, the Python subprocess echoes a JSON result, and the result
// callback fires. Requires a python3 with av + cv2 (set MYWEBSCRCPY_PYTHON or
// put it on PATH); otherwise the test is skipped.
func TestPyMatcherHandshake(t *testing.T) {
	py := os.Getenv("MYWEBSCRCPY_PYTHON")
	if py == "" {
		var err error
		py, err = exec.LookPath("python3")
		if err != nil {
			t.Skip("python3 not found")
		}
	}
	if out, err := exec.Command(py, "-c", "import av, cv2").CombinedOutput(); err != nil {
		t.Skipf("python lacks av/cv2, skipping: %v", string(out))
	}
	resultCh := make(chan struct{})
	p := NewPyMatcher(t.TempDir(), func(serial string, frameID uint64, matches []MatchResult) {
		if serial == "test-dev" {
			select {
			case <-resultCh:
			default:
				close(resultCh)
			}
		}
	})
	if err := p.Start(); err != nil {
		t.Fatalf("start python matcher: %v", err)
	}
	defer p.Close()

	p.SubmitFrame("test-dev", 254, 0, nil) // control: ready -> emits JSON

	select {
	case <-resultCh:
		// success
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for python result callback")
	}
}
