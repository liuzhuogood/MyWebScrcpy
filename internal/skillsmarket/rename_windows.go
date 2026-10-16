//go:build windows

package skillsmarket

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILE_RENAME_INFORMATION layout, including the variable-length UTF-16 name.
// ReplaceIfExists remains zero: the publication itself rejects an existing target.
type skillRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameInDirNoReplace(root *os.Root, parentRel, stageName, targetName string) error {
	parent, err := root.Open(parentRel)
	if err != nil {
		return err
	}
	defer parent.Close()
	stage, err := windows.NewNTUnicodeString(stageName)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: windows.Handle(parent.Fd()), ObjectName: stage,
		Attributes: windows.OBJ_DONT_REPARSE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.DELETE|windows.SYNCHRONIZE|windows.FILE_READ_ATTRIBUTES,
		&attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("暂存目录为重解析点，已拒绝安装")
	}
	name, err := windows.UTF16FromString(targetName)
	if err != nil {
		return err
	}
	var layout skillRenameInformation
	nameBytes := (len(name) - 1) * 2
	buffer := make([]byte, int(unsafe.Offsetof(layout.FileName))+nameBytes)
	rename := (*skillRenameInformation)(unsafe.Pointer(&buffer[0]))
	rename.RootDirectory = windows.Handle(parent.Fd())
	rename.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&rename.FileName[0], len(name)-1), name[:len(name)-1])
	return windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation)
}
