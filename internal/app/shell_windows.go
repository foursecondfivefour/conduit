//go:build windows

package app

import (
	"fmt"
	"log/slog"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

const (
	swShowNormal = 1
	swShow       = 5
)

func openFolder(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	verbPtr, err := windows.UTF16PtrFromString("explore")
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
		return fmt.Errorf("ShellExecuteW explore failed (code %d)", ret)
	}
	return nil
}

func openURL(url string) error {
	urlPtr, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	opPtr, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(opPtr)),
		uintptr(unsafe.Pointer(urlPtr)),
		0,
		0,
		swShowNormal,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecute failed (code %d)", ret)
	}
	return nil
}

func copyToClipboard(text string) {
	cmd := exec.Command("cmd", "/c", "echo "+text+"| clip")
	if err := cmd.Run(); err != nil {
		slog.Warn("copy to clipboard failed", "err", err)
	}
}
