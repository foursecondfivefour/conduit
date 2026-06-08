//go:build !windows

package filelog

import "fmt"

func openWithShell(path string) error {
	return fmt.Errorf("open log not supported on this platform: %s", path)
}
