// Package usage tracks LLM token consumption for the pay-as-you-go billing tier.
//
// Every successful chat completion produces a Record (timestamp, model, tokens
// in/out, optional feature label). The Store appends records to a monthly
// JSONL file so cold-start cost is bounded — startup loads the current month
// only and reconstructs in-memory month/day totals for fast UI reads. Older
// months are read on-demand when queried.
//
// All exported methods are safe to call concurrently.
package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Record is one chat-completion's accounting entry.
type Record struct {
	Timestamp        time.Time `json:"ts"`
	Model            string    `json:"model"`
	Feature          string    `json:"feature,omitempty"` // optional: "diagnostic", "pr-summary", "code-review", …
	PromptTokens     int       `json:"in"`
	CompletionTokens int       `json:"out"`
}

// Rates expresses the cost-per-million-tokens for input and output tokens.
// Both fields are USD per 1,000,000 tokens. Zero means "don't compute cost"
// — the UI will show ‐‐ for the est. cost columns instead of $0.00.
type Rates struct {
	InputPerMTokens  float64
	OutputPerMTokens float64
}

// PeriodTotals aggregates a slice of records for a UI period.
type PeriodTotals struct {
	Calls            int     `json:"calls"`
	PromptTokens     int     `json:"in"`
	CompletionTokens int     `json:"out"`
	EstCostUSD       float64 `json:"estCostUsd"`
}

// ModelTotals aggregates per-model usage across all observed time.
type ModelTotals struct {
	Model            string  `json:"model"`
	Calls            int     `json:"calls"`
	PromptTokens     int     `json:"in"`
	CompletionTokens int     `json:"out"`
	EstCostUSD       float64 `json:"estCostUsd"`
}

// Summary is the structure consumed by the Wails GetUsageSummary binding.
type Summary struct {
	Today    PeriodTotals  `json:"today"`
	Month    PeriodTotals  `json:"month"`
	Lifetime PeriodTotals  `json:"lifetime"`
	ByModel  []ModelTotals `json:"byModel"`
	Rates    Rates         `json:"rates"`
	// FirstRecordedAt is when the very first record was observed; useful for
	// the UI to display "tracking since …".
	FirstRecordedAt *time.Time `json:"firstRecordedAt,omitempty"`
}

// Store is the on-disk JSONL accumulator. The current month is kept fully
// in memory; older months stay on disk and are read lazily by Lifetime().
type Store struct {
	dir string

	mu               sync.RWMutex
	currentMonthKey  string   // "2026-05"
	currentMonthRecs []Record // append-only in-memory buffer for this month
	lifetimeCache    PeriodTotals
	lifetimeByModel  map[string]ModelTotals
	lifetimeLoaded   bool
	firstRecordedAt  *time.Time
}

// New creates a Store rooted at $HOME/.kubewatcher/usage. The directory is
// created if it doesn't exist. Returns an error only on filesystem failures.
func New() (*Store, error) {
	dir := filepath.Join(os.ExpandEnv("$HOME"), ".kubewatcher", "usage")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create usage dir: %w", err)
	}
	s := &Store{
		dir:             dir,
		lifetimeByModel: map[string]ModelTotals{},
	}
	now := time.Now().UTC()
	s.currentMonthKey = monthKey(now)
	if err := s.loadMonth(s.currentMonthKey); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// NewAt is like New but uses an explicit base directory. Useful for tests.
func NewAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create usage dir: %w", err)
	}
	s := &Store{
		dir:             dir,
		lifetimeByModel: map[string]ModelTotals{},
	}
	now := time.Now().UTC()
	s.currentMonthKey = monthKey(now)
	if err := s.loadMonth(s.currentMonthKey); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// Record appends one entry. Records with no tokens at all are dropped — the
