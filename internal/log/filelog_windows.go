//go:build windows

package filelog

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

const swShow = 5

func openWithShell(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	verbPtr, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		0,
		swShow,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW open failed (code %d)", ret)
	}
	return nil
}
