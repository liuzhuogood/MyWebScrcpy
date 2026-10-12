package aiprovider

import (
	"errors"
	"testing"
	"testing/fstest"

	"mywebscrcpy/internal/pibundle"
)

func bundledPiForTest(t *testing.T) pibundle.Paths {
	t.Helper()
	paths, err := pibundle.Prepare(t.TempDir(), fstest.MapFS{})
	if errors.Is(err, pibundle.ErrUnprepared) {
		t.Skip("先运行 python3 scripts/prepare-pi.py 以验证内置 Pi")
	}
	if err != nil {
		t.Fatal(err)
	}
	// Neither Node/Pi nor system search tools are reachable by the child.
	t.Setenv("PATH", t.TempDir())
	return paths
}
