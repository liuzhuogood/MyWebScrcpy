package device

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/dependencies"
)

// Discovery owns at most one scan and one QR session. Polling leases prevent
// abandoned browser tabs from leaving discovery or pairing running indefinitely.
type Discovery struct {
	manager *Manager
	mu      sync.Mutex
	jobs    map[string]*discoveryJob
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
	dial    func(context.Context, string) error
	connect func(context.Context, string) error
}
type DiscoveryStatus struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"`
	State string   `json:"state"`
	Tried int      `json:"tried"`
	Total int      `json:"total"`
	Found []string `json:"found"`
	Error string   `json:"error,omitempty"`
	QR    string   `json:"qr,omitempty"`
}
type discoveryJob struct {
	DiscoveryStatus
	cancel  context.CancelFunc
	touched time.Time
	seen    map[string]bool
	done    chan struct{}
}
type ScanRequest struct {
	Range           string `json:"range"`
	Ports           []int  `json:"ports"`
	Continuous      bool   `json:"continuous"`
	IntervalSeconds int    `json:"interval_seconds"`
}

func NewDiscovery(m *Manager) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Discovery{manager: m, jobs: map[string]*discoveryJob{}, ctx: ctx, cancel: cancel}
	d.dial = func(ctx context.Context, address string) error {
		c, err := (&net.Dialer{Timeout: 350 * time.Millisecond}).DialContext(ctx, "tcp", address)
		if err == nil {
			c.Close()
		}
		return err
	}
	d.connect = d.connectDevice
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.mu.Lock()
				for _, j := range d.jobs {
					if activeDiscovery(j.State) && time.Since(j.touched) > 15*time.Second {
						j.State = "cancelled"
						j.cancel()
						j.QR = ""
					}
				}
				d.mu.Unlock()
			}
		}
	}()
	return d
}
func (d *Discovery) Close() {
	d.mu.Lock()
	d.closed = true
	d.cancel()
	for _, j := range d.jobs {
		j.cancel()
	}
	d.mu.Unlock()
	d.wg.Wait()
}
func activeDiscovery(state string) bool { return state == "running" || state == "waiting" }
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func (d *Discovery) start(kind string, duration time.Duration, work func(context.Context, *discoveryJob) error) (DiscoveryStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return DiscoveryStatus{}, fmt.Errorf("发现服务已停止")
	}
	for id, j := range d.jobs {
		if j.Kind == kind {
			finished := false
			select {
			case <-j.done:
				finished = true
			default:
			}
			if !finished {
				return DiscoveryStatus{}, fmt.Errorf("已有同类任务运行，请先停止")
			}
			delete(d.jobs, id)
		}
	}
	ctx, cancel := context.WithTimeout(d.ctx, duration)
	j := &discoveryJob{DiscoveryStatus: DiscoveryStatus{ID: randomID(), Kind: kind, State: "running", Found: []string{}}, cancel: cancel, touched: time.Now(), seen: map[string]bool{}, done: make(chan struct{})}
	d.jobs[j.ID] = j
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer cancel()
		defer close(j.done)
		err := work(ctx, j)
		d.mu.Lock()
		defer d.mu.Unlock()
		if !activeDiscovery(j.State) {
			return
		}
		j.QR = ""
		if err != nil {
			j.State = "failed"
			j.Error = err.Error()
			log.Printf("[Discovery] %s failed: %v", kind, err)
		} else {
			j.State = "completed"
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			j.State = "cancelled"
			j.Error = ""
		}
	}()
	return j.DiscoveryStatus, nil
}
func (d *Discovery) snapshot(id string, stop bool) (DiscoveryStatus, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	j, ok := d.jobs[id]
	if !ok {
		return DiscoveryStatus{}, false
	}
	j.touched = time.Now()
	if stop && activeDiscovery(j.State) {
		j.State = "cancelled"
		j.QR = ""
		j.cancel()
	}
	s := j.DiscoveryStatus
	s.Found = append([]string{}, j.Found...)
	return s, true
}
func (d *Discovery) found(j *discoveryJob, address string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if activeDiscovery(j.State) && !j.seen[address] {
		j.seen[address] = true
		j.Found = append(j.Found, address)
	}
}

