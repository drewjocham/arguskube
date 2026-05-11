package alerts

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

	"github.com/argues/kube-watcher/pkg/kube/watch"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

type AlertRecord struct {
	ID         string                 `json:"id"`
	Alert      watch.Alert            `json:"alert"`
	ReceivedAt time.Time              `json:"received_at"`
	StoredAt   time.Time              `json:"stored_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	State      string                 `json:"state,omitempty"`
	Comments   []Comment              `json:"comments,omitempty"`
}

type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// PodCache represents cached data for a pod
type PodCache struct {
	PodUID      string    `json:"pod_uid"`
	PodName     string    `json:"pod_name"`
	Namespace   string    `json:"namespace"`
	CachedAt    time.Time `json:"cached_at"`
	Logs        string    `json:"logs,omitempty"`
	YAML        string    `json:"yaml,omitempty"`
	Describe    string    `json:"describe,omitempty"`
	AIHelp      string    `json:"ai_help,omitempty"`
	PodExists   bool      `json:"pod_exists"`
	LastChecked time.Time `json:"last_checked"`
}

type Store struct {
	db     *badger.DB
	closed bool
	mu     sync.RWMutex
}

type StoreInterface interface {
	StoreAlert(ctx context.Context, alert watch.Alert, metadata map[string]interface{}) error
	GetAlert(ctx context.Context, id string) (*AlertRecord, error)
	ListAlerts(ctx context.Context, since time.Time, kind, namespace string) ([]AlertRecord, error)
	UpdateAlertState(ctx context.Context, id string, state string) error
	AddAlertComment(ctx context.Context, id string, author string, content string) error
	StorePodCache(ctx context.Context, cache PodCache) error
	GetPodCache(ctx context.Context, namespace, podName string) (*PodCache, error)
	UpdatePodExistence(ctx context.Context, namespace, podName string, exists bool) error
	Close() error
}

var _ StoreInterface = (*Store)(nil)

func NewStore(path string) (*Store, error) {
	dir, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("alerts: failed to create storage dir: %w", err)
	}

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("alerts: failed to open badger: %w", err)
	}

	return &Store{db: db}, nil
}

func resolvePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("alerts: db path cannot be empty")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("alerts: failed to get home dir: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Abs(path)
}

func (s *Store) StoreAlert(ctx context.Context, alert watch.Alert, metadata map[string]interface{}) error {
	if s.closed {
		return errors.New("alerts: store is closed")
	}

	record := AlertRecord{
		ID:         generateAlertID(alert),
		Alert:      alert,
		ReceivedAt: alert.OccurredAt,
		StoredAt:   time.Now(),
		Metadata:   metadata,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("alerts: failed to marshal alert: %w", err)
	}

	key := alertKey(record.ID, alert.OccurredAt)
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key, data)
		return txn.SetEntry(e)
	})
}

func (s *Store) GetAlert(ctx context.Context, id string) (*AlertRecord, error) {
	if s.closed {
		return nil, errors.New("alerts: store is closed")
	}

	var record *AlertRecord
	err := s.db.View(func(txn *badger.Txn) error {
		// We need to scan since we don't have timestamp
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = []byte("alert:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var alertRec AlertRecord
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &alertRec)
			})
			if err != nil {
				continue
			}
			if alertRec.ID == id {
				record = &alertRec
				return nil
			}
		}
		return fmt.Errorf("alerts: alert not found: %s", id)
	})

	return record, err
}

func (s *Store) ListAlerts(ctx context.Context, since time.Time, kind, namespace string) ([]AlertRecord, error) {
	if s.closed {
		return nil, errors.New("alerts: store is closed")
	}

	var alerts []AlertRecord
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true // Newest first
		opts.Prefix = []byte("alert:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var alertRec AlertRecord
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &alertRec)
			})
			if err != nil {
				continue
			}

			// Apply filters
			if !alertRec.ReceivedAt.After(since) {
				continue
			}
			if kind != "" && string(alertRec.Alert.Kind) != kind {
				continue
			}
			if namespace != "" && alertRec.Alert.Namespace != namespace {
				continue
			}

			alerts = append(alerts, alertRec)
		}
		return nil
	})

	return alerts, err
}

// UpdateAlertState updates the state of an alert
func (s *Store) UpdateAlertState(ctx context.Context, id string, state string) error {
	if s.closed {
		return errors.New("alerts: store is closed")
	}

	return s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = []byte("alert:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var alertRec AlertRecord
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &alertRec)
			})
			if err != nil {
				continue
			}
			if alertRec.ID == id {
				alertRec.State = state
				data, err := json.Marshal(alertRec)
				if err != nil {
					return fmt.Errorf("alerts: failed to marshal updated alert: %w", err)
				}
				return txn.Set(item.Key(), data)
			}
		}
		return fmt.Errorf("alerts: alert not found: %s", id)
	})
}

// AddAlertComment adds a comment to an alert
func (s *Store) AddAlertComment(ctx context.Context, id string, author string, content string) error {
	if s.closed {
		return errors.New("alerts: store is closed")
	}

	return s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = []byte("alert:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var alertRec AlertRecord
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &alertRec)
			})
			if err != nil {
				continue
			}
			if alertRec.ID == id {
				comment := Comment{
					ID:        uuid.New().String(),
					Author:    author,
					Content:   content,
					CreatedAt: time.Now(),
				}
				alertRec.Comments = append(alertRec.Comments, comment)
				data, err := json.Marshal(alertRec)
				if err != nil {
					return fmt.Errorf("alerts: failed to marshal updated alert: %w", err)
				}
				return txn.Set(item.Key(), data)
			}
		}
		return fmt.Errorf("alerts: alert not found: %s", id)
	})
}

// StorePodCache stores cached pod data
func (s *Store) StorePodCache(ctx context.Context, cache PodCache) error {
	if s.closed {
		return errors.New("alerts: store is closed")
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("alerts: failed to marshal pod cache: %w", err)
	}

	key := podCacheKey(cache.Namespace, cache.PodName, cache.CachedAt)
	return s.db.Update(func(txn *badger.Txn) error {
		// Set with 7-day TTL for cache entries
		e := badger.NewEntry(key, data).WithTTL(7 * 24 * time.Hour)
		return txn.SetEntry(e)
	})
}

// GetPodCache retrieves the most recent cache for a pod
func (s *Store) GetPodCache(ctx context.Context, namespace, podName string) (*PodCache, error) {
	if s.closed {
		return nil, errors.New("alerts: store is closed")
	}

	var latestCache *PodCache
	var latestTime time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("podcache:%s:%s:", namespace, podName))
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true // Get newest first
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var cache PodCache
			err := item.Value(func(v []byte) error {
				return json.Unmarshal(v, &cache)
			})
			if err != nil {
				continue
			}

			if cache.CachedAt.After(latestTime) {
				latestTime = cache.CachedAt
				latestCache = &cache
			}
			// Just get the first (newest) entry
			break
		}
		return nil
	})

	if latestCache == nil {
		return nil, fmt.Errorf("alerts: no cache found for pod %s/%s", namespace, podName)
	}
	return latestCache, err
}

// UpdatePodExistence updates pod existence status in cache
func (s *Store) UpdatePodExistence(ctx context.Context, namespace, podName string, exists bool) error {
	if s.closed {
		return errors.New("alerts: store is closed")
	}

	// Get latest cache first
	cache, err := s.GetPodCache(ctx, namespace, podName)
	if err != nil {
		// No cache yet, create minimal one
		cache = &PodCache{
			PodName:     podName,
			Namespace:   namespace,
			CachedAt:    time.Now(),
			PodExists:   exists,
			LastChecked: time.Now(),
		}
	} else {
		cache.PodExists = exists
		cache.LastChecked = time.Now()
	}

	return s.StorePodCache(ctx, *cache)
}

// Close closes the database
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// Helper functions
func generateAlertID(alert watch.Alert) string {
	// Use alert key plus timestamp for uniqueness
	return fmt.Sprintf("%s:%d", alert.Key(), alert.OccurredAt.UnixNano())
}

// GenerateAlertID generates a unique ID for an alert (exported version)
func GenerateAlertID(alert watch.Alert) string {
	return generateAlertID(alert)
}

func alertKey(id string, timestamp time.Time) []byte {
	// Key format: alert:<timestamp_unix_nano>:<id>
	// This allows time-based iteration
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(timestamp.UnixNano()))
	return []byte(fmt.Sprintf("alert:%x:%s", tsBytes, id))
}

func podCacheKey(namespace, podName string, timestamp time.Time) []byte {
	// Key format: podcache:<namespace>:<podname>:<timestamp_unix_nano>
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(timestamp.UnixNano()))
	return []byte(fmt.Sprintf("podcache:%s:%s:%x", namespace, podName, tsBytes))
}
