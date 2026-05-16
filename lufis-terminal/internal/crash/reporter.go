package crash

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Report struct {
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
	Error   string    `json:"error"`
	Stack   string    `json:"stack"`
	OS      string    `json:"os"`
	Arch    string    `json:"arch"`
	GoVer   string    `json:"go_version"`
}

type Reporter struct {
	dir     string
	version string
	logger  *slog.Logger
}

func New(dir, version string, logger *slog.Logger) *Reporter {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "argus-terminal", "crashes")
	}
	_ = os.MkdirAll(dir, 0o700)
	return &Reporter{dir: dir, version: version, logger: logger}
}

func (r *Reporter) Capture(err error) {
	if err == nil {
		return
	}

	stack := make([]byte, 65536)
	n := runtime.Stack(stack, false)

	report := Report{
		Time:    time.Now(),
		Version: r.version,
		Error:   err.Error(),
		Stack:   string(stack[:n]),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		GoVer:   runtime.Version(),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		r.logger.Error("failed to marshal crash report", "error", err)
		return
	}

	path := filepath.Join(r.dir, fmt.Sprintf("crash-%d.json", report.Time.UnixNano()))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		r.logger.Error("failed to write crash report", "error", err)
		return
	}

	r.logger.Error("crash report saved", "path", path, "error", report.Error)
}

func (r *Reporter) History() ([]Report, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var reports []Report
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var report Report
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (r *Reporter) Clear() error {
	entries, _ := os.ReadDir(r.dir)
	for _, e := range entries {
		os.Remove(filepath.Join(r.dir, e.Name()))
	}
	return nil
}
