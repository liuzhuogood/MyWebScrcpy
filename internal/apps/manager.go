package apps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mywebscrcpy/internal/dependencies"
	"os"
	"regexp"
	"strings"
)

var (
	ErrMissingSerial = errors.New("未指定设备序列号")
	ErrEmptyFile     = errors.New("APK 文件内容为空")
)

type commandRunner func(ctx context.Context, args ...string) ([]byte, error)

type Manager struct {
	adbPath string
	run     commandRunner
}

func NewManager(adbPath string) *Manager {
	return &Manager{
		adbPath: adbPath,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			return dependencies.CommandContext(ctx, adbPath, args...).CombinedOutput()
		},
	}
}

// NewManagerWithRunner 用于单元测试注入自定义执行器。
func NewManagerWithRunner(run commandRunner) *Manager {
	return &Manager{
		adbPath: "adb",
		run:     run,
	}
}

var failureCodeRegex = regexp.MustCompile(`\b(INSTALL_[A-Z0-9_]+)\b`)

// ParseInstallOutput 解析 ADB 安装输出并映射为友好中文提示。
func ParseInstallOutput(output string) (success bool, code string, friendlyMsg string) {
	if strings.Contains(output, "Success") {
		return true, "", "应用安装成功"
	}

	matches := failureCodeRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		code = matches[1]
	} else if strings.Contains(output, "INSTALL_PARSE_FAILED") {
		code = "INSTALL_PARSE_FAILED"
	} else if strings.Contains(output, "Failure") {
		code = "INSTALL_FAILED_UNKNOWN"
	}

	switch code {
	case "INSTALL_FAILED_VERSION_DOWNGRADE":
		friendlyMsg = "安装包版本低于设备现有版本，请勾选【允许版本降级】"
	case "INSTALL_FAILED_ALREADY_EXISTS":
		friendlyMsg = "相同应用已存在，请勾选【保留数据覆盖安装】"
	case "INSTALL_FAILED_INSUFFICIENT_STORAGE":
		friendlyMsg = "设备存储空间不足，无法完成安装"
	case "INSTALL_FAILED_INVALID_APK":
		friendlyMsg = "APK 文件损坏或签名无效"
	case "INSTALL_FAILED_UPDATE_INCOMPATIBLE":
		friendlyMsg = "签名与已安装版本不一致，需先卸载旧版本应用"
	case "INSTALL_FAILED_OLDER_SDK":
		friendlyMsg = "设备系统版本过低，不支持此应用"
	case "INSTALL_FAILED_NO_MATCHING_ABIS":
		friendlyMsg = "设备的 CPU 架构不支持该应用原生库"
	case "INSTALL_FAILED_TEST_ONLY":
		friendlyMsg = "此应用被标记为测试专有包 (test-only)"
	case "INSTALL_PARSE_FAILED", "INSTALL_PARSE_FAILED_NOT_APK":
		friendlyMsg = "APK 解析失败，文件可能不是有效的 Android 安装包"
	default:
		if code != "" {
			friendlyMsg = fmt.Sprintf("安装失败 (%s)", code)
		} else {
			friendlyMsg = "安装失败，请检查设备连接或 APK 文件"
		}
	}

	return false, code, friendlyMsg
}

// BuildInstallArgs 装配 adb install 命令参数。
func BuildInstallArgs(serial string, apkPath string, opts InstallOptions) []string {
	args := []string{"-s", serial, "install"}
	if opts.ReplaceExisting {
		args = append(args, "-r")
	}
	// 默认携带 -t 允许安装测试包
	args = append(args, "-t")
	if opts.GrantPermissions {
		args = append(args, "-g")
	}
	if opts.AllowDowngrade {
		args = append(args, "-d")
	}
	args = append(args, apkPath)
	return args
}

// Install 将传入的 APK 流写入临时文件并调用 adb install 执行安装。
func (m *Manager) Install(ctx context.Context, serial string, r io.Reader, filename string, opts InstallOptions) (*InstallResult, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil, ErrMissingSerial
	}

	// 写入本地临时文件
	tmpFile, err := os.CreateTemp("", "mywebscrcpy-install-*.apk")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	written, copyErr := io.Copy(tmpFile, r)
	_ = tmpFile.Close() // 必须在执行 adb install 之前关闭文件句柄
	if copyErr != nil {
		return nil, fmt.Errorf("传输 APK 数据失败: %w", copyErr)
	}
	if written == 0 {
		return nil, ErrEmptyFile
	}

	args := BuildInstallArgs(serial, tmpPath, opts)
	outBytes, cmdErr := m.run(ctx, args...)
	output := strings.TrimSpace(string(outBytes))

	success, code, friendlyMsg := ParseInstallOutput(output)
	if !success {
		if cmdErr != nil && output == "" {
			output = cmdErr.Error()
		}
		return &InstallResult{
			OK:      false,
			Message: friendlyMsg,
			Error:   code,
			Output:  output,
		}, nil
	}

	return &InstallResult{
		OK:      true,
		Message: friendlyMsg,
		Output:  output,
	}, nil
}
