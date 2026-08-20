package files

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
)

func TestParseStatLine(t *testing.T) {
	item, ok := parseStatLine("regular file\t42\t1710000000\t/sdcard/照片/a b.txt")
	if !ok {
		t.Fatal("expected stat line to parse")
	}
	if item.Path != "照片/a b.txt" || item.Kind != "document" || item.Size != 42 {
		t.Fatalf("unexpected item: %#v", item)
	}
	item, ok = parseStatLine("directory\t4096\t1710000000\t/sdcard/照片")
	if !ok || item.Kind != "folder" || item.Size != 0 {
		t.Fatalf("unexpected directory: %#v", item)
	}
	if _, ok := parseStatLine("symbolic link\t10\t1710000000\t/sdcard/link"); ok {
		t.Fatal("symbolic links must not be exposed")
	}
}

func TestCleanRelRejectsServerAndTrashPaths(t *testing.T) {
	for _, value := range []string{"../outside", "/etc/passwd", "\\windows", ".mywebscrcpy-trash/x", "a\x00b"} {
		if _, err := cleanRel(value); !errors.Is(err, ErrForbidden) {
			t.Fatalf("cleanRel(%q) = %v, want ErrForbidden", value, err)
		}
	}
	if got, err := cleanRel("照片/./a.txt"); err != nil || got != "照片/a.txt" {
		t.Fatalf("cleanRel normalized path = %q, %v", got, err)
	}
}

