package update

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256Required(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "conduit.exe")
	content := []byte("MZ" + string(make([]byte, 1<<20)))
	if err := os.WriteFile(exe, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sha := filepath.Join(dir, "conduit.exe.sha256")
	sum := sha256.Sum256(content)
	if err := os.WriteFile(sha, []byte(hex.EncodeToString(sum[:])), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256File(exe, sha); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256File(exe, filepath.Join(dir, "missing.sha256")); err == nil {
		t.Fatal("expected missing sha failure")
	}
}
