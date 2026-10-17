package apidocs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

var httpMethods = map[string]bool{
	"get":     true,
	"post":    true,
	"put":     true,
	"delete":  true,
	"patch":   true,
	"head":    true,
	"options": true,
	"trace":   true,
}

// FilterOpenAPISpec 根据配置过滤 OpenAPI 规范中的路径
func FilterOpenAPISpec(fullSpec []byte, cfg Config) ([]byte, error) {
	var spec map[string]interface{}
	if err := json.Unmarshal(fullSpec, &spec); err != nil {
		return nil, fmt.Errorf("解析原始 OpenAPI 失败: %w", err)
	}

	rawPaths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("未找到 paths 对象")
	}

	filteredPaths := make(map[string]interface{})
	usedTags := make(map[string]bool)

	for path, itemRaw := range rawPaths {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		newItem := make(map[string]interface{})
		hasMethod := false

		for k, v := range item {
			lowerK := strings.ToLower(k)
			if httpMethods[lowerK] {
				if cfg.Matches(path, lowerK) {
					newItem[k] = v
					hasMethod = true
					if op, ok := v.(map[string]interface{}); ok {
						if tags, ok := op["tags"].([]interface{}); ok {
							for _, t := range tags {
								if ts, ok := t.(string); ok {
									usedTags[ts] = true
								}
							}
						}
					}
				}
			} else {
				// 保留 parameters 等 path-level 字段，后面如果有 method 保留则一并写入
				newItem[k] = v
			}
		}

		if hasMethod {
			filteredPaths[path] = newItem
		}
	}

	spec["paths"] = filteredPaths

	// 更新 info 信息
	if info, ok := spec["info"].(map[string]interface{}); ok {
		if cfg.Title != "" {
			info["title"] = cfg.Title
		} else {
			info["title"] = "MyWebScrcpy 自动化 API 文档"
		}
		if cfg.Description != "" {
			info["description"] = cfg.Description
		} else {
			info["description"] = "专供自动化控制、测试与脚本调用的核心接口规范。遵循 OpenAPI 3.1.0。"
		}
	}

	// 过滤无引用的 tags
	if rawTags, ok := spec["tags"].([]interface{}); ok && len(usedTags) > 0 {
		var filteredTags []interface{}
		for _, tagRaw := range rawTags {
			if tagObj, ok := tagRaw.(map[string]interface{}); ok {
				if name, ok := tagObj["name"].(string); ok && usedTags[name] {
					filteredTags = append(filteredTags, tagRaw)
				}
			}
		}
		spec["tags"] = filteredTags
	}

	return json.MarshalIndent(spec, "", "  ")
}

// Manager 负责自动化文档的规范加载与热加载过滤
type Manager struct {
	configPath    string
	defaultConfig []byte
	fullSpec      []byte

	mu            sync.RWMutex
	cachedModTime int64
	cachedSpec    []byte
}

// NewManager 创建自动化文档管理器
func NewManager(configPath string, defaultConfig []byte, fullSpec []byte) *Manager {
	return &Manager{
		configPath:    configPath,
		defaultConfig: defaultConfig,
		fullSpec:      fullSpec,
	}
}

// GetAutomationSpec 获取过滤后的自动化 OpenAPI 规范 JSON（若磁盘文件有修改则自动重新计算）
func (m *Manager) GetAutomationSpec() ([]byte, error) {
	var currentModTime int64
	var configData []byte

	if m.configPath != "" {
		if fi, err := os.Stat(m.configPath); err == nil {
			currentModTime = fi.ModTime().UnixNano()
			data, readErr := os.ReadFile(m.configPath)
			if readErr == nil {
				configData = data
			}
		}
	}

	if len(configData) == 0 {
		configData = m.defaultConfig
		currentModTime = -1
	}

	m.mu.RLock()
	if m.cachedSpec != nil && m.cachedModTime == currentModTime {
		spec := m.cachedSpec
		m.mu.RUnlock()
		return spec, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 二次检查
	if m.cachedSpec != nil && m.cachedModTime == currentModTime {
		return m.cachedSpec, nil
	}

	cfg, err := ParseConfig(configData)
	if err != nil {
		return nil, fmt.Errorf("解析自动化接口配置失败: %w", err)
	}

	spec, err := FilterOpenAPISpec(m.fullSpec, cfg)
	if err != nil {
		return nil, fmt.Errorf("过滤自动化接口规范失败: %w", err)
	}

	m.cachedSpec = spec
	m.cachedModTime = currentModTime
	return spec, nil
}

// ServeAutomationOpenAPISpec HTTP Handler 处理自动化 OpenAPI 规范请求
func (m *Manager) ServeAutomationOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	spec, err := m.GetAutomationSpec()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to generate automation openapi spec: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(spec)
}
