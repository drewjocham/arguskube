package recorder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Recording struct {
	ID         string    `json:"id"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Duration   string    `json:"duration"`
	Status     string    `json:"status"`
	FilePath   string    `json:"file_path"`
	Transcript string    `json:"transcript,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	Tasks      []string  `json:"tasks,omitempty"`
}

type Recorder struct {
	mu        sync.Mutex
	recording *Recording
	logger    *slog.Logger
	dataDir   string
}

func New(dataDir string, logger *slog.Logger) *Recorder {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".config", "argus-terminal", "recordings")
	}
	_ = os.MkdirAll(dataDir, 0o700)
	return &Recorder{logger: logger, dataDir: dataDir}
}

func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording != nil
}

func (r *Recorder) Start() (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording != nil {
		return nil, fmt.Errorf("already recording")
	}
	id := fmt.Sprintf("rec-%d", time.Now().UnixNano())
	path := filepath.Join(r.dataDir, id+".wav")
	r.recording = &Recording{
		ID: id, StartTime: time.Now(), Status: "recording", FilePath: path,
	}
	r.logger.Info("recording started", "id", id)
	return r.recording, nil
}

func (r *Recorder) Stop() (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording == nil {
		return nil, fmt.Errorf("not recording")
	}
	r.recording.EndTime = time.Now()
	r.recording.Duration = r.recording.EndTime.Sub(r.recording.StartTime).String()
	r.recording.Status = "completed"
	rec := r.recording
	r.recording = nil
	r.save(rec)
	r.logger.Info("recording stopped", "id", rec.ID, "duration", rec.Duration)
	return rec, nil
}

func (r *Recorder) Cancel() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording == nil {
		return fmt.Errorf("not recording")
	}
	os.Remove(r.recording.FilePath)
	r.recording = nil
	return nil
}

func (r *Recorder) History() []Recording {
	entries, _ := os.ReadDir(r.dataDir)
	var recordings []Recording
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(r.dataDir, e.Name()))
		var rec Recording
		if json.Unmarshal(data, &rec) == nil {
			recordings = append(recordings, rec)
		}
	}
	return recordings
}

func (r *Recorder) save(rec *Recording) {
	data, _ := json.Marshal(rec)
	path := filepath.Join(r.dataDir, rec.ID+".json")
	_ = os.WriteFile(path, data, 0o600)
}
