package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mywebscrcpy/internal/adbshell"
	"mywebscrcpy/internal/aiprovider"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/device"
	"mywebscrcpy/internal/files"
	"mywebscrcpy/internal/mdscript"
	"mywebscrcpy/internal/pibundle"
	"mywebscrcpy/internal/scheduler"
	"mywebscrcpy/internal/scripts"
	"mywebscrcpy/internal/taskmanager"
	"mywebscrcpy/internal/template"
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

//go:embed all:.pi/skills
var piSkillsFS embed.FS

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
	executable, err := os.Executable()
	if err != nil {
		log.Fatalf("定位程序目录失败: %v", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		log.Fatalf("解析程序路径失败: %v", err)
	}
	piSkills, err := fs.Sub(piSkillsFS, ".pi/skills")
	if err != nil {
		log.Fatalf("读取内置技能失败: %v", err)
	}
	piRuntime, err := pibundle.Prepare(filepath.Dir(executable), piSkills)
	if err != nil {
		log.Fatalf("初始化内置 Pi 失败: %v", err)
	}

	adbPath := findADB()
	addr := "0.0.0.0:8080"
	if p := os.Getenv("PORT"); p != "" {
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

	// 初始化脚本管理目录
	if err := scripts.EnsureRoot(); err != nil {
		log.Printf("警告: 初始化脚本目录失败: %v", err)
	}
	fileManager := files.NewManager(adbPath)
	uiXML := uixml.New(adbPath)
	uiPage := uipage.New(adbPath)

	mux := http.NewServeMux()
	dependencies.RegisterRoutes(mux, *readyFlag != "")
	mux.HandleFunc("/api/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(openAPISpec)
	})

	// 脚本管理 API
	scripts.RegisterRoutes(mux)
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

	var updateTasks *taskmanager.Manager
	updatePrepare = func() error {
		if !updatePrepared.CompareAndSwap(false, true) {
			return fmt.Errorf("更新正在进行")
		}
		prepared := false
		defer func() {
			if !prepared {
				updatePrepared.Store(false)
			}
		}()
		if !updateGate.TryLock() {
			return fmt.Errorf("请关闭投屏连接并等待当前操作完成后再更新")
		}
		if err := hub.CheckUpdateIdle(); err != nil {
			updateGate.Unlock()
			return err
		}
		if database == nil {
			updateGate.Unlock()
			return fmt.Errorf("数据库不可用，无法安全更新")
		}
		if updateTasks != nil {
			if err := updateTasks.BeginMaintenance(); err != nil {
				updateGate.Unlock()
				return err
			}
		}
		prepared = true
		return nil
	}
	updateResume = func() {
		if !updatePrepared.Swap(false) {
			return
		}
		if updateTasks != nil {
			updateTasks.EndMaintenance()
		}
		updateGate.Unlock()
	}

	// 模板管理与匹配 API
	templateStorage, err := template.NewStorage("")
	if err != nil {
		log.Printf("警告: 初始化模板存储失败: %v，尝试回退到临时目录", err)
		fallbackDir := filepath.Join(os.TempDir(), "mywebscrcpy-templates")
		templateStorage, err = template.NewStorage(fallbackDir)
		if err != nil {
			log.Fatalf("致命错误: 模板存储完全无法初始化: %v", err)
		}
	}
	log.Printf("模板存储目录: %s", templateStorage.BaseDir())
	templateService := template.NewService(templateStorage)
	defer templateService.Stop()
	template.RegisterRoutes(mux, templateService)

	if database == nil {
		registerUnavailableScriptRoutes(mux)
	} else {
		// Markdown 脚本与任务管理器 API
		taskMgr := taskmanager.NewManager(database)
		updateTasks = taskMgr
		taskMgr.Artifacts.SetRecorder(hub)
		if err := taskMgr.Recover(); err != nil {
			log.Fatalf("任务产物恢复失败: %v", err)
		}
		defer taskMgr.Close()
		taskHandler := taskmanager.NewHandler(taskMgr)
		taskHandler.RegisterRoutes(mux)

		executor := mdscript.NewExecutor(database, taskMgr, func(serial string) (bool, bool) {
			device, known := dm.GetDevice(serial)
			return known, known && device.State == "device"
		}, func(serial, id string) error {
			asset, err := templateStorage.GetTemplate(id)
			if err != nil {
				return fmt.Errorf("读取模板 %s 失败: %w", id, err)
			}
			if asset == nil {
				return fmt.Errorf("模板不存在: %s", id)
			}
			if !asset.Enabled {
				return fmt.Errorf("模板已停用: %s", asset.Name)
			}
			if asset.Serial != template.GlobalSerial && asset.Serial != serial {
				return fmt.Errorf("模板 %s 不属于设备 %s", asset.Name, serial)
			}
			return nil
		})
		// 内嵌 Cron 调度器
		sched := scheduler.NewScheduler(database, executor)
		defer startUpdateAwareScheduler(sched)()

		schedHandler := scheduler.NewHandler(sched)
		schedHandler.RegisterRoutes(mux)

		mdHandler := mdscript.NewHandler(database, executor, sched)
		mdHandler.RegisterRoutes(mux)

		// AI 模型配置与代理 API
		aiSvc := aiprovider.NewService(database, piRuntime)
		taskMgr.Artifacts.ConfigureReports(aiSvc.SummarizeTask)
		aiHandler := aiprovider.NewHandler(aiSvc)
		aiHandler.RegisterRoutes(mux)
	}

	// API: 设备管理
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
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
		if err := exec.Command(adbPath, "-s", serial, "reboot").Run(); err != nil {
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

	serveApplication(addr, httpsFlag, readyFlag, mux, adbPath)
}

// findADB 查找 adb 可执行文件。
func findADB() string {
	path, err := dependencies.ADBPath()
	if err == nil {
		return path
	}
	if value := os.Getenv("MYWEBSCRCPY_ADB"); value != "" {
		log.Printf("ADB 配置不可用: %v", err)
		return value
	}
	if value := dependencies.Current().ADB; value != "" {
		log.Printf("ADB 配置不可用: %v", err)
		return value
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

// Preserve the device app's existing non-persistent mode when script migration cannot open SQLite.
func registerUnavailableScriptRoutes(mux *http.ServeMux) {
	log.Print("脚本数据库不可用，脚本、调度和文档助手接口暂停")
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "脚本数据库不可用，请检查服务日志"})
	}
	for _, prefix := range []string{"/api/md-scripts", "/api/md-scripts/", "/api/tasks/", "/api/scheduler/", "/api/ai/"} {
		mux.HandleFunc(prefix, unavailable)
	}
}
