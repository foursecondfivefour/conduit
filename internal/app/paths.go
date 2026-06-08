package app

import (
	"os"
	"path/filepath"
)

// RuntimePaths resolves config and log directories for installed vs portable mode.
type RuntimePaths struct {
	ConfigDir string
	LogDir    string
	Portable  bool
}

// ResolveRuntimePaths returns config/log directories for installed or portable mode.
func ResolveRuntimePaths(portable bool) (RuntimePaths, error) {
	if portable {
		exe, err := os.Executable()
		if err != nil {
			return RuntimePaths{}, err
		}
		base := filepath.Join(filepath.Dir(exe), "Conduit")
		return RuntimePaths{
			ConfigDir: base,
			LogDir:    filepath.Join(base, "logs"),
			Portable:  true,
		}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return RuntimePaths{}, err
	}
	base := filepath.Join(dir, "Conduit")
	return RuntimePaths{
		ConfigDir: base,
		LogDir:    filepath.Join(base, "logs"),
		Portable:  false,
	}, nil
}

func (p RuntimePaths) PreferencesPath() string {
	return filepath.Join(p.ConfigDir, "preferences.json")
}

// LoadPreferences reads preferences from disk without starting the debounced store.
func LoadPreferences(paths RuntimePaths) (Preferences, error) {
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		return Preferences{}, err
	}
	store, err := newPreferenceStore(paths)
	if err != nil {
		return Preferences{}, err
	}
	defer store.Close()
	return store.Get(), nil
}
