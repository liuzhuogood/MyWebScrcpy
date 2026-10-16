package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mywebscrcpy/internal/adbshell"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/desktop"
	"mywebscrcpy/internal/device"
	"mywebscrcpy/internal/files"
	"mywebscrcpy/internal/inputconfig"
	"mywebscrcpy/internal/matchingservice"
	"mywebscrcpy/internal/operationlog"
	"mywebscrcpy/internal/taskrun"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/uixml"
	"mywebscrcpy/internal/updater"
	"mywebscrcpy/internal/ws"
)

//go:embed VERSION
var versionBytes []byte

//go:embed assets/scrcpy-server
var serverJarFS []byte

//go:embed all:web/dist web/api-docs.html
var webFS embed.FS

//go:embed web/openapi.json
var openAPISpec []byte

//go:embed assets/certs/cert.pem
var defaultCert []byte

//go:embed assets/certs/key.pem
var defaultKey []byte

func main() {
	if handleUpdateCommand() {
		return
	}
	defer updater.MarkExited()
	opts := parseRuntimeOptions()
	if opts.Exit {
		return
	}
	httpsFlag, listenFlag, readyFlag := opts.HTTPS, opts.Listen, opts.Ready
	dataDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dependencyManager, err := dependencies.Initialize(dataDir)
	if err != nil {
		log.Fatalf("读取依赖配置失败: %v", err)
	}
	defer dependencyManager.Close()
	dependencyManager.BeforeMutation = func() error {
		if updatePrepared.Load() {
			return fmt.Errorf("更新正在进行，请稍后修改依赖")
		}
		return nil
	}

	adbPath := findADB()
	addr := "0.0.0.0:8080"
	if *readyFlag != "" {
		port := desktop.ResolvePort(*listenFlag)
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	} else if p := os.Getenv("PORT"); p != "" {
		addr = "0.0.0.0:" + p
	}

	if *listenFlag != "" {
		addr = *listenFlag
	}

	// 把嵌入的 jar 写到临时文件，供 scrcpy.Server.Push 使用
	jarPath, err := writeJarToTemp()
	if err != nil {
		log.Fatalf("写入 server jar 失败: %v", err)
	}
	defer os.Remove(jarPath)

	// 初始化 SQLite 数据库
	dbPath := filepath.Join("data", "mywebscrcpy.db")
	database, err := db.Open(dbPath)
	if err != nil {
		if os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT") != "" {
			log.Fatalf("[Updater] 新版数据库初始化失败: %v", err)
		}
		log.Printf("警告: 初始化 SQLite 数据库失败: %v，将在无持久化模式下运行", err)
	} else {
		defer database.Close()
	}

	dm := device.NewManager(adbPath, database)
	dm.Start()
	defer dm.Stop()

	hub := ws.NewHub(adbPath, jarPath)
	defer hub.Close()

	fileManager := files.NewManager(adbPath)
	uiXML := uixml.New(adbPath)
	uiPage := uipage.New(adbPath)

	mux := http.NewServeMux()

	inputConfig := inputconfig.New(database)
	hub.SetInputConfig(inputConfig)
	hub.RegisterInputConfigRoutes(mux)

	operationLogs := operationlog.New(database)
	defer operationLogs.Close()
	operationLogs.RegisterRoutes(mux)
	dependencies.RegisterRoutes(mux, true)
	if *readyFlag == "" {
		dependencies.RegisterSettings(mux, func(next http.HandlerFunc) http.HandlerFunc { return next })
	}
	mux.HandleFunc("/api/openapi.json", serveOpenAPISpec)

	// 文件管理 API：每个请求都携带当前投屏页面选定的设备 serial，操作手机共享存储。
	files.RegisterRoutes(mux, fileManager)

	// 自动更新 API
	currentVersionStr := strings.TrimSpace(string(versionBytes))
	updaterService, err := updater.NewService(currentVersionStr)
	if err != nil {
		log.Printf("警告: 初始化更新器失败: %v", err)
	} else {
		updater.RegisterRoutes(mux, updaterService)
		updaterService.ConfigureRuntime(serviceUpdatePolicy(*readyFlag != "", addr), func() error { return updatePrepare() }, func() { close(updateStop) }, func() { updateResume() })
	}

	var matchingManager *matchingservice.Manager
	dependencyResume := func() {}
	updatePrepare = func() error {
		if !updatePrepared.CompareAndSwap(false, true) {
			return fmt.Errorf("更新正在进行")
		}
		prepared := false
		freezeDependencies, freezeErr := dependencyManager.BeginMaintenance()
		if freezeErr != nil {
			updatePrepared.Store(false)
			return freezeErr
		}
		dependencyResume = freezeDependencies
		defer func() {
			if !prepared {
				dependencyResume()
				updatePrepared.Store(false)
			}
		}()
		if err := dependencyManager.CheckIdle(); err != nil {
			return err
		}
		if !updateGate.TryLock() {
			return fmt.Errorf("请关闭投屏连接并等待当前操作完成后再更新")
		}
		if matchingManager != nil {
			if err := matchingManager.CheckIdle(); err != nil {
				updateGate.Unlock()
				return err
			}
		}
		if err := hub.CheckUpdateIdle(); err != nil {
			updateGate.Unlock()
			return err
		}
		if database == nil {
			updateGate.Unlock()
			return fmt.Errorf("数据库不可用，无法安全更新")
		}
		prepared = true
		return nil
	}
	updateResume = func() {
		if !updatePrepared.Swap(false) {
			return
		}
		updateGate.Unlock()
		dependencyResume()
	}

	dependencyManager.BeforeADBChange = func() (func(), error) {
		if !updateGate.TryLock() {
			return nil, fmt.Errorf("请关闭投屏连接并等待当前操作完成")
		}
		if matchingManager != nil {
			if err := matchingManager.CheckIdle(); err != nil {
				updateGate.Unlock()
				return nil, err
			}
		}
		if err := hub.CheckDependencyIdle(); err != nil {
			updateGate.Unlock()
			return nil, err
		}
		return func() {
			updateGate.Unlock()
		}, nil
	}

	// 模板管理与匹配 API
	auxiliary := configureAuxiliaryServices(dataDir, database, hub, mux)
	matchingManager = auxiliary.matching
	dependencyManager.BeforePythonChange = auxiliary.stopPythonServices
	defer auxiliary.Close()

	if database == nil {
		registerUnavailableTaskRoutes(mux)
	} else {
		taskRunHandler := taskrun.NewHandler(taskrun.NewService(database))
		taskRunHandler.RegisterRoutes(mux)

	}

	// API: 设备管理
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := dependencies.ADBPath(); err != nil {
				http.Error(w, "ADB 不可用，请在首页系统配置中安装或配置 ADB", http.StatusServiceUnavailable)
				return
			}
			devices := dm.Devices()
			w.Header().Set("Content-Type", "application/json")
			if devices == nil {
				devices = []device.Device{}
			}
			json.NewEncoder(w).Encode(devices)
		case http.MethodPost:
			var body struct {
				Serial string `json:"serial"`
				Alias  string `json:"alias"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			dev, err := dm.AddManualDevice(body.Serial, body.Alias)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(dev)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// API: 单设备操作 (/api/devices/{serial} 或 /api/devices/{serial}/connect)
	mux.HandleFunc("/api/devices/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		parts := strings.Split(path, "/")
		serial := parts[0]
		if serial == "" {
			http.Error(w, "missing serial", http.StatusBadRequest)
			return
		}

		if len(parts) == 2 && parts[1] == "connect" {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := dm.Connect(serial); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}

		switch r.Method {
		case http.MethodPatch:
			var body struct {
				Alias string `json:"alias"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if err := dm.UpdateAlias(serial, body.Alias); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case http.MethodDelete:
			if err := dm.RemoveManualDevice(serial); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			w.Header().Set("Allow", "PATCH, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// API: 获取当前设备的 Android UI XML 快照（只读，按请求执行）。
	mux.Handle("/api/ui/xml", uixml.Handler{
		Service: uiXML,
		IsOnline: func(serial string) bool {
			_, ok := dm.GetDevice(serial)
			return ok
		},
	})

	// API: 获取当前设备的前台页面信息及屏幕指标 (只读，按请求执行)
	mux.Handle("/api/ui/page-info", uipage.Handler{
		Service: uiPage,
		IsOnline: func(serial string) bool {
			_, ok := dm.GetDevice(serial)
			return ok
		},
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
		dependencies.Command(adbPath, "-s", serial, "shell", "pkill", "-f", "scrcpy").Run()
		time.Sleep(300 * time.Millisecond)
		// 关闭自动旋转
		dependencies.Command(adbPath, "-s", serial, "shell", "settings", "put", "system", "accelerometer_rotation", "0").Run()
		// 读取当前旋转值并切换 0↔1
		out, _ := dependencies.Command(adbPath, "-s", serial, "shell", "settings", "get", "system", "user_rotation").Output()
		current := strings.TrimSpace(string(out))
		newRot := "1"
		if current == "1" {
			newRot = "0"
		}
		dependencies.Command(adbPath, "-s", serial, "shell", "settings", "put", "system", "user_rotation", newRot).Run()
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
		out, err := dependencies.Command(adbPath, "-s", serial, "shell", "dumpsys", "power").Output()
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

	// API: 重启设备 (POST /api/reboot?serial=xxx)
	mux.HandleFunc("/api/reboot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serial := r.URL.Query().Get("serial")
		if serial == "" {
			http.Error(w, "missing serial", http.StatusBadRequest)
			return
		}
		if err := dependencies.Command(adbPath, "-s", serial, "reboot").Run(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	// WebSocket
	mux.HandleFunc("/ws", hub.ServeWS)
	hub.RegisterRecordingRoutes(mux)
	hub.RegisterScreenshotRoutes(mux)
	hub.RegisterReplayRoutes(mux)
	hub.RegisterADBCommandRoutes(mux)
	hub.RegisterSendKeyRoute(mux)

	// ADB Shell 终端：WebSocket PTY 桥接，每个 serial 独立 session。
	mux.HandleFunc("/ws/adb-shell", adbshell.Handler(adbPath, func(serial string) bool {
		_, ok := dm.GetDevice(serial)
		return ok
	}))
	// Python Vision 双向帧/结果通道，以及浏览器检测结果通道。
	mux.HandleFunc("/api/vision/stream", hub.ServeVisionWS)
	mux.HandleFunc("/api/vision/results", hub.ServeVisionResults)
	mux.HandleFunc("/api/vision/stats", hub.ServeVisionStats)
	mux.HandleFunc("/api/vision/debug", hub.ServeVisionDebug)

	// 静态资源
	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("初始化 Web 资源失败: %v", err)
	}
	appRoot, err := fs.Sub(webRoot, "dist")
	if err != nil {
		log.Fatalf("初始化前端资源失败: %v", err)
	}
	mux.Handle("/api-docs.html", http.FileServer(http.FS(webRoot)))
	mux.Handle("/", newAppHandler(appRoot))

	auxiliary.restore()
	serveApplication(addr, httpsFlag, readyFlag, mux, adbPath)
}

// findADB 查找 adb 可执行文件。
func findADB() string { return dependencies.ManagedADB }

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

// Preserve the device app's existing non-persistent mode when SQLite cannot be opened.
func registerUnavailableTaskRoutes(mux *http.ServeMux) {
	log.Print("任务数据库不可用，任务运行接口暂停")
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "数据库不可用，请检查服务日志"})
	}
	for _, prefix := range []string{"/api/task-runs", "/api/task-runs/"} {
		mux.HandleFunc(prefix, unavailable)
	}
}
