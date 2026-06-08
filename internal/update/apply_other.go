//go:build !windows

package update

import "errors"

func Apply(targetExe, sourceExe string, parentPID int) error {
	return errors.New("update apply: windows only")
}
