package app

import "os"

func writePreferencesFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
