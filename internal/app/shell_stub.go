//go:build !windows

package app

import "fmt"

func openFolder(path string) error {
	return fmt.Errorf("open folder not supported: %s", path)
}

func openURL(url string) error {
	return fmt.Errorf("open url not supported: %s", url)
}
