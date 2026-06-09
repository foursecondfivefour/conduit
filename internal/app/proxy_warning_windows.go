//go:build windows

package app

import (
	"syscall"
	"unsafe"

	"github.com/foursecondfivefour/conduit/internal/i18n"
)

const (
	mbOK              = 0x00000000
	mbIconWarning     = 0x00000030
	mbTopmost         = 0x00040000
)

var procMessageBoxW = user32.NewProc("MessageBoxW")

func showSystemProxyWarning(lang i18n.Lang) {
	text := i18n.T(lang, "tray.system_proxy.warning")
	title := i18n.T(lang, "tray.system_proxy.warning.title")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	_, _, _ = procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbOK|mbIconWarning|mbTopmost),
	)
}
