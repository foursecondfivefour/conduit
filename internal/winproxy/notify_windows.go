//go:build windows

package winproxy

import (
	"syscall"
	"unsafe"
)

var (
	wininet                    = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW     = wininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

func notifyProxyChanged() error {
	ret, _, err := procInternetSetOptionW.Call(0, internetOptionSettingsChanged, 0, 0)
	if ret == 0 {
		return err
	}
	ret, _, err = procInternetSetOptionW.Call(0, internetOptionRefresh, 0, 0)
	if ret == 0 {
		return err
	}
	_ = unsafe.Pointer(nil)
	return nil
}
