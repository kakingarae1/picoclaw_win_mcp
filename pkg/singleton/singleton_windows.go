//go:build windows

package singleton

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

var h windows.Handle

func Acquire(name string) error {
	n, _ := syscall.UTF16PtrFromString("Global\\" + name)
	handle, err := windows.CreateMutex(nil, false, n)
	if err != nil { return fmt.Errorf("mutex: %w", err) }
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(handle)
		return fmt.Errorf("%s is already running", name)
	}
	h = handle
	return nil
}
