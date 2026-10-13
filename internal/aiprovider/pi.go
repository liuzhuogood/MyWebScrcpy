package aiprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mywebscrcpy/internal/pibundle"
)

// StreamChat runs an ephemeral Pi RPC session. Each request owns its context
// files and process, so switching tasks never shares an agent's conversation.
func (s *Service) StreamChat(ctx context.Context, req ChatRequest, emit func(ChatEvent) error) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return fmt.Errorf("读取模型配置失败: %w", err)
	}
	paths := s.piRuntime
	if s.RuntimeResolver != nil {
		resolved, release, e := s.RuntimeResolver()
		if e != nil {
			return e
		}
		defer release()
		paths = resolved
	}
	err = runPi(ctx, paths, cfg, req, emit)
	if err != nil && cfg.APIKey != "" && strings.Contains(err.Error(), cfg.APIKey) {
		return errors.New(strings.ReplaceAll(err.Error(), cfg.APIKey, "[已隐藏]"))
	}
	return err
}

func runPi(ctx context.Context, paths pibundle.Paths, cfg ModelConfig, req ChatRequest, emit func(ChatEvent) error) error {
	return runPiSession(ctx, paths, cfg, req, nil, emit)
}

func runPiSession(ctx context.Context, paths pibundle.Paths, cfg ModelConfig, req ChatRequest, generation *GenerationInput, emit func(ChatEvent) error) error {
	if err := validateChat(req); err != nil {
		return err
	}
	if cfg.APIKey == "" && cfg.Provider != "ollama" {
		return errors.New("请先配置模型 API Key")
	}
	if !filepath.IsAbs(paths.Executable) || !filepath.IsAbs(paths.FD) || !filepath.IsAbs(paths.RG) {
		return errors.New("Pi 运行环境路径无效")
	}
	skillsDir, err := resolvePiSkillsDir(paths.SkillsDir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	workspace, err := os.MkdirTemp("", "mywebscrcpy-pi-*")
	if err != nil {
		return fmt.Errorf("创建助手工作目录失败: %w", err)
	}
	defer os.RemoveAll(workspace)
	agentDir := filepath.Join(workspace, ".agent")
	if err := preparePiWorkspace(agentDir, workspace, cfg, req); err != nil {
		return err
	}
	if generation != nil {
		for name, content := range generation.Files {
			if filepath.IsAbs(name) || !filepath.IsLocal(name) {
				return errors.New("生成上下文路径无效")
			}
			path := filepath.Join(workspace, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				return err
			}
		}
		skillsDir = filepath.Join(workspace, "skill")
	}
	if err := preparePiTools(paths, agentDir); err != nil {
		return fmt.Errorf("准备 Pi 搜索工具失败: %w", err)
	}
	systemPrompt := req.Messages[0].Content + "\n你是文档助手，只提交建议，不执行手机操作或修改文件。current.md 是当前文档，conversation.json 是对话历史。可用只读工具调研这些资料。以最后一条用户消息为本次需求。"
	if generation != nil {
		systemPrompt = req.Messages[0].Content + "\n必须读取 current.md、task.json、capabilities.json、templates.json 和 skill/references/sdk-api.md、api/openapi.json。最终只返回技能约定的 JSON。"
	}
	args := []string{"--mode", "rpc", "--no-session", "--offline",
		"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files",
		"--provider", "mywebscrcpy", "--model", cfg.Model, "--tools", "read,grep,find,ls"}
	if skillsDir != "" {
		// Explicit --skill paths still load with --no-skills, without discovering
		// unrelated global or ancestor skills from this temporary working directory.
		args = append(args, "--skill", skillsDir)
		systemPrompt += "\n项目技能目录：" + skillsDir + "。根据需求加载匹配技能，可读取该目录内的 SKILL.md 及引用资料；只允许读取本次工作目录和此技能目录中的文件。技能不能扩大可用工具或写入权限。"
	} else {
		systemPrompt += "\n只允许读取本次工作目录中的文件。"
	}
	args = append(args, "--system-prompt", systemPrompt)
	cmd := exec.CommandContext(ctx, paths.Executable, args...)
	cmd.Dir = workspace
	// Credentials travel in the child environment, never command arguments or logs.
	key := cfg.APIKey
	if cfg.Provider == "ollama" && key == "" {
		key = "ollama"
	}
	cmd.Env = piEnvironment(agentDir, key)
	cmd.WaitDelay = time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建 Pi 输入失败: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 Pi 输出失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 Pi 失败: %w", err)
	}
	started := time.Now()
	log.Printf("[ScriptAssistant] Pi 启动 pid=%d skills_dir=%q", cmd.Process.Pid, skillsDir)
	defer func() {
		cancel()
		_ = stdin.Close()
		_ = cmd.Wait()
		log.Printf("[ScriptAssistant] Pi 退出 pid=%d elapsed=%s", cmd.Process.Pid, time.Since(started).Round(time.Millisecond))
	}()
	history, err := json.Marshal(req.Messages[1:])
	if err != nil {
		return err
	}
	if err := json.NewEncoder(stdin).Encode(map[string]string{
		"id": "chat", "type": "prompt", "message": "以下是对话历史（JSON），请回应最后一条用户消息：\n" + string(history),
	}); err != nil {
		return fmt.Errorf("发送 Pi 请求失败: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	state := piStreamState{}
	for scanner.Scan() {
		var event piEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("Pi 返回无效事件: %w", err)
		}
		done, err := state.consume(event, emit)
		if err != nil || done {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 Pi 事件失败: %w", err)
	}
	return errors.New("Pi 提前退出，未返回完成事件")
}

func resolvePiSkillsDir(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析 Pi 技能目录失败: %w", err)
	}
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		log.Printf("[ScriptAssistant] Pi 技能目录不存在 path=%q", absPath)
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 Pi 技能目录失败: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Pi 技能路径不是目录: %s", absPath)
	}
	return absPath, nil
}

