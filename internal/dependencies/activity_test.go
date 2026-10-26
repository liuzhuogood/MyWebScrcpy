package dependencies

import (
	"context"
	"os"
	"testing"
)

func TestManagedADBActivityAndPassiveMonitoring(t *testing.T) {
	manager := newTestManager(t)
	manager.settings.ADB = Selection{Source: "system", Path: os.Args[0]}
	previous := active
	active = manager
	t.Cleanup(func() { active = previous; SetDeviceActivityObserver(nil) })
	refs := 0
	SetDeviceActivityObserver(func(serial string) func() {
		if serial != "phone" {
			t.Errorf("wrong target %s", serial)
		}
		refs++
		return func() { refs-- }
	})
	for _, passive := range []bool{false, true} {
		ctx := context.Background()
		if passive {
			ctx = BackgroundADB(ctx)
		}
		command := CommandContext(ctx, ManagedADB, "-s", "phone", "shell")
		if err := command.Acquire(); err != nil {
			t.Fatal(err)
		}
		expected := 1
		if passive {
			expected = 0
		}
		if refs != expected {
			t.Fatalf("passive=%t active=%d", passive, refs)
		}
		command.Release()
		command.Release()
		if refs != 0 {
			t.Fatal("activity lease leaked")
		}
	}
}
