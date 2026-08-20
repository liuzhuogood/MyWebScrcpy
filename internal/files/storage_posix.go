//go:build darwin || linux

package files

import "syscall"

func diskStorage(root string) Storage {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return Storage{}
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - available
	if used < 0 {
		used = 0
	}
	return Storage{Used: used, Total: total, Available: available}
}
