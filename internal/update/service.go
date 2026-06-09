package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/foursecondfivefour/conduit/internal/config"
)

type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateError       State = "error"
)

type Service struct {
	client   *Client
	// downloadURLValidator is injectable for tests (defaults to ValidateDownloadURL).
	downloadURLValidator func(string) error
	mu                   sync.RWMutex
	state    State
	latest   Release
	percent  int
	err      error
	path     string
	shaPath  string
	onChange func()
}

func NewService() *Service {
	return &Service{
		client:               NewClient(),
		downloadURLValidator: ValidateDownloadURL,
		state:                StateIdle,
	}
}

// OnChange registers a callback invoked when update state changes.
func (s *Service) OnChange(fn func()) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Service) notify() {
	s.mu.RLock()
	fn := s.onChange
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// Apply launches the updater helper to replace the running binary.
func (s *Service) Apply(targetExe, sourceExe string, parentPID int) error {
	s.mu.RLock()
	sha := s.shaPath
	s.mu.RUnlock()
	return Apply(targetExe, sourceExe, parentPID, sha)
}

func (s *Service) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Service) Latest() Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

func (s *Service) Progress() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.percent
}

func (s *Service) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *Service) DownloadPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

func (s *Service) Check(ctx context.Context, skippedVersion string) (bool, error) {
	s.set(StateChecking, 0, nil)
	rel, err := s.client.Latest(ctx)
	if err != nil {
		s.set(StateError, 0, err)
		return false, err
	}
	ver := rel.Version()
	if skippedVersion != "" && Compare(ver, strings.TrimPrefix(strings.TrimSpace(skippedVersion), "v")) == 0 {
		s.mu.Lock()
		s.latest = rel
		s.state = StateIdle
		s.mu.Unlock()
		s.notify()
		return false, nil
	}
	if !IsNewer(config.Version, ver) {
		s.mu.Lock()
		s.latest = rel
		s.state = StateIdle
		s.mu.Unlock()
		s.notify()
		return false, nil
	}
	s.mu.Lock()
	s.latest = rel
	s.state = StateAvailable
	s.mu.Unlock()
	s.notify()
	return true, nil
}

func (s *Service) Download(ctx context.Context) error {
	s.mu.RLock()
	rel := s.latest
	s.mu.RUnlock()

	url, ok := rel.AssetURL("conduit.exe")
	if !ok {
		err := fmt.Errorf("update: conduit.exe asset not found")
		s.set(StateError, 0, err)
		return err
	}
	validate := s.downloadURLValidator
	if validate == nil {
		validate = ValidateDownloadURL
	}
	if err := validate(url); err != nil {
		s.set(StateError, 0, err)
		return err
	}

	dir, err := UpdateDir(rel.TagName)
	if err != nil {
		s.set(StateError, 0, err)
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		s.set(StateError, 0, err)
		return err
	}
	dest := filepath.Join(dir, "conduit.exe")
	shaDest := filepath.Join(dir, "conduit.exe.sha256")

	s.set(StateDownloading, 0, nil)

	if err := downloadFile(ctx, s.client.HTTP, url, dest, func(p int) {
		s.set(StateDownloading, p, nil)
	}); err != nil {
		s.set(StateError, 0, err)
		return err
	}

	if err := verifyPE(dest); err != nil {
		s.set(StateError, 0, err)
		return err
	}

	shaURL, ok := rel.AssetURL("conduit.exe.sha256")
	if !ok {
		err := fmt.Errorf("update: sha256 asset not found")
		s.set(StateError, 0, err)
		return err
	}
	if err := validate(shaURL); err != nil {
		s.set(StateError, 0, err)
		return err
	}
	if err := downloadFile(ctx, s.client.HTTP, shaURL, shaDest, nil); err != nil {
		s.set(StateError, 0, err)
		return err
	}
	if err := VerifySHA256File(dest, shaDest); err != nil {
		s.set(StateError, 0, err)
		return err
	}

	s.mu.Lock()
	s.path = dest
	s.shaPath = shaDest
	s.state = StateReady
	s.percent = 100
	s.err = nil
	s.mu.Unlock()
	s.notify()
	return nil
}

func (s *Service) set(state State, percent int, err error) {
	s.mu.Lock()
	prevState := s.state
	prevPercent := s.percent
	s.state = state
	if percent >= 0 {
		s.percent = percent
	}
	s.err = err
	notify := prevState != state
	if !notify && state == StateDownloading && percent >= 0 {
		notify = percent/10 != prevPercent/10
	}
	s.mu.Unlock()
	if notify {
		s.notify()
	}
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string, progress func(int)) error {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Conduit/"+config.Version)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: status %d", resp.StatusCode)
	}

	tmp := dest + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				_ = os.Remove(tmp)
				return werr
			}
			written += int64(n)
			if progress != nil && total > 0 {
				progress(int(written * 100 / total))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func verifyPE(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return err
	}
	if string(hdr) != "MZ" {
		return fmt.Errorf("update: invalid PE header")
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() < 1<<20 {
		return fmt.Errorf("update: file too small")
	}
	return nil
}
