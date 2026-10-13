package aiprovider

import (
	"os"
	"testing"
	"testing/fstest"

	"mywebscrcpy/internal/pibundle"
)

func bundledPiForTest(t *testing.T) pibundle.Paths {
	t.Helper()
	source := os.Getenv("MYWEBSCRCPY_PI_TEST_PAYLOAD")
	if source == "" {
		t.Skip("设置 MYWEBSCRCPY_PI_TEST_PAYLOAD 验证独立 Pi 包")
	}
	paths, err := pibundle.PrepareFrom(t.TempDir(), fstest.MapFS{}, os.DirFS(source))
	if err != nil {
		t.Fatal(err)
	}
	// Neither Node/Pi nor system search tools are reachable by the child.
	t.Setenv("PATH", t.TempDir())
	return paths
}
