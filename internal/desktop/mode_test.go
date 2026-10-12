package desktop

import "testing"

func TestSelect(t *testing.T) {
	for _, c := range []struct {
		name string
		o    Options
		want Mode
		fail bool
	}{
		{"headless", Options{}, Service, false},
		{"desktop", Options{Shell: "host", DesktopAvailable: true}, GUI, false},
		{"force service", Options{NoGUI: true, Shell: "host", DesktopAvailable: true}, Service, false},
		{"missing shell", Options{GUI: true, DesktopAvailable: true}, "", true},
		{"unreachable display", Options{GUI: true, Shell: "host"}, "", true},
		{"conflict", Options{GUI: true, NoGUI: true}, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := Select(c.o)
			if got != c.want || (err != nil) != c.fail {
				t.Fatalf("mode=%q err=%v", got, err)
			}
		})
	}
}

func TestLinuxDisplayAvailable(t *testing.T) {
	cases := []struct {
		name             string
		env              map[string]string
		network, address string
		reachable, want  bool
	}{
		{"no display", map[string]string{}, "", "", false, false},
		{"wayland", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "XDG_RUNTIME_DIR": "/run/user/1000"}, "unix", "/run/user/1000/wayland-0", true, true},
		{"stale wayland", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "XDG_RUNTIME_DIR": "/run/user/1000"}, "unix", "/run/user/1000/wayland-0", false, false},
		{"missing runtime", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, "", "", false, false},
		{"x11", map[string]string{"DISPLAY": ":0.0"}, "unix", "/tmp/.X11-unix/X0", true, true},
		{"ssh", map[string]string{"DISPLAY": "localhost:10.0"}, "tcp", "localhost:6010", true, true},
		{"malformed", map[string]string{"DISPLAY": ":invalid"}, "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LinuxDisplayAvailable(func(k string) string { return c.env[k] }, func(n, a string) bool {
				if n != c.network || a != c.address {
					t.Errorf("probe %s %s", n, a)
				}
				return c.reachable
			})
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
