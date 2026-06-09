//go:build windows

package app

import (
	"syscall"
	"unsafe"

	"github.com/foursecondfivefour/conduit/internal/i18n"
)

const (
	mbYesNo           = 0x00000004
	mbIconInformation = 0x00000040
	idYes             = 6
)

func showUpdateReadyPrompt(lang i18n.Lang, version string) bool {
	text := i18n.Tf(lang, "tray.updates.ready.prompt", version)
	title := i18n.T(lang, "tray.updates.ready.title")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ret, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbYesNo|mbIconInformation|mbTopmost),
	)
	return ret == idYes
}
