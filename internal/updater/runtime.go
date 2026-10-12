package updater

import (
	"os"
	"path/filepath"
	"strings"
)

type Policy struct {
	Mode   string `json:"runtime_mode"`
	Method string `json:"update_method"`
	Reason string `json:"update_reason"`
}

// DetectPolicy uses an explicit container declaration and known container markers.
func DetectPolicy(desktop bool) Policy {
	if os.Getenv("MYWEBSCRCPY_RUNTIME") == "docker" || fileExists("/.dockerenv") || fileExists("/run/.containerenv") {
		return Policy{"docker", "image", "请更新镜像并重建容器"}
	}
	if desktop {
		return Policy{"desktop", "app", "请在 App 内使用完整应用更新"}
	}
	exe, _ := os.Executable()
	if strings.Contains(filepath.ToSlash(exe), ".app/Contents/") || fileExists(filepath.Join(filepath.Dir(exe), "mywebscrcpy-gui")) || fileExists(filepath.Join(filepath.Dir(exe), "mywebscrcpy-gui.exe")) {
		return Policy{"desktop-service", "installer", "请使用完整桌面安装包更新"}
	}
	if os.Getenv("INVOCATION_ID") != "" || os.Getenv("MYWEBSCRCPY_RUNTIME") == "systemd" {
		return Policy{"systemd", "managed", "请通过服务部署流程更新并由 systemd 重启"}
	}
	return Policy{"standalone", "binary", ""}
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
