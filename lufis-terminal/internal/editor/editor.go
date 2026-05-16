package editor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Tab struct {
	Path    string
	Name    string
	Content []string
	Dirty   bool
	Cursor  Cursor
}

type Cursor struct {
	Line int
	Col  int
}

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []FileNode
	GitState string
}

type Buffer struct {
	mu     sync.Mutex
	tabs   []*Tab
	active int
}

func NewBuffer() *Buffer {
	return &Buffer{}
}

func (b *Buffer) Open(path string) (*Tab, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, t := range b.tabs {
		if t.Path == path {
			return t, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tab := &Tab{
		Path:    path,
		Name:    filepath.Base(path),
		Content: strings.Split(string(data), "\n"),
	}
	b.tabs = append(b.tabs, tab)
	b.active = len(b.tabs) - 1
	return tab, nil
}

func (b *Buffer) Active() *Tab {
	if b.active < len(b.tabs) {
		return b.tabs[b.active]
	}
	return nil
}

func (b *Buffer) Tabs() []*Tab { return b.tabs }

func (b *Buffer) Close(index int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if index < 0 || index >= len(b.tabs) {
		return
	}
	b.tabs = append(b.tabs[:index], b.tabs[index+1:]...)
	if b.active >= len(b.tabs) {
		b.active = len(b.tabs) - 1
	}
}

func (b *Buffer) Save() error {
	tab := b.Active()
	if tab == nil || !tab.Dirty {
		return nil
	}
	data := strings.Join(tab.Content, "\n")
	if err := os.WriteFile(tab.Path, []byte(data), 0o644); err != nil {
		return err
	}
	tab.Dirty = false
	return nil
}

func (b *Buffer) Insert(path string) error {
	tab, err := b.Open(path)
	if err != nil {
		return err
	}
	tab.Dirty = true
	return nil
}

func (b *Buffer) InsertAt(text string, line, col int) {
	tab := b.Active()
	if tab == nil || line < 0 || line >= len(tab.Content) {
		return
	}
	tab.Dirty = true
	row := tab.Content[line]
	if col > len(row) {
		col = len(row)
	}
	tab.Content[line] = row[:col] + text + row[col:]
	tab.Cursor = Cursor{Line: line, Col: col + len(text)}
}

func (b *Buffer) DeleteLine(line int) {
	tab := b.Active()
	if tab == nil || line < 0 || line >= len(tab.Content) {
		return
	}
	tab.Dirty = true
	tab.Content = append(tab.Content[:line], tab.Content[line+1:]...)
	if tab.Cursor.Line >= len(tab.Content) {
		tab.Cursor.Line = len(tab.Content) - 1
	}
}

func (b *Buffer) Search(query string) []struct {
	Tab  int
	Line int
	Col  int
	Text string
} {
	var results []struct {
		Tab  int
		Line int
		Col  int
		Text string
	}
	q := strings.ToLower(query)
	for ti, tab := range b.tabs {
		for li, line := range tab.Content {
			col := strings.Index(strings.ToLower(line), q)
			if col >= 0 {
				result := struct {
					Tab  int
					Line int
					Col  int
					Text string
				}{Tab: ti, Line: li, Col: col, Text: strings.TrimSpace(line)}
				results = append(results, result)
			}
		}
	}
	return results
}

func FileTree(root string, maxDepth int) []FileNode {
	var build func(dir string, depth int) []FileNode
	build = func(dir string, depth int) []FileNode {
		if depth > maxDepth {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		var nodes []FileNode
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			node := FileNode{
				Name:  e.Name(),
				Path:  filepath.Join(dir, e.Name()),
				IsDir: e.IsDir(),
			}
			if e.IsDir() {
				node.Children = build(node.Path, depth+1)
			}
			nodes = append(nodes, node)
		}
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsDir != nodes[j].IsDir {
				return nodes[i].IsDir
			}
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		})
		return nodes
	}
	return build(root, 0)
}

func SyntaxHighlight(fileType string) map[string]string {
	tokens := make(map[string]string)
	switch fileType {
	case "go":
		tokens["package"] = "keyword"
		tokens["import"] = "keyword"
		tokens["func"] = "keyword"
		tokens["return"] = "keyword"
		tokens["if"] = "keyword"
		tokens["else"] = "keyword"
		tokens["for"] = "keyword"
		tokens["range"] = "keyword"
		tokens["var"] = "keyword"
		tokens["const"] = "keyword"
		tokens["type"] = "keyword"
		tokens["struct"] = "keyword"
		tokens["interface"] = "keyword"
		tokens["map"] = "keyword"
		tokens["chan"] = "keyword"
		tokens["go"] = "keyword"
		tokens["defer"] = "keyword"
		tokens["select"] = "keyword"
		tokens["switch"] = "keyword"
		tokens["case"] = "keyword"
		tokens["default"] = "keyword"
		tokens["break"] = "keyword"
		tokens["continue"] = "keyword"
	case "py":
		tokens["def"] = "keyword"
		tokens["class"] = "keyword"
		tokens["import"] = "keyword"
		tokens["from"] = "keyword"
		tokens["return"] = "keyword"
		tokens["if"] = "keyword"
		tokens["elif"] = "keyword"
		tokens["else"] = "keyword"
		tokens["for"] = "keyword"
		tokens["while"] = "keyword"
		tokens["in"] = "keyword"
		tokens["not"] = "keyword"
		tokens["and"] = "keyword"
		tokens["or"] = "keyword"
		tokens["True"] = "constant"
		tokens["False"] = "constant"
		tokens["None"] = "constant"
	case "js", "ts":
		tokens["function"] = "keyword"
		tokens["const"] = "keyword"
		tokens["let"] = "keyword"
		tokens["var"] = "keyword"
		tokens["return"] = "keyword"
		tokens["if"] = "keyword"
		tokens["else"] = "keyword"
		tokens["for"] = "keyword"
		tokens["while"] = "keyword"
		tokens["class"] = "keyword"
		tokens["import"] = "keyword"
		tokens["export"] = "keyword"
		tokens["from"] = "keyword"
		tokens["async"] = "keyword"
		tokens["await"] = "keyword"
	case "yaml", "yml":
		tokens["true"] = "constant"
		tokens["false"] = "constant"
		tokens["yes"] = "constant"
		tokens["no"] = "constant"
		tokens["on"] = "constant"
		tokens["off"] = "constant"
	case "hcl", "tf":
		tokens["resource"] = "keyword"
		tokens["data"] = "keyword"
		tokens["variable"] = "keyword"
		tokens["output"] = "keyword"
		tokens["module"] = "keyword"
		tokens["provider"] = "keyword"
		tokens["terraform"] = "keyword"
		tokens["locals"] = "keyword"
	}
	return tokens
}
