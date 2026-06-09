//go:build !windows

package app

import "github.com/foursecondfivefour/conduit/internal/i18n"

func showUpdateReadyPrompt(_ i18n.Lang, _ string) bool {
	return false
}
