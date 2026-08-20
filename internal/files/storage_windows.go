//go:build windows

package files

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func diskStorage(root string) Storage {
	directory, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return Storage{}
	}

	var available, total, totalFree uint64
	result, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(directory)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if result == 0 {
		return Storage{}
	}

	used := total - totalFree
	return Storage{Used: int64(used), Total: int64(total), Available: int64(available)}
}
