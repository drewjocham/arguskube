package logging

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	mu      sync.Mutex
	closers []io.Closer
)

func New(debug bool, logFile string) (*slog.Logger, error) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	var w io.Writer = os.Stderr
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		closers = append(closers, f)
		mu.Unlock()
		w = f
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})), nil
}

// Shutdown closes any open log files.
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	for _, c := range closers {
		_ = c.Close()
	}
	closers = nil
}
