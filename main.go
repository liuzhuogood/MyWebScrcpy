package main

import (
	"crypto/tls"
	"embed"
	"encoding/json"
	"flag"
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

//go:embed assets/certs/cert.pem
var defaultCert []byte

//go:embed assets/certs/key.pem
var defaultKey []byte

func main() {
	httpsFlag := flag.Bool("https", false, "启用 HTTPS（使用内置证书）")
	flag.Parse()

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

	// API: 屏幕状态检测 (GET /api/screen-state?serial=xxx)
	// 通过 adb dumpsys power 获取真实的屏幕电源状态
	mux.HandleFunc("/api/screen-state", func(w http.ResponseWriter, r *http.Request) {
		serial := r.URL.Query().Get("serial")
		if serial == "" {
			http.Error(w, "missing serial", http.StatusBadRequest)
			return
		}
		// 执行 adb shell dumpsys power 获取电源状态
		out, err := exec.Command(adbPath, "-s", serial, "shell", "dumpsys", "power").Output()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"screenOn": nil,
				"error":    err.Error(),
			})
			return
		}
		// 解析输出，查找 "Display Power: state=ON" 或 "Display Power: state=OFF"
		output := string(out)
		screenOn := true // 默认认为屏幕是亮的
		if strings.Contains(output, "Display Power: state=OFF") {
			screenOn = false
		} else if strings.Contains(output, "Display Power: state=ON") {
			screenOn = true
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"screenOn": screenOn,
		})
	})

	// WebSocket
	mux.HandleFunc("/ws", hub.ServeWS)

	// 静态资源
	webRoot, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	port := strings.TrimPrefix(addr, "0.0.0.0:")
	log.Printf("adb 路径: %s", adbPath)

	// TLS 配置
	tlsCertFile := os.Getenv("TLS_CERT")
	tlsKeyFile := os.Getenv("TLS_KEY")

	if *httpsFlag || (tlsCertFile != "" && tlsKeyFile != "") {
		var cert tls.Certificate
		var err error

		if tlsCertFile != "" && tlsKeyFile != "" {
			// 使用自定义证书
			cert, err = tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
			if err != nil {
				log.Fatalf("加载自定义证书失败: %v", err)
			}
			log.Printf("使用自定义证书: %s", tlsCertFile)
		} else {
			// 使用内置证书
			cert, err = tls.X509KeyPair(defaultCert, defaultKey)
			if err != nil {
				log.Fatalf("加载内置证书失败: %v", err)
			}
			log.Printf("使用内置证书（自签名）")
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		listener, err := tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			log.Fatalf("TLS 监听失败: %v", err)
		}
		log.Printf("MyWebScrcpy 启动 → https://localhost:%s (0.0.0.0:%s)", port, port)
		if err := http.Serve(listener, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	} else {
		log.Printf("MyWebScrcpy 启动 → http://localhost:%s (0.0.0.0:%s)", port, port)
		log.Printf("提示: 使用 -https 参数启用 HTTPS，或设置 TLS_CERT/TLS_KEY 环境变量")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
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
