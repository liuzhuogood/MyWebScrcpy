package pythoncheck

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

const Version = "sdk-v1"

//go:embed validate.py
var sources embed.FS

func Hash() string {
	validator, _ := sources.ReadFile("validate.py")
	sum := sha256.Sum256(validator)
	return hex.EncodeToString(sum[:])
}

func Validate(ctx context.Context, pythonPath, source string) error {
	if len(source) == 0 || len(source) > 256*1024 {
		return fmt.Errorf("Python 源码为空或超过 256 KiB")
	}
	validator, err := sources.ReadFile("validate.py")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, pythonPath, "-I", "-X", "utf8", "-c", string(validator))
	cmd.Stdin = strings.NewReader(source)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Python 校验失败：%s", strings.TrimSpace(string(output)))
	}
	return nil
}
