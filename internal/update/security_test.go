package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeVersion(t *testing.T) {
	ok, err := SanitizeVersion("v1.2.3")
	if err != nil || ok != "1.2.3" {
		t.Fatalf("got %q err %v", ok, err)
	}
	if _, err := SanitizeVersion("../evil"); err == nil {
		t.Fatal("expected path traversal rejection")
	}
	if _, err := SanitizeVersion("v1..0"); err == nil {
		t.Fatal("expected double-dot rejection")
	}
}

func TestValidateDownloadURL(t *testing.T) {
	if err := ValidateDownloadURL("https://github.com/foursecondfivefour/conduit/releases/download/v1.2.0/conduit.exe"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDownloadURL("http://github.com/foo"); err == nil {
		t.Fatal("expected http rejection")
	}
	if err := ValidateDownloadURL("https://evil.example.com/conduit.exe"); err == nil {
		t.Fatal("expected host rejection")
	}
}

func TestValidateReleaseURL(t *testing.T) {
	if err := ValidateReleaseURL("https://github.com/foursecondfivefour/conduit/releases/tag/v1.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReleaseURL("https://github.com/other/repo"); err == nil {
		t.Fatal("expected repo rejection")
	}
}

func TestUpdateDirPathEscape(t *testing.T) {
	if _, err := UpdateDir(".."); err == nil {
		t.Fatal("expected invalid version")
	}
	dir, err := UpdateDir("v1.2.1")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(os.TempDir(), "Conduit", "update"))
	if !filepath.HasPrefix(dir, root) {
		t.Fatalf("dir %q outside %q", dir, root)
	}
}

func TestValidateSourceAndTargetPaths(t *testing.T) {
	root := filepath.Join(os.TempDir(), "Conduit", "update", "1.2.1")
	good := filepath.Join(root, "conduit.exe")
	if err := ValidateSourcePath(good); err != nil {
		t.Fatalf("expected ok source: %v", err)
	}
	if err := ValidateSourcePath(`C:\Windows\System32\cmd.exe`); err == nil {
		t.Fatal("expected outside temp rejection")
	}
	if err := ValidateTargetPath(`C:\Apps\conduit.exe`); err != nil {
		t.Fatalf("target: %v", err)
	}
	if err := ValidateTargetPath(`C:\Apps\evil.exe`); err == nil {
		t.Fatal("expected basename check")
	}
}
