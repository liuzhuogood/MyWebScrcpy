package apidocs

import (
	"encoding/json"
	"strings"
)

// EndpointRule 定义一条接口匹配规则
type EndpointRule struct {
	Path        string   `json:"path"`
	Methods     []string `json:"methods,omitempty"`
	PrefixMatch bool     `json:"-"`
}

// Config 自动化接口配置
type Config struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Endpoints   []EndpointRule `json:"endpoints"`
}

// rawRule 用于解析既可能是字符串又可能是对象的规则项
type rawRule struct {
	Path    string   `json:"path"`
	Methods []string `json:"methods"`
}

func parseSingleRule(val interface{}) (EndpointRule, bool) {
	switch v := val.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return EndpointRule{}, false
		}
		// 支持 "GET /api/devices" 这种格式
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			method := strings.ToUpper(parts[0])
			path := parts[1]
			prefix := strings.HasSuffix(path, "*")
			if prefix {
				path = strings.TrimSuffix(path, "*")
			}
			return EndpointRule{
				Path:        path,
				Methods:     []string{method},
				PrefixMatch: prefix,
			}, true
		}
		prefix := strings.HasSuffix(s, "*")
		if prefix {
			s = strings.TrimSuffix(s, "*")
		}
		return EndpointRule{
			Path:        s,
			PrefixMatch: prefix,
		}, true

	case map[string]interface{}:
		rawBytes, err := json.Marshal(v)
		if err != nil {
			return EndpointRule{}, false
		}
		var rr rawRule
		if err := json.Unmarshal(rawBytes, &rr); err != nil {
			return EndpointRule{}, false
		}
		p := strings.TrimSpace(rr.Path)
		if p == "" {
			return EndpointRule{}, false
		}
		prefix := strings.HasSuffix(p, "*")
		if prefix {
			p = strings.TrimSuffix(p, "*")
		}
		var methods []string
		for _, m := range rr.Methods {
			m = strings.ToUpper(strings.TrimSpace(m))
			if m != "" {
				methods = append(methods, m)
			}
		}
		return EndpointRule{
			Path:        p,
			Methods:     methods,
			PrefixMatch: prefix,
		}, true
	}
	return EndpointRule{}, false
}

// ParseConfig 从 JSON 数据中解析自动化接口配置
func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return cfg, nil
	}

	// 尝试作为数组解析
	if strings.HasPrefix(trimmed, "[") {
		var list []interface{}
		if err := json.Unmarshal(data, &list); err != nil {
			return cfg, err
		}
		for _, item := range list {
			if rule, ok := parseSingleRule(item); ok {
				cfg.Endpoints = append(cfg.Endpoints, rule)
			}
		}
		return cfg, nil
	}

	// 作为对象解析
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return cfg, err
	}
	if t, ok := obj["title"].(string); ok {
		cfg.Title = t
	}
	if d, ok := obj["description"].(string); ok {
		cfg.Description = d
	}

	var rawList []interface{}
	for _, key := range []string{"endpoints", "apis", "paths"} {
		if val, exists := obj[key]; exists {
			if arr, ok := val.([]interface{}); ok {
				rawList = arr
				break
			}
		}
	}

	for _, item := range rawList {
		if rule, ok := parseSingleRule(item); ok {
			cfg.Endpoints = append(cfg.Endpoints, rule)
		}
	}

	return cfg, nil
}

// Matches 判断指定路径和 HTTP 方法是否符合配置规则
func (c *Config) Matches(path string, method string) bool {
	if len(c.Endpoints) == 0 {
		return false
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, r := range c.Endpoints {
		var pathMatched bool
		if r.PrefixMatch {
			pathMatched = strings.HasPrefix(path, r.Path)
		} else {
			pathMatched = (path == r.Path)
		}
		if !pathMatched {
			continue
		}

		if len(r.Methods) == 0 {
			return true
		}
		for _, m := range r.Methods {
			if m == method {
				return true
			}
		}
	}
	return false
}
