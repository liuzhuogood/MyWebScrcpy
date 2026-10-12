package dependencies

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitPathDoesNotFallback(t *testing.T) {
	t.Setenv("MYWEBSCRCPY_PYTHON", filepath.Join(t.TempDir(), "missing-python"))
	if _, err := PythonPath(); err == nil {
		t.Fatal("configured missing executable must not fall back")
	}
	t.Setenv("MYWEBSCRCPY_ADB", "relative/adb")
	if _, err := ADBPath(); err == nil {
		t.Fatal("relative executable accepted")
	}
}
func TestDependencyConfigRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	original := Current()
	t.Cleanup(func() { config.Lock(); config.paths = original; config.Unlock() })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	value := Paths{Python: executable}
	if err = Save(value); err != nil {
		t.Fatal(err)
	}
	if err = Load(); err != nil {
		t.Fatal(err)
	}
	if got := Current(); got != value {
		t.Fatalf("got %+v", got)
	}
	missing := Paths{ADB: filepath.Join(t.TempDir(), "missing-adb")}
	if err = Save(missing); err == nil {
		t.Fatal("missing executable saved")
	}
	if got := Current(); got != value {
		t.Fatal("failed save changed config")
	}
}
