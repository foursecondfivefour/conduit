package update

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.-]*$`)

var allowedDownloadHosts = map[string]struct{}{
	"github.com":                 {},
	"objects.githubusercontent.com": {},
}

// SanitizeVersion normalizes a release tag for use as a directory name.
func SanitizeVersion(tag string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if v == "" {
		return "", fmt.Errorf("update: empty version")
	}
	if strings.Contains(v, "..") || strings.ContainsAny(v, `/\`) {
		return "", fmt.Errorf("update: invalid version path")
	}
	if !versionPattern.MatchString(v) {
		return "", fmt.Errorf("update: invalid version format")
	}
	return v, nil
}

// ValidateDownloadURL ensures release assets are fetched only from GitHub hosts.
func ValidateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("update: invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("update: download must use https")
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := allowedDownloadHosts[host]; !ok {
		return fmt.Errorf("update: download host not allowed: %s", host)
	}
	return nil
}

// ValidateReleaseURL restricts browser release links to the Conduit GitHub repo.
func ValidateReleaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("update: release url must use https")
	}
	host := strings.ToLower(u.Hostname())
	if host != "github.com" {
		return fmt.Errorf("update: release host not allowed")
	}
	path := strings.ToLower(u.Path)
	if !strings.HasPrefix(path, "/foursecondfivefour/conduit") {
		return fmt.Errorf("update: release path not allowed")
	}
	return nil
}

// UpdateDir returns a safe directory under temp for a sanitized version.
func UpdateDir(version string) (string, error) {
	safe, err := SanitizeVersion(version)
	if err != nil {
		return "", err
	}
	base := filepath.Join(filepath.Clean(filepath.Join(os.TempDir(), "Conduit", "update")), safe)
	cleanBase := filepath.Clean(filepath.Join(os.TempDir(), "Conduit", "update"))
	if !strings.HasPrefix(base, cleanBase+string(filepath.Separator)) && base != cleanBase {
		return "", fmt.Errorf("update: path escape")
	}
	return base, nil
}

// ValidateSourcePath ensures the downloaded binary lives under the update temp root.
func ValidateSourcePath(source string) error {
	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	root := filepath.Clean(filepath.Join(os.TempDir(), "Conduit", "update"))
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return fmt.Errorf("update: source outside update directory")
	}
	if filepath.Base(abs) != "conduit.exe" {
		return fmt.Errorf("update: unexpected source filename")
	}
	return nil
}

// ValidateTargetPath ensures the install target is named conduit.exe.
func ValidateTargetPath(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if filepath.Base(abs) != "conduit.exe" {
		return fmt.Errorf("update: target must be conduit.exe")
	}
	return nil
}
