//go:build !windows

package app

func setAutostart(enabled bool) error {
	return nil
}

func isAutostartEnabled() bool {
	return false
}
