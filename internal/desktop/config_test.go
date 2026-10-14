package desktop

import (
	"os"
	"testing"
)

func TestPortValidation(t *testing.T) {
	if err := ValidatePort(1989); err != nil {
		t.Fatalf("1989 应该有效: %v", err)
	}
	if err := ValidatePort(80); err != nil {
		t.Fatalf("80 应该有效: %v", err)
	}
	if err := ValidatePort(65535); err != nil {
		t.Fatalf("65535 应该有效: %v", err)
	}
	if err := ValidatePort(0); err == nil {
		t.Fatal("0 应该无效")
	}
	if err := ValidatePort(-1); err == nil {
		t.Fatal("-1 应该无效")
	}
	if err := ValidatePort(65536); err == nil {
		t.Fatal("65536 应该无效")
	}
}

func TestConfigLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// 默认配置
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("加载默认配置失败: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("期望默认端口 %d，实际 %d", DefaultPort, cfg.Port)
	}

	// 保存并重新加载
	if err := SaveConfig(Config{Port: 2024}); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("重新加载配置失败: %v", err)
	}
	if cfg.Port != 2024 {
		t.Fatalf("期望端口 2024，实际 %d", cfg.Port)
	}

	// 保存非法端口应失败
	if err := SaveConfig(Config{Port: 99999}); err == nil {
		t.Fatal("保存非法端口应该报错")
	}
}

func TestResolvePort(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	os.Unsetenv("MYWEBSCRCPY_DESKTOP_PORT")
	os.Unsetenv("PORT")

	// 1. 无配置无环境 -> 1989
	if p := ResolvePort(""); p != DefaultPort {
		t.Fatalf("期望默认端口 %d，实际 %d", DefaultPort, p)
	}

	// 2. 配置文件配置 -> 3000
	_ = SaveConfig(Config{Port: 3000})
	if p := ResolvePort(""); p != 3000 {
		t.Fatalf("期望配置文件端口 3000，实际 %d", p)
	}

	// 3. 环境变量 PORT -> 4000
	t.Setenv("PORT", "4000")
	if p := ResolvePort(""); p != 4000 {
		t.Fatalf("期望环境变量 PORT 4000，实际 %d", p)
	}

	// 4. 环境变量 MYWEBSCRCPY_DESKTOP_PORT -> 5000 (优先级高于 PORT)
	t.Setenv("MYWEBSCRCPY_DESKTOP_PORT", "5000")
	if p := ResolvePort(""); p != 5000 {
		t.Fatalf("期望环境变量 MYWEBSCRCPY_DESKTOP_PORT 5000，实际 %d", p)
	}

	// 5. 显式参数 -> 6000
	if p := ResolvePort("127.0.0.1:6000"); p != 6000 {
		t.Fatalf("期望显式参数端口 6000，实际 %d", p)
	}
}
