//go:build windows

package delegation

import (
	"strings"

	"golang.org/x/sys/windows"
)

func platformCanonicalPath(value string) string {
	path, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return value
	}
	handle, err := windows.CreateFile(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err == nil {
		defer func() { _ = windows.CloseHandle(handle) }()
		buffer := make([]uint16, 32768)
		length, finalErr := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if finalErr == nil && length > 0 && int(length) < len(buffer) {
			return strings.TrimPrefix(windows.UTF16ToString(buffer[:length]), `\\?\`)
		}
	}
	buffer := make([]uint16, 32768)
	length, err := windows.GetLongPathName(path, &buffer[0], uint32(len(buffer)))
	if err != nil || length == 0 || int(length) >= len(buffer) {
		return value
	}
	return windows.UTF16ToString(buffer[:length])
}
