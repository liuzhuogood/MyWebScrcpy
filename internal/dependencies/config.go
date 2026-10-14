package dependencies

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type Paths struct {
	ADB    string `json:"adb"`
	Python string `json:"python"`
}

var config struct {
	sync.RWMutex
	paths Paths
}

const configFile = "dependencies.json"

func Load() error {
	contents, err := os.ReadFile(configFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var value Paths
	if err = json.Unmarshal(contents, &value); err != nil {
		return fmt.Errorf("读取依赖配置失败: %w", err)
	}
	// Missing executables are shown in dependency status, not guessed or replaced.
	for _, path := range []string{value.ADB, value.Python} {
		if path != "" && !filepath.IsAbs(path) {
			return fmt.Errorf("依赖配置必须使用绝对路径")
		}
	}
	config.Lock()
	config.paths = value
	config.Unlock()
	return nil
}
func Current() Paths { config.RLock(); defer config.RUnlock(); return config.paths }
func chosenPath(key, configured string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return configured
}
func Save(value Paths) error {
	for _, path := range []string{value.ADB, value.Python} {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("请选择可执行文件的绝对路径")
		}
		if _, err := checkExecutable(path); err != nil {
			return err
		}
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	config.Lock()
	defer config.Unlock()
	temp, err := os.CreateTemp(".", "dependencies-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(contents); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp.Name(), configFile); err != nil {
		return err
	}
	config.paths = value
	return nil
}
func RegisterSettings(mux *http.ServeMux, authorize func(http.HandlerFunc) http.HandlerFunc) {
	registerManagedSettings(mux, authorize)
	mux.HandleFunc("PUT /api/dependencies/paths", authorize(func(w http.ResponseWriter, r *http.Request) {
		if active != nil {
			guard, err := active.GuardMutation()
			if err != nil {
				http.Error(w, err.Error(), 409)
				return
			}
			defer guard()
		}

		var previous, next Settings
		managedChange := false
		var value Paths
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			http.Error(w, "依赖路径格式错误", http.StatusBadRequest)
			return
		}

		if active != nil && value != Current() {
			adbChanged, pythonChanged := value.ADB != Current().ADB, value.Python != Current().Python
			ids := []string{}
			if adbChanged {
				ids = append(ids, "adb")
			}
			if pythonChanged {
				ids = append(ids, "python")
			}
			release, err := active.exclusive(ids, adbChanged)
			if err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			defer release()
			previous = active.Settings()
			next = previous
			managedChange = true
			if adbChanged {
				next.ADB = Selection{Source: "system", Path: value.ADB}
			}
			if pythonChanged {
				next.Python = Selection{Source: "system", Path: value.Python}
			}
			for _, item := range []struct {
				id        string
				changed   bool
				selection Selection
			}{{"adb", adbChanged, next.ADB}, {"python", pythonChanged, next.Python}} {
				if item.changed {
					if checked := active.detect(item.id, item.selection); !checked.Installed {
						http.Error(w, checked.Error, 400)
						return
					}
				}
			}
			if err := writeJSON(filepath.Join(active.root, "config.json"), next); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		if err := Save(value); err != nil {
			if managedChange {
				if rollbackErr := writeJSON(filepath.Join(active.root, "config.json"), previous); rollbackErr != nil {
					log.Printf("[Dependencies] 旧接口配置回滚失败: %v", rollbackErr)
				}
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if managedChange {
			active.mu.Lock()
			active.settings = next
			active.mu.Unlock()
		}
		log.Print("[Dependencies] 已保存依赖路径；重启 App 后完整生效")
		w.WriteHeader(http.StatusNoContent)
	}))
}
