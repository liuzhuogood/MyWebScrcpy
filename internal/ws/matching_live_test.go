package ws

import (
	"bufio"
	"context"
	"encoding/json"
	"mywebscrcpy/internal/matchingservice"
	"mywebscrcpy/internal/template"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Explicit opt-in is restricted to the phone authorized for read-only acceptance.
func TestLiveMatchingServiceReadOnly(t *testing.T) {
	if os.Getenv("MYWEBSCRCPY_MATCHING_LIVE_SERIAL") != "10.0.0.30:5555" {
		t.Skip("requires explicit Xiaomi read-only opt-in")
	}
	python := os.Getenv("MYWEBSCRCPY_MATCHING_TEST_PYTHON")
	adb := os.Getenv("MYWEBSCRCPY_MATCHING_LIVE_ADB")
	if !filepath.IsAbs(python) || !filepath.IsAbs(adb) {
		t.Fatal("explicit absolute Python/ADB paths required")
	}
	jar, err := filepath.Abs("../../assets/scrcpy-server")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(adb, jar)
	defer h.Close()
	// Isolate forwarding from any already running MyWebScrcpy instance.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.nextPort = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	storage, err := template.NewStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	service := template.NewService(storage)
	defer service.Stop()
	mux := http.NewServeMux()
	template.RegisterRoutes(mux, service)
	mux.HandleFunc("/api/vision/stream", h.ServeVisionWS)
	mux.HandleFunc("/api/vision/results", h.ServeVisionResults)
	server := httptest.NewServer(mux)
	defer server.Close()
	m := matchingservice.New(matchingservice.Options{Python: func() (string, func(), error) { return python, func() {}, nil }, Devices: h.MatchingDevices, API: mux, Vision: http.HandlerFunc(h.ServeMatchingVisionWS), TemplateReadable: func(id, serial string) bool {
		asset, err := storage.GetTemplate(id)
		return err == nil && asset.Serial == serial
	}})
	defer m.Close()
	source := `import sys,json,time,threading
import cv2,numpy as np,websocket
from mywebscrcpy import Client
url=sys.argv[1]
with Client(url) as client:
 with client.device('10.0.0.30:5555') as device:
  frame=device.frame(timeout=20)
  for attempt in range(5):
   if float(np.max(np.std(frame.image.astype(np.float64),axis=(0,1))))>1:break
   frame=device.frame(timeout=10)
  image=frame.image
  # Select a textured interior tile; retain no phone screenshot outside the temporary asset store.
  candidates=[image[y:y+48,x:x+80] for y in range(0,frame.height-48,24) for x in range(20,frame.width-100,80)]
  crop=max(candidates,key=lambda tile:float(np.max(np.std(tile.astype(np.float64),axis=(0,1)))))
  png=cv2.imencode('.png',crop)[1].tobytes()
  asset=client.templates.create('只读验收',png,serial='10.0.0.30:5555',threshold=.85,grayscale=False)
  peer=None
  try:
   peer=websocket.create_connection(url.replace('http:','ws:')+'/api/vision/results?serial=10.0.0.30:5555',timeout=80,http_no_proxy=['*'])
   print(json.dumps(dict(type='prepared',width=frame.width,height=frame.height,texture=float(np.max(np.std(crop.astype(np.float64),axis=(0,1)))))),flush=True)
   deadline=time.monotonic()+80
   while time.monotonic()<deadline:
    value=json.loads(peer.recv())
    if value['type']=='detection.result' and value['objects']:
     confidence=max(o['confidence'] for o in value['objects'])
     print(json.dumps(dict(type='matched',confidence=confidence,objects=len(value['objects']))),flush=True)
     break
   else: raise RuntimeError('matching result timeout')
   sys.stdin.buffer.read()
  finally:
   if peer is not None:peer.close()
   client.templates.delete(asset.id)
print(json.dumps(dict(type='closed',receivers=sum(t.name.startswith('mywebscrcpy-vision-') for t in threading.enumerate()))),flush=True)
`
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-u", "-c", source, server.URL)
	sdk, err := filepath.Abs("../../packages/python-sdk/src")
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), "PYTHONPATH="+sdk)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	scanner := bufio.NewScanner(stdout)
	read := func() map[string]any {
		t.Helper()
		if !scanner.Scan() {
			t.Fatal("live verifier exited before event")
		}
		var event map[string]any
		if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		t.Logf("live event: %v", event)
		return event
	}
	if read()["type"] != "prepared" {
		t.Fatal("phone frame unavailable")
	}
	if err = m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if result := read(); result["type"] != "matched" || result["confidence"].(float64) < .85 {
		t.Fatalf("no match: %v", result)
	}
	if err = m.Stop(); err != nil {
		t.Fatal(err)
	}
	stdin.Close()
	if result := read(); result["type"] != "closed" || result["receivers"] != float64(0) {
		t.Fatalf("receiver leak: %v", result)
	}
	if err = cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if m.Status().PID != 0 || len(storage.ListAll()) != 0 {
		t.Fatal("process/template not cleaned")
	}
}
