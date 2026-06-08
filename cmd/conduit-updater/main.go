// conduit-updater replaces a running conduit.exe after the parent exits.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

func main() {
	pidFlag := flag.Int("pid", 0, "parent process id")
	target := flag.String("target", "", "path to conduit.exe to replace")
	source := flag.String("source", "", "path to new conduit.exe")
	relaunch := flag.Bool("relaunch", false, "start target after replace")
	flag.Parse()

	if *pidFlag <= 0 || *target == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "usage: conduit-updater --pid N --target path --source path [--relaunch]")
		os.Exit(2)
	}

	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(*pidFlag))
	if err == nil {
		event, _ := windows.WaitForSingleObject(handle, windows.INFINITE)
		_ = event
		_ = windows.CloseHandle(handle)
	} else {
		time.Sleep(2 * time.Second)
	}

	backup := *target + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(*target, backup); err != nil {
		fmt.Fprintf(os.Stderr, "backup: %v\n", err)
		os.Exit(1)
	}
	if err := copyFile(*source, *target); err != nil {
		_ = os.Rename(backup, *target)
		fmt.Fprintf(os.Stderr, "copy: %v\n", err)
		os.Exit(1)
	}
	go func() {
		time.Sleep(10 * time.Second)
		_ = os.Remove(backup)
	}()

	if *relaunch {
		cmd := exec.Command(*target)
		cmd.Dir = filepath.Dir(*target)
		_ = cmd.Start()
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
