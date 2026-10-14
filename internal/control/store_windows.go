package control

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// windowsFilePath preserves Unicode and extended local/UNC paths for native file operations.
func windowsFilePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absolute, `\\?\`) {
		if strings.HasPrefix(absolute, `\\`) {
			absolute = `\\?\UNC\` + absolute[2:]
		} else {
			absolute = `\\?\` + absolute
		}
	}
	return absolute, nil
}

// openStoreObject opens a read-only snapshot while permitting atomic replacement by a writer.
// Delete sharing changes handle coexistence, not ACLs; open readers retain the old file bytes.
// A reparse point is not followed, and readObject verifies the opened handle is a regular file.
func openStoreObject(path string) (*os.File, error) {
	absolute, err := windowsFilePath(path)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		windows.CloseHandle(handle)
		return nil, &os.PathError{Op: "open", Path: path, Err: windows.ERROR_INVALID_HANDLE}
	}
	return file, nil
}
