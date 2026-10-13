//go:build darwin

package desktop

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	envStartMarker = "__MYWEBSCRCPY_ENV_START__\n"
	envEndMarker   = "__MYWEBSCRCPY_ENV_END__\n"
)

// parseEnvOutput parses the raw stdout of a shell running `env -0` delimited by markers.
func parseEnvOutput(data []byte) map[string]string {
	startTag := []byte(envStartMarker)
	endTag := []byte(envEndMarker)

	startIdx := bytes.Index(data, startTag)
	if startIdx < 0 {
		return nil
	}
	payload := data[startIdx+len(startTag):]
	endIdx := bytes.Index(payload, endTag)
	if endIdx >= 0 {
		payload = payload[:endIdx]
	}

	result := make(map[string]string)
	entries := bytes.Split(payload, []byte{0})
	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		parts := bytes.SplitN(entry, []byte{'='}, 2)
		if len(parts) == 2 {
			result[string(parts[0])] = string(parts[1])
		}
	}
	return result
}

// applyDarwinEnv selectively applies shell environment variables to current process.
func applyDarwinEnv(envMap map[string]string) {
	for k, v := range envMap {
		switch k {
		case "PATH", "ANDROID_HOME", "ANDROID_SDK_ROOT", "NVM_DIR":
			_ = os.Setenv(k, v)
		default:
			if os.Getenv(k) == "" && !strings.HasPrefix(k, "XPC_") && !strings.HasPrefix(k, "__CF") {
				_ = os.Setenv(k, v)
			}
		}
	}
}

// InitEnvironment loads user shell environment variables on macOS when
// the application is launched with minimal launchd/GUI PATH.
func InitEnvironment() {
	path := os.Getenv("PATH")
	if path != "/usr/bin:/bin:/usr/sbin:/sbin" && path != "" {
		return
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", "printf '"+envStartMarker+"'; env -0; printf '"+envEndMarker+"'")
	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		log.Printf("[Desktop] 获取用户 Shell 环境警告: %v", err)
	}

	envMap := parseEnvOutput(buf.Bytes())
	if len(envMap) == 0 {
		log.Printf("[Desktop] 未能解析到 macOS 用户 Shell 环境变量")
		return
	}

	applyDarwinEnv(envMap)
	log.Printf("[Desktop] 已加载 macOS Shell 环境变量 (PATH=%s)", os.Getenv("PATH"))
}
