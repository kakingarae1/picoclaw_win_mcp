//go:build windows

package secrets

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modAdvapi32    = windows.NewLazySystemDLL("advapi32.dll")
	procCredRead   = modAdvapi32.NewProc("CredReadW")
	procCredWrite  = modAdvapi32.NewProc("CredWriteW")
	procCredDelete = modAdvapi32.NewProc("CredDeleteW")
	procCredFree   = modAdvapi32.NewProc("CredFree")
)

type credential struct {
	Flags, Type        uint32
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

func Set(name, key string) error {
	tgt, _ := syscall.UTF16PtrFromString("picoclaw/" + name)
	b := []byte(key)
	c := credential{Type: 1, TargetName: tgt, CredentialBlobSize: uint32(len(b)), CredentialBlob: &b[0], Persist: 2}
	r, _, e := procCredWrite.Call(uintptr(unsafe.Pointer(&c)), 0)
	if r == 0 {
		return fmt.Errorf("CredWrite: %w", e)
	}
	return nil
}

func Get(name string) (string, error) {
	tgt, _ := syscall.UTF16PtrFromString("picoclaw/" + name)
	var p *credential
	r, _, e := procCredRead.Call(uintptr(unsafe.Pointer(tgt)), 1, 0, uintptr(unsafe.Pointer(&p)))
	if r == 0 {
		return "", fmt.Errorf("CredRead (%q): %w", name, e)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(p)))
	return string(unsafe.Slice(p.CredentialBlob, p.CredentialBlobSize)), nil
}

func Delete(name string) error {
	tgt, _ := syscall.UTF16PtrFromString("picoclaw/" + name)
	r, _, e := procCredDelete.Call(uintptr(unsafe.Pointer(tgt)), 1, 0)
	if r == 0 {
		return fmt.Errorf("CredDelete: %w", e)
	}
	return nil
}
