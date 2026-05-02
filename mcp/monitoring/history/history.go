package history

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type IssueKind string

const (
	IncidentTypeNode  IssueKind = "node_anomaly"
	IncidentTypePod   IssueKind = "pod_anomaly"
	IncidentTypeEvent IssueKind = "event_spike"
)

var SupportedKinds = []IssueKind{
	IncidentTypeNode,
	IncidentTypePod,
	IncidentTypeEvent,
}

type Incident struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Kind        IssueKind      `json:"kind"`
	Severity    string         `json:"severity"`
	Namespace   string         `json:"namespace,omitempty"`
	Name        string         `json:"name,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Message     string         `json:"message,omitempty"`
	Occurrences int            `json:"occurrences"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func (i Incident) GetID() string {
	return i.ID
}

func (i Incident) GetTimestamp() time.Time {
	return i.Timestamp
}

func (i Incident) GetKind() IssueKind {
	return i.Kind
}

func (i Incident) ToIncident() Incident {
	return i
}

type FrequencyComparison struct {
	Kind             IssueKind `json:"kind"`
	RecentCount      int       `json:"recent_count"`
	PreviousCount    int       `json:"previous_count"`
	PercentChange    float64   `json:"percent_change"`
	WindowHours      float64   `json:"window_hours"`
	PreviousWindowHr float64   `json:"previous_window_hours"`
}

type Recordable interface {
	GetID() string
	GetTimestamp() time.Time
	GetKind() IssueKind
	ToIncident() Incident
}

type Recorder interface {
	Record(ctx context.Context, entry Recordable) error
	List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error)
	CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error)
	Close() error
}

var _ Recorder = (*Store)(nil)

type Store struct {
	db     *badger.DB
	closed bool
	mu     sync.RWMutex
}

func NewStore(path string) (*Store, error) {
	dir, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("history: failed to create storage dir: %w", err)
	}

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("history: failed to open badger: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Record(ctx context.Context, entry Recordable) error {
	if entry == nil {
		return errors.New("history: entry is nil")
	}

	inc := entry.ToIncident()
	if inc.ID == "" {
		return errors.New("history: entry ID is required")
	}

	key := s.buildKey(inc.Kind, inc.Timestamp, inc.ID)
	val, err := json.Marshal(inc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *Store) List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error) {
	var incidents []Incident
	start := time.Now().Add(-since)

	err := s.forEachInRange(ctx, kind, start, time.Now(), true, func(item *badger.Item) error {
		return item.Value(func(v []byte) error {
			var inc Incident
			if err := json.Unmarshal(v, &inc); err != nil {
				return err
			}
			incidents = append(incidents, inc)
			return nil
		})
	})

	return incidents, err
}

func (s *Store) CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error) {
	now := time.Now()

	rCount, err := s.countRange(ctx, kind, now.Add(-recent), now)
	if err != nil {
		return FrequencyComparison{}, err
	}

	pCount, err := s.countRange(ctx, kind, now.Add(-(recent + previous)), now.Add(-recent))
	if err != nil {
		return FrequencyComparison{}, err
	}

	return FrequencyComparison{
		Kind:             kind,
		RecentCount:      rCount,
		PreviousCount:    pCount,
		PercentChange:    CalcChange(pCount, rCount),
		WindowHours:      recent.Hours(),
		PreviousWindowHr: previous.Hours(),
	}, nil
}

func (s *Store) StartGC(ctx context.Context, retention, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup(ctx, retention)
		}
	}
}

func (s *Store) countRange(ctx context.Context, kind IssueKind, start, end time.Time) (int, error) {
	var count int
	err := s.forEachInRange(ctx, kind, start, end, false, func(item *badger.Item) error {
		count++
		return nil
	})
	return count, err
}

func (s *Store) performCleanup(ctx context.Context, retention time.Duration) {
	cutoffNs := uint64(time.Now().Add(-retention).UnixNano())

	for _, kind := range SupportedKinds {
		_ = s.db.Update(func(txn *badger.Txn) error {
			prefix := []byte(string(kind) + ":")
			it := txn.NewIterator(badger.IteratorOptions{
				PrefetchValues: false,
			})
			defer it.Close()

			seek := s.buildKey(kind, time.Unix(0, 0), "")
			for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				key := it.Item().Key()
				ts := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])

				if ts >= cutoffNs {
					break
				}

				if err := txn.Delete(it.Item().KeyCopy(nil)); err != nil {
					return err
				}
			}
			return nil
		})
	}

	for s.db.RunValueLogGC(0.5) == nil {
	}
}

func (s *Store) forEachInRange(ctx context.Context, kind IssueKind, start, end time.Time, fetchValues bool, fn func(item *badger.Item) error) error {
	prefix := []byte(string(kind) + ":")
	endNs := uint64(end.UnixNano())

	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = fetchValues
		it := txn.NewIterator(opts)
		defer it.Close()

		seek := s.buildKey(kind, start, "")
		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			key := it.Item().Key()
			ts := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])

			if ts > endNs {
				break
			}

			if err := fn(it.Item()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) buildKey(kind IssueKind, ts time.Time, id string) []byte {
	pre := string(kind) + ":"
	buf := make([]byte, len(pre)+8+len(id))
	copy(buf, pre)
	binary.BigEndian.PutUint64(buf[len(pre):len(pre)+8], uint64(ts.UnixNano()))
	copy(buf[len(pre)+8:], id)
	return buf
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

func CalcChange(prev, curr int) float64 {
	if prev <= 0 {
		if curr > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(curr-prev) / float64(prev)) * 100.0
}

func resolvePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}
