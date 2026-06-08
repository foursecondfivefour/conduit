//go:build windows

package filelog

import (
	"os/exec"
	"syscall"
)

func openWithShell(path string) error {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
