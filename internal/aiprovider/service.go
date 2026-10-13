package aiprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/pibundle"
)

const (
	ConfigKeyAIProvider = "ai_model_config"
)

// ModelConfig AI 模型配置
type ModelConfig struct {
	Provider string `json:"provider"` // "deepseek", "openai", "ollama", "custom"
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	Document string        `json:"document"`
}

type Service struct {
	database        *db.DB
	client          *http.Client
	piRuntime       pibundle.Paths
	RuntimeResolver func() (pibundle.Paths, func(), error)
}

func NewService(database *db.DB, piRuntime pibundle.Paths) *Service {
	return &Service{
		database:  database,
		client:    &http.Client{Timeout: 60 * time.Second},
		piRuntime: piRuntime,
	}
}

// GetConfig 获取保存的配置 (脱敏 API Key)
func (s *Service) GetConfig() (ModelConfig, error) {
	val, err := s.database.GetConfig(ConfigKeyAIProvider)
	if err != nil {
		return ModelConfig{}, err
	}
	var cfg ModelConfig
	if val != "" {
		_ = json.Unmarshal([]byte(val), &cfg)
	}
	// 默认兜底
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.Provider == "" {
		cfg.Provider = "deepseek"
	}
	return cfg, nil
}

// SaveConfig 保存配置
func (s *Service) SaveConfig(cfg ModelConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.database.SetConfig(ConfigKeyAIProvider, string(raw))
}

// TestConnection 测试模型连通性
func (s *Service) TestConnection(ctx context.Context, cfg ModelConfig) (string, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reqURL := baseURL + "/chat/completions"

	bodyData := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 5,
	}
	jsonBody, _ := json.Marshal(bodyData)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络连接失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return "连接成功，模型响应正常", nil
}
