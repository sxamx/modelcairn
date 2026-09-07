//go:build windows

package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestKeyringRejectsBroadWindowsACL(t *testing.T) {
	dir := t.TempDir()
	installation, err := OpenInstallation(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, keyringName, keyFilename(1))
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	keys, err := openKeyring(dir)
	if err != nil {
		t.Fatal(err)
	}
	if key, err := keys.load(1); key != nil || !errors.Is(err, errKeyMaterialUnavailable) {
		clear(key)
		t.Fatalf("broad ACL load = (%v,%v)", key, err)
	}

	if err := applyPrivateACL(path); err != nil {
		t.Fatal(err)
	}
	descriptor, err = windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err = descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var callbackACE *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &callbackACE); err != nil {
		t.Fatal(err)
	}
	callbackACE.Header.AceType = 9 // ACCESS_ALLOWED_CALLBACK_ACE_TYPE
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if key, err := keys.load(1); key != nil || !errors.Is(err, errKeyMaterialUnavailable) {
		clear(key)
		t.Fatalf("callback ACE load = (%v,%v)", key, err)
	}
}
