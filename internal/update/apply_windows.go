//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Apply launches conduit-updater to replace the running binary and relaunch.
func Apply(targetExe, sourceExe string, parentPID int) error {
	updater, err := findUpdater(targetExe)
	if err != nil {
		return err
	}
	cmd := exec.Command(updater,
		"--pid", strconv.Itoa(parentPID),
		"--target", targetExe,
		"--source", sourceExe,
		"--relaunch",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func findUpdater(targetExe string) (string, error) {
	dir := filepath.Dir(targetExe)
	candidate := filepath.Join(dir, "conduit-updater.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidate = filepath.Join(filepath.Dir(exe), "conduit-updater.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("update: conduit-updater.exe not found")
}
