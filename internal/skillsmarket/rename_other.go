//go:build !darwin && !linux && !windows

package skillsmarket

import (
	"errors"
	"fmt"
	"os"
)

func renameInDirNoReplace(_ *os.Root, _, _, _ string) error {
	return fmt.Errorf("当前平台不支持安全安装技能: %w", errors.ErrUnsupported)
}