func TestFileRoutesRequireCurrentPhone(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewManager("adb"))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "missing_serial") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestFileRoutesReportDisconnectedPhone(t *testing.T) {
	m := NewManager("adb")
	m.run = func(string, ...string) ([]byte, error) {
		return []byte("error: device 'phone-a' not found\n"), errors.New("exit status 1: error: device 'phone-a' not found")
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, m)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/files?serial=phone-a", nil))
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "device_unavailable") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestListUsesSelectedPhoneStorage(t *testing.T) {
	m := NewManager("adb")
	seenSerial := ""
	m.run = func(serial string, args ...string) ([]byte, error) {
		seenSerial = serial
		if len(args) < 2 || args[0] != "shell" {
			t.Fatalf("unexpected adb command: %v", args)
		}
		script := args[1]
		switch {
		case strings.HasPrefix(script, "stat -c"):
			return []byte("directory\t0\t1710000000\t/sdcard\n"), nil
		case strings.HasPrefix(script, "find"):
			return []byte("directory\t0\t1710000000\t/sdcard/照片\nregular file\t4\t1710000001\t/sdcard/readme.txt\n"), nil
		case strings.HasPrefix(script, "df"):
			return []byte("Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/block 100 40 60 40% /sdcard\n"), nil
		default:
			t.Fatalf("unexpected shell script: %s", script)
			return nil, nil
		}
	}
	result, err := m.List("phone-1", "", "", "", "name", "asc", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if seenSerial != "phone-1" || len(result.Items) != 2 || result.Items[0].Name != "照片" || result.Storage.Total != 100*1024 {
		t.Fatalf("unexpected phone list: serial=%q result=%#v", seenSerial, result)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a'b"); got != "'a'\\''b'" {
		t.Fatalf("unexpected shell quote: %q", got)
	}
}

type fakePhone struct {
	dirs  map[string]bool
	files map[string][]byte
}

type fakeADB struct{ phones map[string]*fakePhone }

func newFakeADB(serials ...string) *fakeADB {
	adb := &fakeADB{phones: make(map[string]*fakePhone)}
	for _, serial := range serials {
		adb.phones[serial] = &fakePhone{dirs: map[string]bool{"/sdcard": true}, files: make(map[string][]byte)}
	}
	return adb
}

func (a *fakeADB) phone(serial string) (*fakePhone, error) {
	phone, ok := a.phones[serial]
	if !ok {
		return nil, fmt.Errorf("unknown serial %s", serial)
	}
	return phone, nil
}

func (a *fakeADB) run(serial string, args ...string) ([]byte, error) {
	phone, err := a.phone(serial)
	if err != nil {
		return nil, err
	}
	if len(args) > 0 && args[0] == "push" {
		data, err := os.ReadFile(args[1])
		if err != nil {
			return nil, err
		}
		phone.files[args[2]] = data
		return nil, nil
	}
	if len(args) < 2 || args[0] != "shell" {
		return nil, fmt.Errorf("unsupported adb command: %v", args)
	}
	return phone.shell(args[1])
}

func (a *fakeADB) stream(serial string, dst io.Writer, args ...string) error {
	phone, err := a.phone(serial)
	if err != nil {
		return err
	}
	if len(args) != 3 || args[0] != "exec-out" || args[1] != "cat" {
		return fmt.Errorf("unsupported stream command: %v", args)
	}
	data, ok := phone.files[args[2]]
	if !ok {
		return errors.New("No such file or directory")
	}
	_, err = dst.Write(data)
	return err
}

func quotedPaths(script string) []string {
	var result []string
	for len(script) > 0 {
		start := strings.IndexByte(script, '\'')
		if start < 0 {
			break
		}
		script = script[start+1:]
		end := strings.IndexByte(script, '\'')
		if end < 0 {
			break
		}
		result = append(result, script[:end])
		script = script[end+1:]
	}
	return result
}

func (p *fakePhone) exists(remote string) bool { return p.dirs[remote] || p.files[remote] != nil }

func (p *fakePhone) statLine(remote string) ([]byte, error) {
	if p.dirs[remote] {
		return []byte(fmt.Sprintf("directory\t0\t1710000000\t%s\n", remote)), nil
	}
	data, ok := p.files[remote]
	if !ok {
		return nil, errors.New("No such file or directory")
	}
	return []byte(fmt.Sprintf("regular file\t%d\t1710000000\t%s\n", len(data), remote)), nil
}

func (p *fakePhone) shell(script string) ([]byte, error) {
	paths := quotedPaths(script)
	var remotePaths []string
	for _, value := range paths {
		if strings.HasPrefix(value, androidRoot) {
			remotePaths = append(remotePaths, value)
		}
	}
	switch {
	case strings.HasPrefix(script, "stat -c"):
		return p.statLine(remotePaths[len(remotePaths)-1])
	case strings.HasPrefix(script, "find"):
		return p.find(remotePaths[0], strings.Contains(script, "-maxdepth"))
	case strings.HasPrefix(script, "df"):
		return []byte("Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/block 100 40 60 40% /sdcard\n"), nil
	case strings.HasPrefix(script, "mkdir"):
		p.dirs[remotePaths[len(remotePaths)-1]] = true
		return nil, nil
	case strings.HasPrefix(script, "mv"):
		return p.move(remotePaths[0], remotePaths[1])
	case strings.HasPrefix(script, "rm"):
		p.remove(remotePaths[len(remotePaths)-1])
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported shell script: %s", script)
	}
}

func (p *fakePhone) find(base string, directOnly bool) ([]byte, error) {
	if !p.dirs[base] {
		return nil, errors.New("No such file or directory")
	}
	var remotes []string
	for remote := range p.dirs {
		if remote != base && strings.HasPrefix(remote, base+"/") && (!directOnly || path.Dir(remote) == base) {
			remotes = append(remotes, remote)
		}
	}
	for remote := range p.files {
		if strings.HasPrefix(remote, base+"/") && (!directOnly || path.Dir(remote) == base) {
			remotes = append(remotes, remote)
		}
	}
	sort.Strings(remotes)
	var output strings.Builder
	for _, remote := range remotes {
		if p.dirs[remote] {
			fmt.Fprintf(&output, "directory\t0\t1710000000\t%s\n", remote)
		} else {
			fmt.Fprintf(&output, "regular file\t%d\t1710000000\t%s\n", len(p.files[remote]), remote)
		}
	}
	return []byte(output.String()), nil
}

func (p *fakePhone) move(source, target string) ([]byte, error) {
	if !p.exists(source) {
		return nil, errors.New("No such file or directory")
	}
	if p.dirs[source] {
		for remote := range p.dirs {
			if remote == source || strings.HasPrefix(remote, source+"/") {
				rel := strings.TrimPrefix(remote, source)
				p.dirs[target+rel] = true
				delete(p.dirs, remote)
			}
		}
		for remote := range p.files {
			if strings.HasPrefix(remote, source+"/") {
				rel := strings.TrimPrefix(remote, source)
				p.files[target+rel] = p.files[remote]
				delete(p.files, remote)
			}
		}
		return nil, nil
	}
	p.files[target] = p.files[source]
	delete(p.files, source)
	return nil, nil
}

func (p *fakePhone) remove(remote string) {
	for dir := range p.dirs {
		if dir == remote || strings.HasPrefix(dir, remote+"/") {
			delete(p.dirs, dir)
		}
	}
	for file := range p.files {
		if file == remote || strings.HasPrefix(file, remote+"/") {
			delete(p.files, file)
		}
	}
}

func TestManagerOperationsStayOnSelectedPhone(t *testing.T) {
	fake := newFakeADB("phone-a", "phone-b")
	fake.phones["phone-a"].files["/sdcard/readme.txt"] = []byte("hello")
	m := NewManager("adb")
	m.maxUploadBytes = 1024
	m.run = fake.run
	m.stream = fake.stream

	if _, err := m.CreateFolder("phone-a", "", "目标"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Upload("phone-a", "", "上传.txt", "fail", strings.NewReader("uploaded")); err != nil {
		t.Fatal(err)
	}
	item, err := m.Rename("phone-a", "上传.txt", "重命名.txt", "fail")
	if err != nil || item.Path != "重命名.txt" {
		t.Fatalf("rename result=%#v err=%v", item, err)
	}
	item, err = m.Move("phone-a", "重命名.txt", "目标", "fail")
	if err != nil || item.Path != "目标/重命名.txt" {
		t.Fatalf("move result=%#v err=%v", item, err)
	}
	var downloaded strings.Builder
	if err := m.Download("phone-a", item.Path, &downloaded); err != nil || downloaded.String() != "uploaded" {
		t.Fatalf("download=%q err=%v", downloaded.String(), err)
	}
	token, err := m.Delete("phone-a", []string{item.Path})
	if err != nil || token == "" {
		t.Fatalf("delete token=%q err=%v", token, err)
	}
	if _, err := m.stat("phone-a", item.Path); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted item stat error=%v", err)
	}
	if err := m.Undo("phone-a", token); err != nil {
		t.Fatal(err)
	}
	if _, err := m.stat("phone-a", item.Path); err != nil {
		t.Fatalf("undo did not restore item: %v", err)
	}
	result, err := m.List("phone-b", "", "", "", "name", "asc", 1, 100)
	if err != nil || result.Total != 0 {
		t.Fatalf("second phone was affected: result=%#v err=%v", result, err)
	}
}
