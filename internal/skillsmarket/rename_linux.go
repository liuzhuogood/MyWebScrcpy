//go:build linux

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
	return unix.Renameat2(int(parentFile.Fd()), stageName, int(parentFile.Fd()), targetName, unix.RENAME_NOREPLACE)
}
