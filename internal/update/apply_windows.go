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
func Apply(targetExe, sourceExe string, parentPID int, shaPath string) error {
	if err := ValidateTargetPath(targetExe); err != nil {
		return err
	}
	if err := ValidateSourcePath(sourceExe); err != nil {
		return err
	}
	if shaPath != "" {
		if err := VerifySHA256File(sourceExe, shaPath); err != nil {
			return err
		}
	}

	updater, err := findUpdater(targetExe)
	if err != nil {
		return err
	}
	if err := verifyUpdater(updater); err != nil {
		return err
	}

	args := []string{
		"--pid", strconv.Itoa(parentPID),
		"--target", targetExe,
		"--source", sourceExe,
	}
	if shaPath != "" {
		args = append(args, "--sha", shaPath)
	}
	args = append(args, "--relaunch")

	cmd := exec.Command(updater, args...)
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

func verifyUpdater(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("update: updater stat: %w", err)
	}
	if info.Size() < 64*1024 {
		return fmt.Errorf("update: updater file too small")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := make([]byte, 2)
	if _, err := f.Read(hdr); err != nil {
		return err
	}
	if string(hdr) != "MZ" {
		return fmt.Errorf("update: invalid updater PE header")
	}
	return nil
}