// LLM either failed mid-request or returned an empty Usage block, neither of
// which is useful for billing.
func (s *Store) Record(rec Record) error {
	if rec.PromptTokens == 0 && rec.CompletionTokens == 0 {
		return nil
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	} else {
		rec.Timestamp = rec.Timestamp.UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If the calendar month rolled over since startup, flush our in-memory
	// buffer and switch to the new file.
	mk := monthKey(rec.Timestamp)
	if mk != s.currentMonthKey {
		s.currentMonthKey = mk
		s.currentMonthRecs = nil
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	path := filepath.Join(s.dir, mk+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("open usage file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("write usage record: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close usage file: %w", err)
	}

	s.currentMonthRecs = append(s.currentMonthRecs, rec)
	if s.firstRecordedAt == nil || rec.Timestamp.Before(*s.firstRecordedAt) {
		t := rec.Timestamp
		s.firstRecordedAt = &t
	}
	if s.lifetimeLoaded {
		s.lifetimeCache.Calls++
		s.lifetimeCache.PromptTokens += rec.PromptTokens
		s.lifetimeCache.CompletionTokens += rec.CompletionTokens
		mt := s.lifetimeByModel[rec.Model]
		mt.Model = rec.Model
		mt.Calls++
		mt.PromptTokens += rec.PromptTokens
		mt.CompletionTokens += rec.CompletionTokens
		s.lifetimeByModel[rec.Model] = mt
	}
	return nil
}

// Summary returns the today/month/lifetime aggregates plus per-model totals.
// The first call lazily ingests every historical month; subsequent calls are
// O(records-this-month).
func (s *Store) Summary(rates Rates) (Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lifetimeLoaded {
		if err := s.loadAllMonthsLocked(); err != nil {
			return Summary{}, err
		}
	}

	now := time.Now().UTC()
	today := PeriodTotals{}
	month := PeriodTotals{}
	for _, r := range s.currentMonthRecs {
		month.Calls++
		month.PromptTokens += r.PromptTokens
		month.CompletionTokens += r.CompletionTokens
		if sameDay(r.Timestamp, now) {
			today.Calls++
			today.PromptTokens += r.PromptTokens
			today.CompletionTokens += r.CompletionTokens
		}
	}
	applyCost(&today, rates)
	applyCost(&month, rates)

	lifetime := s.lifetimeCache
	applyCost(&lifetime, rates)

	byModel := make([]ModelTotals, 0, len(s.lifetimeByModel))
	for _, mt := range s.lifetimeByModel {
		mt.EstCostUSD = costOf(mt.PromptTokens, mt.CompletionTokens, rates)
		byModel = append(byModel, mt)
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].PromptTokens+byModel[i].CompletionTokens >
			byModel[j].PromptTokens+byModel[j].CompletionTokens
	})

	return Summary{
		Today:           today,
		Month:           month,
		Lifetime:        lifetime,
		ByModel:         byModel,
		Rates:           rates,
		FirstRecordedAt: s.firstRecordedAt,
	}, nil
}

// Clear removes every recorded month. The user's "reset usage" button.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read usage dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil {
			return fmt.Errorf("remove %s: %w", e.Name(), err)
		}
	}
	s.currentMonthRecs = nil
	s.lifetimeCache = PeriodTotals{}
	s.lifetimeByModel = map[string]ModelTotals{}
	s.lifetimeLoaded = true
	s.firstRecordedAt = nil
	return nil
}

// --- internals ---

func (s *Store) loadMonth(monthKey string) error {
	path := filepath.Join(s.dir, monthKey+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			// Skip malformed lines — better than aborting the whole load.
			continue
		}
		s.currentMonthRecs = append(s.currentMonthRecs, r)
		if s.firstRecordedAt == nil || r.Timestamp.Before(*s.firstRecordedAt) {
			t := r.Timestamp
			s.firstRecordedAt = &t
		}
	}
	return scanner.Err()
}

// loadAllMonthsLocked walks every .jsonl file in the usage dir and rebuilds
// the lifetime totals + per-model breakdown. Caller must hold s.mu.
func (s *Store) loadAllMonthsLocked() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read usage dir: %w", err)
	}
	s.lifetimeCache = PeriodTotals{}
	s.lifetimeByModel = map[string]ModelTotals{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		mk := strings.TrimSuffix(e.Name(), ".jsonl")
		recs, err := s.readMonthFile(mk)
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		for _, r := range recs {
			s.lifetimeCache.Calls++
			s.lifetimeCache.PromptTokens += r.PromptTokens
			s.lifetimeCache.CompletionTokens += r.CompletionTokens
			mt := s.lifetimeByModel[r.Model]
			mt.Model = r.Model
			mt.Calls++
			mt.PromptTokens += r.PromptTokens
			mt.CompletionTokens += r.CompletionTokens
			s.lifetimeByModel[r.Model] = mt
			if s.firstRecordedAt == nil || r.Timestamp.Before(*s.firstRecordedAt) {
				t := r.Timestamp
				s.firstRecordedAt = &t
			}
		}
	}
	s.lifetimeLoaded = true
	return nil
}

// readMonthFile is loadMonth without mutating store state.
func (s *Store) readMonthFile(monthKey string) ([]Record, error) {
	path := filepath.Join(s.dir, monthKey+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, scanner.Err()
}

func monthKey(t time.Time) string { return t.UTC().Format("2006-01") }

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

func costOf(in, out int, r Rates) float64 {
	if r.InputPerMTokens <= 0 && r.OutputPerMTokens <= 0 {
		return 0
	}
	return (float64(in)/1_000_000)*r.InputPerMTokens + (float64(out)/1_000_000)*r.OutputPerMTokens
}

func applyCost(p *PeriodTotals, r Rates) {
	p.EstCostUSD = costOf(p.PromptTokens, p.CompletionTokens, r)
}
