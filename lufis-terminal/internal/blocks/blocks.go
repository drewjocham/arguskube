package blocks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Block struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Output    []string  `json:"output"`
	ExitCode  int       `json:"exit_code"`
	Duration  string    `json:"duration"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	NoteID    string    `json:"note_id,omitempty"`
	Tag       string    `json:"tag,omitempty"`
}

type Store struct {
	mu     sync.Mutex
	path   string
	blocks map[string]*Block
	order  []string
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home: %w", err)
		}
		dataDir = filepath.Join(home, ".config", "argus-terminal")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	s := &Store{
		path:   filepath.Join(dataDir, "blocks.json"),
		blocks: make(map[string]*Block),
	}
	s.load()
	return s, nil
}

func (s *Store) Save(cmd string, exitCode int, output string, duration time.Duration) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("b-%d", time.Now().UnixNano())
	block := &Block{
		ID:        id,
		Command:   cmd,
		Output:    []string{output},
		ExitCode:  exitCode,
		Duration:  duration.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.blocks[id] = block
	s.order = append(s.order, id)
	s.persist()
	return block
}

func (s *Store) Get(id string) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocks[id]
}

func (s *Store) Rerun(id, newOutput string, exitCode int, duration time.Duration) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()

	block, ok := s.blocks[id]
	if !ok {
		return nil
	}
	block.Output = append(block.Output, newOutput)
	block.ExitCode = exitCode
	block.Duration = duration.String()
	block.UpdatedAt = time.Now()
	s.persist()
	return block
}

func (s *Store) Edit(id, newCommand string) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()

	block, ok := s.blocks[id]
	if !ok {
		return nil
	}
	block.Command = newCommand
	block.UpdatedAt = time.Now()
	s.persist()
	return block
}

func (s *Store) List(limit int) []*Block {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := len(s.order)
	if limit > 0 && limit < n {
		n = limit
	}
	start := len(s.order) - n
	if start < 0 {
		start = 0
	}
	result := make([]*Block, n)
	for i, id := range s.order[start:] {
		result[i] = s.blocks[id]
	}
	return result
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.blocks, id)
	for i, oid := range s.order {
		if oid == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	s.persist()
}

func (s *Store) Search(query string) []*Block {
	s.mu.Lock()
	defer s.mu.Unlock()

	q := strings.ToLower(query)
	var results []*Block
	for _, id := range s.order {
		block := s.blocks[id]
		if strings.Contains(strings.ToLower(block.Command), q) {
			results = append(results, block)
		}
	}
	return results
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var list []*Block
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, b := range list {
		s.blocks[b.ID] = b
		s.order = append(s.order, b.ID)
	}
}

func (s *Store) persist() {
	list := make([]*Block, 0, len(s.order))
	for _, id := range s.order {
		list = append(list, s.blocks[id])
	}
	data, _ := json.Marshal(list)
	tmp := s.path + ".tmp"
	_ = os.WriteFile(tmp, data, 0o600)
	_ = os.Rename(tmp, s.path)
}
