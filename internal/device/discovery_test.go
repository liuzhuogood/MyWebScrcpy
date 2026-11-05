package device

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestScanAddressesBoundsAndValidation(t *testing.T) {
	tests := []struct {
		name string
		req  ScanRequest
		want []string
		bad  bool
	}{
		{"single", ScanRequest{Range: "192.168.1.8", Ports: []int{5555}}, []string{"192.168.1.8:5555"}, false},
		{"full range inclusive", ScanRequest{Range: "192.168.1.1-192.168.1.3", Ports: []int{1, 2}}, []string{"192.168.1.1:1", "192.168.1.1:2", "192.168.1.2:1", "192.168.1.2:2", "192.168.1.3:1", "192.168.1.3:2"}, false},
		{"short tail inclusive", ScanRequest{Range: "192.168.1.254-255", Ports: []int{5555}}, []string{"192.168.1.254:5555", "192.168.1.255:5555"}, false},
		{"chinese 到 range", ScanRequest{Range: "192.168.1.1 到 2", Ports: []int{5555}}, []string{"192.168.1.1:5555", "192.168.1.2:5555"}, false},
		{"256 allowed", ScanRequest{Range: "10.0.0.0-255", Ports: []int{1}}, nil, false},
		{"257 rejected", ScanRequest{Range: "10.0.0.0-10.0.1.0", Ports: []int{1}}, nil, true},
		{"descending rejected", ScanRequest{Range: "10.0.0.2-1", Ports: []int{1}}, nil, true},
		{"public rejected", ScanRequest{Range: "8.8.8.8", Ports: []int{1}}, nil, true},
		{"bad port rejected", ScanRequest{Range: "10.0.0.1", Ports: []int{0}}, nil, true},
		{"duplicate port rejected", ScanRequest{Range: "10.0.0.1", Ports: []int{5555, 5555}}, nil, true},
		{"too many ports rejected", ScanRequest{Range: "10.0.0.1", Ports: []int{1, 2, 3, 4, 5, 6, 7, 8, 9}}, nil, true},
		{"continuous interval too short", ScanRequest{Range: "10.0.0.1", Ports: []int{5555}, Continuous: true, IntervalSeconds: 9}, nil, true},
		{"continuous interval too long", ScanRequest{Range: "10.0.0.1", Ports: []int{5555}, Continuous: true, IntervalSeconds: 3601}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanAddresses(tt.req)
			if (err != nil) != tt.bad {
				t.Fatalf("err=%v, bad=%v", err, tt.bad)
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if tt.name == "256 allowed" && (err != nil || len(got) != 256 || got[255] != "10.0.0.255:1") {
				t.Fatalf("256 boundary: len=%d last=%v err=%v", len(got), got[len(got)-1], err)
			}
		})
	}
}

// buildFakeADB creates a small native executable, avoiding platform-specific shell scripts.
func buildFakeADB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := `package main
import ("fmt"; "io"; "os"; "strings")
func main() {
  args := strings.Join(os.Args[1:], " ")
  switch os.Getenv("FAKE_ADB_MODE") {
  case "screen":
    if strings.Contains(args, "dumpsys power") { fmt.Print(os.Getenv("FAKE_ADB_POWER")); return }
    if strings.Contains(args, "dumpsys window policy") { fmt.Print(os.Getenv("FAKE_ADB_POLICY")); return }
    os.Exit(9)
  case "fail": fmt.Fprintln(os.Stderr, "fake adb failure"); os.Exit(7)
  case "pair":
    if strings.Contains(args, "mdns services") {
      fmt.Printf("wrong _adb-tls-pairing._tcp. 192.168.1.4:37123\n%s _adb-tls-pairing._tcp. 192.168.1.4:37124\nphone _adb-tls-connect._tcp. 192.168.1.4:41231\n", os.Getenv("FAKE_ADB_NAME")); return
    }
    if strings.HasPrefix(args, "pair ") {
      b, _ := io.ReadAll(os.Stdin)
      if string(b) != os.Getenv("FAKE_ADB_PASSWORD")+"\n" { fmt.Fprintln(os.Stderr, "wrong pairing stdin"); os.Exit(8) }
      if os.Getenv("FAKE_ADB_PAIR_FAIL") == "1" { fmt.Print("Pairing rejected"); return }
      fmt.Print("Successfully paired to 192.168.1.4")
      return
    }
  }
  fmt.Fprintf(os.Stderr, "unexpected fake adb args: %s", args); os.Exit(10)
}`
	src := filepath.Join(dir, "fake_adb.go")
	if err := os.WriteFile(src, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "fake-adb")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake adb: %v: %s", err, out)
	}
	return bin
}

