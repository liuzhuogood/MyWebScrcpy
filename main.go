package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mywebscrcpy/internal/device"
	"mywebscrcpy/internal/ws"
)

//go:embed assets/scrcpy-server
var serverJarFS []byte

//go:embed all:web
var webFS embed.FS

func main() {
	adbPath := findADB()
	addr := "0.0.0.0:8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = "0.0.0.0:" + p
	}

	// 把嵌入的 jar 写到临时文件，供 scrcpy.Server.Push 使用
	jarPath, err := writeJarToTemp()
	if err != nil {
		log.Fatalf("写入 server jar 失败: %v", err)
	}
	defer os.Remove(jarPath)

	dm := device.NewManager(adbPath)
	dm.Start()
	defer dm.Stop()

	hub := ws.NewHub(adbPath, jarPath)

	mux := http.NewServeMux()

	// API: 设备列表
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		devices := dm.Devices()
		w.Header().Set("Content-Type", "application/json")
		if devices == nil {
			devices = []device.Device{}
		}
		json.NewEncoder(w).Encode(devices)
	})

	// API: 旋转设备 (GET /api/rotate?serial=xxx)
	// 通过 adb settings 强制旋转，然后前端会重连获取新方向的视频流
	mux.HandleFunc("/api/rotate", func(w http.ResponseWriter, r *http.Request) {
		serial := r.URL.Query().Get("serial")
		if serial == "" {
			http.Error(w, "missing serial", http.StatusBadRequest)
			return
		}
		// 先杀掉设备上的 scrcpy server，确保重连时全新启动
		exec.Command(adbPath, "-s", serial, "shell", "pkill", "-f", "scrcpy").Run()
		time.Sleep(300 * time.Millisecond)
		// 关闭自动旋转
		exec.Command(adbPath, "-s", serial, "shell", "settings", "put", "system", "accelerometer_rotation", "0").Run()
		// 读取当前旋转值并切换 0↔1
		out, _ := exec.Command(adbPath, "-s", serial, "shell", "settings", "get", "system", "user_rotation").Output()
		current := strings.TrimSpace(string(out))
		newRot := "1"
		if current == "1" {
			newRot = "0"
		}
		exec.Command(adbPath, "-s", serial, "shell", "settings", "put", "system", "user_rotation", newRot).Run()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"rotation": newRot})
	})

	// WebSocket
	mux.HandleFunc("/ws", hub.ServeWS)

	// 静态资源
	webRoot, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	port := strings.TrimPrefix(addr, "0.0.0.0:")
	log.Printf("MyWebScrcpy 启动 → http://localhost:%s (0.0.0.0:%s)", port, port)
	log.Printf("adb 路径: %s", adbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// findADB 查找 adb 可执行文件。
func findADB() string {
	for _, p := range []string{
		os.Getenv("ANDROID_HOME"),
	} {
		if p != "" {
			candidate := filepath.Join(p, "platform-tools", "adb")
			if _, err := exec.LookPath(candidate); err == nil {
				return candidate
			}
		}
	}
	if path, err := exec.LookPath("adb"); err == nil {
		return path
	}
	return "adb"
}

// writeJarToTemp 把嵌入的 server jar 写到临时文件。
func writeJarToTemp() (string, error) {
	f, err := os.CreateTemp("", "scrcpy-server-*.jar")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(serverJarFS); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
