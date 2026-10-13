package pythoncheck

import (
	"context"
	"mywebscrcpy/internal/dependencies"
	"os"
	"strings"
	"testing"
)

func TestSDKContractValidator(t *testing.T) {
	source, err := os.ReadFile("../../.pi/skills/markdown-to-python/assets/example.py")
	if err != nil {
		t.Fatal(err)
	}
	python, err := dependencies.PythonPath()
	if err != nil {
		t.Fatal(err)
	}
	code := string(source)
	if err := Validate(context.Background(), python, code); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{"device.wait_for(['shell'], timeout=10)", "device.adb(['shell'], serial='other')", "__import__('os').system('echo bad')", "device.adb(['shell'], unexpected=1)"} {
		invalid := strings.Replace(code, `device.adb(["shell", "getprop", "ro.product.model"], timeout=10)`, replacement, 1)
		if Validate(context.Background(), python, invalid) == nil {
			t.Fatalf("accepted %s", replacement)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Validate(ctx, python, code) == nil {
		t.Fatal("cancelled validation accepted")
	}
}
