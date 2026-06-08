package filelog

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const maxLogSize = 5 * 1024 * 1024

// Setup configures slog and the standard library log to write to a rotating file.
func Setup(logDir string) (*slog.Logger, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(logDir, "conduit.log")
	w, err := newRotatingWriter(path)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return logger, nil
}

type rotatingWriter struct {
	path string
	mu   sync.Mutex
	file *os.File
}

func newRotatingWriter(path string) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w.file = f
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if info, err := w.file.Stat(); err == nil && info.Size()+int64(len(p)) > maxLogSize {
		_ = w.file.Close()
		_ = os.Rename(w.path, w.path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

// LogPath returns the primary log file path.
func LogPath(logDir string) string {
	return filepath.Join(logDir, "conduit.log")
}

// MultiWriter returns a writer that duplicates to file and optional stderr.
func MultiWriter(file io.Writer, stderr bool) io.Writer {
	if !stderr {
		return file
	}
	return io.MultiWriter(file, os.Stderr)
}

// OpenLog opens the log file in the default application.
func OpenLog(logPath string) error {
	return openWithShell(logPath)
}
