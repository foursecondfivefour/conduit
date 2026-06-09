package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// FileSHA256 returns the hex digest of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256File requires a sidecar file and matching digest (fail-closed).
func VerifySHA256File(exePath, shaPath string) error {
	raw, err := os.ReadFile(shaPath)
	if err != nil {
		return fmt.Errorf("update: sha256 file missing: %w", err)
	}
	want := strings.TrimSpace(string(raw))
	if len(want) < 64 {
		return fmt.Errorf("update: invalid sha256 file")
	}
	want = want[:64]

	got, err := FileSHA256(exePath)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("update: sha256 mismatch")
	}
	return nil
}
