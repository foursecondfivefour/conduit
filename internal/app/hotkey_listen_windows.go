//go:build windows

package app

import (
	"log/slog"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandle   = kernel32.NewProc("GetModuleHandleW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procGetMessage        = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
)

const (
	wmDestroy   = 0x0002
	hwndMessage = ^uintptr(0) - 3 // (HWND)-3 message-only window
)

type wndclassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type msg struct {
	HWND    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// setupHotkeyToggle registers Ctrl+Shift+Y on a background message loop.
func setupHotkeyToggle(win *application.WebviewWindow, stopCh <-chan struct{}) {
	if win == nil {
		return
	}
	hk := NewHotkeyToggle(func() {
		if win.IsVisible() {
			win.Hide()
		} else {
			win.Show()
		}
	})
	go runHotkeyMessageLoop(hk, stopCh)
}

func runHotkeyMessageLoop(hk *HotkeyToggle, stopCh <-chan struct{}) {
	className, _ := syscall.UTF16PtrFromString("ConduitHotkeyClass")
	hInstance, _, _ := procGetModuleHandle.Call(0)

	wc := wndclassEx{
		Size:      uint32(unsafe.Sizeof(wndclassEx{})),
		Instance:  syscall.Handle(hInstance),
		ClassName: className,
		WndProc:   syscall.NewCallback(defWindowProc),
	}
	if ret, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		slog.Warn("hotkey window class registration failed")
		return
	}

	hwnd, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		hwndMessage,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		slog.Warn("hotkey message window creation failed")
		return
	}
	defer func() { _, _, _ = procDestroyWindow.Call(hwnd) }()

	if err := hk.Register(hwnd); err != nil {
		slog.Warn("hotkey register failed", "err", err)
		return
	}
	defer hk.Unregister()

	if stopCh != nil {
		go func() {
			<-stopCh
			_, _, _ = procPostQuitMessage.Call(0)
		}()
	}

	var m msg
	for {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&m)),
			0,
			0,
			0,
		)
		if ret == 0 || int32(ret) == -1 {
			return
		}
		if hk.HandleMessage(m.Message) {
			continue
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func defWindowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == wmDestroy {
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}