func scanAddresses(req ScanRequest) ([]string, error) {
	input := strings.NewReplacer("到", "-", "至", "-", "~", "-").Replace(strings.TrimSpace(req.Range))
	ends := strings.Split(input, "-")
	if len(ends) > 2 {
		return nil, fmt.Errorf("IP 范围格式无效")
	}
	first, err := netip.ParseAddr(strings.TrimSpace(ends[0]))
	if err != nil || !first.Is4() {
		return nil, fmt.Errorf("请输入 IPv4 地址或起止范围")
	}
	last := first
	if len(ends) == 2 {
		tail := strings.TrimSpace(ends[1])
		if !strings.Contains(tail, ".") {
			prefix := strings.Split(first.String(), ".")
			tail = strings.Join(prefix[:3], ".") + "." + tail
		}
		last, err = netip.ParseAddr(tail)
		if err != nil || !last.Is4() {
			return nil, fmt.Errorf("结束 IP 无效")
		}
	}
	a := first.As4()
	b := last.As4()
	start, end := binary.BigEndian.Uint32(a[:]), binary.BigEndian.Uint32(b[:])
	if end < start || uint64(end)-uint64(start)+1 > 256 {
		return nil, fmt.Errorf("IP 范围须递增且最多 256 个地址")
	}
	if len(req.Ports) == 0 || len(req.Ports) > 8 {
		return nil, fmt.Errorf("请输入 1 至 8 个端口")
	}
	if req.Continuous && (req.IntervalSeconds < 10 || req.IntervalSeconds > 3600) {
		return nil, fmt.Errorf("扫描间隔须为 10 至 3600 秒")
	}
	var result []string
	seen := map[int]bool{}
	for _, port := range req.Ports {
		if port < 1 || port > 65535 || seen[port] {
			return nil, fmt.Errorf("端口须为不重复的 1 至 65535 整数")
		}
		seen[port] = true
	}
	for n := uint64(start); n <= uint64(end); n++ {
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], uint32(n))
		ip := netip.AddrFrom4(bytes)
		if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("仅支持局域网或回环地址扫描")
		}
		for _, port := range req.Ports {
			result = append(result, net.JoinHostPort(ip.String(), strconv.Itoa(port)))
		}
	}
	return result, nil
}
func (d *Discovery) scan(ctx context.Context, j *discoveryJob, req ScanRequest, addresses []string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		d.mu.Lock()
		if !activeDiscovery(j.State) {
			d.mu.Unlock()
			return context.Canceled
		}
		j.State = "running"
		j.Total = len(addresses)
		j.Tried = 0
		d.mu.Unlock()
		queue := make(chan string)
		var workers sync.WaitGroup
		for i := 0; i < 8; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for address := range queue {
					if ctx.Err() != nil {
						return
					}
					if err := d.dial(ctx, address); err == nil {
						if err = d.connect(ctx, address); err == nil {
							d.found(j, address)
						} else if ctx.Err() == nil {
							log.Printf("[Discovery] ADB connection not established at %s: %v", address, err)
						}
					}
					d.mu.Lock()
					j.Tried++
					d.mu.Unlock()
				}
			}()
		}
	send:
		for _, address := range addresses {
			select {
			case queue <- address:
			case <-ctx.Done():
				break send
			}
		}
		close(queue)
		workers.Wait()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !req.Continuous {
			return nil
		}
		d.mu.Lock()
		if activeDiscovery(j.State) {
			j.State = "waiting"
		}
		d.mu.Unlock()
		timer := time.NewTimer(time.Duration(req.IntervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (d *Discovery) command(ctx context.Context, stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := dependencies.CommandContext(ctx, d.manager.adbPath, args...)
	cmd.Dir = os.TempDir()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ADB 命令失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
func (d *Discovery) connectDevice(ctx context.Context, address string) error {
	if _, err := d.command(ctx, "", "connect", address); err != nil {
		return err
	}
	state, err := d.command(ctx, "", "-s", address, "get-state")
	if err != nil {
		return err
	}
	if state != "device" {
		return fmt.Errorf("设备尚未授权或连接")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if d.manager.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	_, err = d.manager.db.UpsertManualDevice(address, "", 1)
	return err
}

type mdnsService struct{ name, kind, address string }

func parseMDNSServices(output string) []mdnsService {
	var services []mdnsService
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		if fields[1] != "_adb-tls-pairing._tcp." && fields[1] != "_adb-tls-pairing._tcp" && fields[1] != "_adb-tls-connect._tcp." && fields[1] != "_adb-tls-connect._tcp" {
			continue
		}
		host, port, err := net.SplitHostPort(fields[2])
		if err != nil {
			continue
		}
		ip, err := netip.ParseAddr(host)
		p, e := strconv.Atoi(port)
		if err != nil || e != nil || p < 1 || p > 65535 || !ip.Is4() {
			continue
		}
		services = append(services, mdnsService{fields[0], strings.TrimSuffix(fields[1], "."), fields[2]})
	}
	return services
}
func (d *Discovery) pair(ctx context.Context, j *discoveryJob, name, password string) error {
	var host string
	paired := false
	for {
		out, err := d.command(ctx, "", "mdns", "services")
		if err != nil {
			return fmt.Errorf("无法发现无线调试服务，请检查 ADB 的 mDNS 支持: %w", err)
		}
		services := parseMDNSServices(out)
		for _, s := range services {
			if !paired && s.kind == "_adb-tls-pairing._tcp" && s.name == name {
				result, err := d.command(ctx, password+"\n", "pair", s.address)
				if err != nil {
					return fmt.Errorf("ADB 配对失败: %w", err)
				}
				if !strings.Contains(result, "Successfully paired to") {
					return fmt.Errorf("ADB 未确认配对成功，请重新扫码")
				}
				host, _, _ = net.SplitHostPort(s.address)
				paired = true
				log.Printf("[Discovery] QR pairing established; waiting for connection")
			}
		}
		if paired {
			for _, s := range services {
				h, _, _ := net.SplitHostPort(s.address)
				if s.kind == "_adb-tls-connect._tcp" && h == host {
					if err := d.connect(ctx, s.address); err == nil {
						d.found(j, s.address)
						return nil
					} else if ctx.Err() != nil {
						return ctx.Err()
					}
				}
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			if paired {
				return fmt.Errorf("已配对但连接未完成，请检查手机无线调试及网络后重试")
			}
			return fmt.Errorf("等待扫码超时或已取消")
		case <-timer.C:
		}
	}
}
func (d *Discovery) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/device-discovery/scan", func(w http.ResponseWriter, r *http.Request) {
		var req ScanRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			http.Error(w, "扫描参数无效", 400)
			return
		}
		addresses, err := scanAddresses(req)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		status, err := d.start("scan", 30*time.Minute, func(ctx context.Context, j *discoveryJob) error { return d.scan(ctx, j, req, addresses) })
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		discoveryJSON(w, status)
	})
	mux.HandleFunc("POST /api/device-discovery/pair", func(w http.ResponseWriter, r *http.Request) {
		name := "studio-" + randomID()[:10]
		password := randomID()[:16]
		status, err := d.start("pair", 2*time.Minute, func(ctx context.Context, j *discoveryJob) error { return d.pair(ctx, j, name, password) })
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		// The payload is returned only to the initiating UI, never included in logs.
		status.QR = "WIFI:T:ADB;S:" + name + ";P:" + password + ";;"
		w.Header().Set("Cache-Control", "no-store")
		discoveryJSON(w, status)
	})
	mux.HandleFunc("/api/device-discovery/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "DELETE" {
			w.Header().Set("Allow", "GET, DELETE")
			http.Error(w, "method not allowed", 405)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/device-discovery/jobs/")
		status, ok := d.snapshot(id, r.Method == "DELETE")
		if !ok {
			http.Error(w, "发现任务不存在", 404)
			return
		}
		if r.Method == "DELETE" {
			d.mu.Lock()
			var done <-chan struct{}
			if job, exists := d.jobs[id]; exists {
				done = job.done
			}
			d.mu.Unlock()
			if done != nil {
				select {
				case <-done:
				case <-r.Context().Done():
					return
				case <-time.After(5 * time.Second):
					http.Error(w, "任务正在停止，请稍后重试", http.StatusConflict)
					return
				}
			}
		}
		discoveryJSON(w, status)
	})
}
func discoveryJSON(w http.ResponseWriter, status DiscoveryStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(status)
}
