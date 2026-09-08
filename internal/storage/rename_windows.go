//go:build windows

package storage

import "golang.org/x/sys/windows"

func renameNoReplace(oldPath, newPath string) error {
	oldUTF16, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newUTF16, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	// Omitting MOVEFILE_REPLACE_EXISTING preserves immutable version names.
	return windows.MoveFileEx(oldUTF16, newUTF16, windows.MOVEFILE_WRITE_THROUGH)
}
