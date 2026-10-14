package control

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// keyDescriptor constructs a protected DACL for the current owner and required SYSTEM principal.
func keyDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")(A;;FA;;;SY)")
}

// createPrivate applies the protected ACL at creation and obtains a handle for verification.
func createPrivate(path string) (*os.File, error) {
	sd, err := keyDescriptor()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE|windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, &sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

// restrictKey protects an empty generated file with only owner and SYSTEM full access.
func restrictKey(f *os.File) error {
	sd, err := keyDescriptor()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if err = windows.SetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("restrict generated private-key ACL: %w", err)
	}
	actual, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	return verifyKeyACL(actual, sd)
}

// verifyKeyACL compares protected access entries; Windows' auto-inherited bookkeeping is harmless.
func verifyKeyACL(actual, expected *windows.SECURITY_DESCRIPTOR) error {
	control, _, err := actual.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("generated key DACL is not protected")
	}
	a, _, err := actual.DACL()
	if err != nil {
		return err
	}
	e, _, err := expected.DACL()
	if err != nil {
		return err
	}
	if a == nil || e == nil || a.AceCount != e.AceCount {
		return errors.New("generated key ACL entry count mismatch")
	}
	for i := uint32(0); i < uint32(e.AceCount); i++ {
		var got, want *windows.ACCESS_ALLOWED_ACE
		if err = windows.GetAce(a, i, &got); err != nil {
			return err
		}
		if err = windows.GetAce(e, i, &want); err != nil {
			return err
		}
		gotSID := (*windows.SID)(unsafe.Pointer(&got.SidStart))
		wantSID := (*windows.SID)(unsafe.Pointer(&want.SidStart))
		if got.Header.AceType != want.Header.AceType || got.Header.AceFlags != want.Header.AceFlags || got.Mask != want.Mask || gotSID.String() != wantSID.String() {
			return errors.New("generated key ACL access mismatch")
		}
	}
	return nil
}

// restrictDirectory establishes an inheritable protected owner/SYSTEM ACL on an owned store directory.
func restrictDirectory(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	if err = windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		return err
	}
	actual, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	return verifyKeyACL(actual, sd)
}

// fileRenameInfo matches the variable-length FILE_RENAME_INFO header on either pointer width.
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

// replaceFile atomically publishes an already synced same-directory restricted staging file.
// POSIX replacement preserves open readers' old snapshots; unsupported filesystems fail explicitly.
// It never unlinks the target first, bypasses ACL/read-only checks, or copies partially visible bytes.
func replaceFile(source, target string) error {
	fromPath, err := windowsFilePath(source)
	if err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(fromPath)
	if err != nil {
		return err
	}
	toPath, err := windowsFilePath(target)
	if err != nil {
		return err
	}
	to, err := windows.UTF16FromString(toPath)
	if err != nil {
		return err
	}
	if len(to) > windows.MAX_LONG_PATH {
		return windows.ERROR_FILENAME_EXCED_RANGE
	}
	handle, err := windows.CreateFile(from, windows.DELETE|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var header fileRenameInfo
	buffer := make([]byte, int(unsafe.Offsetof(header.FileName))+len(to)*2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.FileNameLength = uint32((len(to) - 1) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(to)), to)
	return windows.SetFileInformationByHandle(handle, windows.FileRenameInfoEx, &buffer[0], uint32(len(buffer)))
}
