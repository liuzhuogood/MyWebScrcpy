//go:build darwin

package skillsmarket

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameInDirNoReplace(root *os.Root, parentRel, stageName, targetName string) error {
	parentFile, err := root.Open(parentRel)
	if err != nil {
		return err
	}
	defer parentFile.Close()
	// 0x4 is RENAME_EXCL on Darwin
	return unix.RenameatxNp(int(parentFile.Fd()), stageName, int(parentFile.Fd()), targetName, 0x4)
}