func TestParseMDNSServicesRequiresExactThreeFieldsAndValidIPv4(t *testing.T) {
	out := "pair _adb-tls-pairing._tcp. 192.168.1.4:37123\n" +
		"connect _adb-tls-connect._tcp 192.168.1.4:41231\n" +
		"extra ignored _adb-tls-connect._tcp 192.168.1.4:1\n" +
		"bad _adb-tls-pairing._tcp host:3\n" +
		"v6 _adb-tls-connect._tcp [::1]:3\n" +
		"port _adb-tls-connect._tcp 192.168.1.4:70000\n" +
		"other _adb-tcp._tcp 192.168.1.4:3"
	got := parseMDNSServices(out)
	if len(got) != 2 || got[0] != (mdnsService{"pair", "_adb-tls-pairing._tcp", "192.168.1.4:37123"}) || got[1] != (mdnsService{"connect", "_adb-tls-connect._tcp", "192.168.1.4:41231"}) {
		t.Fatalf("unexpected parsed services: %#v", got)
	}
}

func TestDiscoveryCapacityCancelAndClose(t *testing.T) {
	d := NewDiscovery(nil)
	defer d.Close()
	started, release := make(chan struct{}), make(chan struct{})
	first, err := d.start("scan", time.Minute, func(ctx context.Context, _ *discoveryJob) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := d.start("scan", time.Minute, func(context.Context, *discoveryJob) error { return nil }); err == nil {
		t.Fatal("expected same-kind capacity rejection")
	}
	if _, err := d.start("pair", time.Minute, func(context.Context, *discoveryJob) error { return nil }); err != nil {
		t.Fatalf("scan and pair should coexist: %v", err)
	}
	if s, ok := d.snapshot(first.ID, true); !ok || s.State != "cancelled" {
		t.Fatalf("stop snapshot = %#v, %v", s, ok)
	}
	close(release)
	d.Close()
	if _, err := d.start("scan", time.Minute, func(context.Context, *discoveryJob) error { return nil }); err == nil {
		t.Fatal("start after close should fail")
	}
}

func TestScanDoesNotRecordTCPOnlyADBFailure(t *testing.T) {
	d := NewDiscovery(nil)
	defer d.Close()
	d.dial = func(context.Context, string) error { return nil }
	d.connect = func(context.Context, string) error { return errors.New("adb unauthorized") }
	j := &discoveryJob{DiscoveryStatus: DiscoveryStatus{State: "running", Found: []string{}}, seen: map[string]bool{}}
	addresses, err := scanAddresses(ScanRequest{Range: "127.0.0.1", Ports: []int{5555}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.scan(context.Background(), j, ScanRequest{}, addresses); err != nil {
		t.Fatal(err)
	}
	if len(j.Found) != 0 || j.Tried != 1 {
		t.Fatalf("TCP-only result recorded: found=%v tried=%d", j.Found, j.Tried)
	}
}

func TestPairMatchesServiceNameReadsPasswordAndConnectsBeforeFound(t *testing.T) {
	bin := buildFakeADB(t)
	t.Setenv("FAKE_ADB_MODE", "pair")
	t.Setenv("FAKE_ADB_NAME", "expected-name")
	t.Setenv("FAKE_ADB_PASSWORD", "123456")
	d := NewDiscovery(&Manager{adbPath: bin})
	defer d.Close()
	connected := ""
	d.connect = func(_ context.Context, address string) error { connected = address; return nil }
	j := &discoveryJob{DiscoveryStatus: DiscoveryStatus{State: "running", Found: []string{}}, seen: map[string]bool{}}
	if err := d.pair(context.Background(), j, "expected-name", "123456"); err != nil {
		t.Fatal(err)
	}
	if connected != "192.168.1.4:41231" || !reflect.DeepEqual(j.Found, []string{connected}) {
		t.Fatalf("connect=%q found=%v", connected, j.Found)
	}
}

func TestPairDoesNotClaimSuccessWhenADBRejectsPairing(t *testing.T) {
	bin := buildFakeADB(t)
	t.Setenv("FAKE_ADB_MODE", "pair")
	t.Setenv("FAKE_ADB_NAME", "expected-name")
	t.Setenv("FAKE_ADB_PASSWORD", "123456")
	t.Setenv("FAKE_ADB_PAIR_FAIL", "1")
	d := NewDiscovery(&Manager{adbPath: bin})
	defer d.Close()
	connected := false
	d.connect = func(context.Context, string) error { connected = true; return nil }
	j := &discoveryJob{DiscoveryStatus: DiscoveryStatus{State: "running", Found: []string{}}, seen: map[string]bool{}}
	err := d.pair(context.Background(), j, "expected-name", "123456")
	if err == nil || connected || len(j.Found) != 0 {
		t.Fatalf("error=%v connected=%v found=%v", err, connected, j.Found)
	}
}