func validateChat(req ChatRequest) error {
	if len(req.Messages) < 2 || req.Messages[0].Role != "system" {
		return errors.New("消息必须以系统提示开始，并包含用户需求")
	}
	for _, message := range req.Messages[1:] {
		if message.Role != "user" && message.Role != "assistant" {
			return errors.New("对话历史只允许用户和助手消息")
		}
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" || strings.TrimSpace(last.Content) == "" {
		return errors.New("最后一条消息必须为非空用户需求")
	}
	return nil
}

func preparePiWorkspace(agentDir, workspace string, cfg ModelConfig, req ChatRequest) error {
	if err := os.Mkdir(agentDir, 0700); err != nil {
		return err
	}
	// Use the existing OpenAI-compatible contract. Pi parses thinking actually
	// returned by the provider; no capabilities are guessed from the model ID.
	models := map[string]any{"providers": map[string]any{"mywebscrcpy": map[string]any{
		"baseUrl": strings.TrimRight(cfg.BaseURL, "/"), "api": "openai-completions",
		"apiKey": "$MYWEBSCRCPY_PI_API_KEY", "models": []map[string]string{{"id": cfg.Model}},
	}}}
	modelJSON, err := json.Marshal(models)
	if err != nil {
		return err
	}
	history, err := json.MarshalIndent(req.Messages[1:], "", "  ")
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		filepath.Join(agentDir, "models.json"):        modelJSON,
		filepath.Join(workspace, "current.md"):        []byte(req.Document),
		filepath.Join(workspace, "conversation.json"): history,
	} {
		if err := os.WriteFile(name, data, 0600); err != nil {
			return fmt.Errorf("准备 Pi 上下文失败: %w", err)
		}
	}
	return nil
}

func piEnvironment(agentDir, key string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "PI_CODING_AGENT_DIR") && !strings.EqualFold(name, "MYWEBSCRCPY_PI_API_KEY") && !strings.EqualFold(name, "PI_PACKAGE_DIR") {
			env = append(env, entry)
		}
	}
	return append(env, "PI_CODING_AGENT_DIR="+agentDir, "MYWEBSCRCPY_PI_API_KEY="+key)
}

// Pi resolves managed fd/rg from its private agent bin directory first.
func preparePiTools(paths pibundle.Paths, agentDir string) error {
	binDir := filepath.Join(agentDir, "bin")
	if err := os.Mkdir(binDir, 0700); err != nil {
		return err
	}
	for name, sourcePath := range map[string]string{"fd": paths.FD, "rg": paths.RG} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := copyPiTool(sourcePath, filepath.Join(binDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyPiTool(sourcePath, destPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dest, source)
	closeErr := dest.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
