// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Keychain on Windows stores secrets as generic credentials in the user's
// Windows Credential Manager, which encrypts them with DPAPI under the
// user's logon. The name stays Keychain so callers need no build tags.
// Credentials are never roamed (CRED_PERSIST_LOCAL_MACHINE).
type Keychain struct{}

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW  = advapi32.NewProc("CredReadW")
	procCredWriteW = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	// CRED_MAX_CREDENTIAL_BLOB_SIZE: every AnyProfile secret is far smaller.
	maxBlob = 5 * 512
)

// credential is CREDENTIALW.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// target is "<service>/<account>", e.g. dev.anyprofile.proxy/px1.
func target(service, account string) (*uint16, error) {
	return windows.UTF16PtrFromString(service + "/" + account)
}

func (Keychain) Get(service, account string) ([]byte, error) {
	name, err := target(service, account)
	if err != nil {
		return nil, err
	}
	var c *credential
	r, _, e := procCredReadW.Call(uintptr(unsafe.Pointer(name)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c)))
	if r == 0 {
		if errors.Is(e, windows.ERROR_NOT_FOUND) {
			return nil, ErrNotFound
		}
		return nil, e
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(c)))
	return append([]byte(nil), unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)...), nil
}

func (Keychain) Set(service, account string, data []byte) error {
	if len(data) > maxBlob {
		return errors.New("secret is too large for Windows Credential Manager")
	}
	name, err := target(service, account)
	if err != nil {
		return err
	}
	user, _ := windows.UTF16PtrFromString(account)
	c := credential{Type: credTypeGeneric, TargetName: name, UserName: user,
		CredentialBlobSize: uint32(len(data)), Persist: credPersistLocalMachine}
	if len(data) > 0 {
		c.CredentialBlob = &data[0]
	}
	if r, _, e := procCredWriteW.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return e
	}
	return nil
}

func (Keychain) Delete(service, account string) error {
	name, err := target(service, account)
	if err != nil {
		return err
	}
	if r, _, e := procCredDelete.Call(uintptr(unsafe.Pointer(name)), credTypeGeneric, 0); r == 0 && !errors.Is(e, windows.ERROR_NOT_FOUND) {
		return e
	}
	return nil
}
