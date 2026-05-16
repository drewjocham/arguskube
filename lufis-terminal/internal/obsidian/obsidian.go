package obsidian

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Vault struct {
	Path string
	Name string
}

type Note struct {
	Path        string
	Title       string
	Content     string
	Tags        []string
	Wikilinks   []string
	Frontmatter map[string]string
}

type Client struct {
	vaults []Vault
}

func NewClient() *Client {
	return &Client{vaults: findVaults()}
}

func findVaults() []Vault {
	var vaults []Vault
	home, _ := os.UserHomeDir()
	searchPaths := []string{
		home,
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Dropbox"),
		filepath.Join(home, "iCloud"),
		filepath.Join(home, "Library", "Mobile Documents", "iCloud~md~obsidian"),
	}
	for _, p := range searchPaths {
		discoverVault(p, &vaults)
	}
	return vaults
}

func discoverVault(root string, vaults *[]Vault) {
	if root == "" {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".obsidian" {
			continue
		}
		obsPath := filepath.Join(root, e.Name(), ".obsidian")
		if info, err := os.Stat(obsPath); err == nil && info.IsDir() {
			*vaults = append(*vaults, Vault{
				Path: filepath.Join(root, e.Name()),
				Name: e.Name(),
			})
		}
	}
}

func (c *Client) Vaults() []Vault {
	return c.vaults
}

func (c *Client) Notes(name string) ([]Note, error) {
	vault := c.find(name)
	if vault == nil {
		return nil, fmt.Errorf("vault %s not found", name)
	}
	return c.readNotes(vault.Path), nil
}

func (c *Client) AllNotes() ([]Note, error) {
	var all []Note
	for _, v := range c.vaults {
		notes := c.readNotes(v.Path)
		all = append(all, notes...)
	}
	return all, nil
}

func (c *Client) CreateNote(vaultName, relPath, content string) error {
	vault := c.find(vaultName)
	if vault == nil {
		return fmt.Errorf("vault %s not found", vaultName)
	}
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	fullPath := filepath.Join(vault.Path, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(fullPath, []byte(content), 0o644)
}

func (c *Client) find(name string) *Vault {
	for i := range c.vaults {
		if c.vaults[i].Name == name || c.vaults[i].Path == name {
			return &c.vaults[i]
		}
	}
	return nil
}

func (c *Client) readNotes(vaultPath string) []Note {
	var notes []Note
	_ = filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		note := parseNote(path, string(data))
		notes = append(notes, note)
		return nil
	})
	return notes
}

func parseNote(path, content string) Note {
	note := Note{
		Path:    path,
		Content: content,
		Title:   filepath.Base(path),
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "---") {
		note.Frontmatter = make(map[string]string)
		for i := 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "---") {
				break
			}
			parts := strings.SplitN(lines[i], ":", 2)
			if len(parts) == 2 {
				note.Frontmatter[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	for _, line := range lines {
		note.Tags = append(note.Tags, extractTags(line)...)
	}
	for _, line := range lines {
		note.Wikilinks = append(note.Wikilinks, extractWikilinks(line)...)
	}

	return note
}

func extractTags(line string) []string {
	var tags []string
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.HasPrefix(f, "#") && len(f) > 1 {
			tag := strings.TrimRight(f, ",.;:!?")
			tags = append(tags, tag[1:])
		}
	}
	return tags
}

func extractWikilinks(line string) []string {
	var links []string
	remaining := line
	for {
		start := strings.Index(remaining, "[[")
		if start < 0 {
			break
		}
		remaining = remaining[start+2:]
		end := strings.Index(remaining, "]]")
		if end < 0 {
			break
		}
		link := remaining[:end]
		if pipe := strings.Index(link, "|"); pipe >= 0 {
			link = link[:pipe]
		}
		links = append(links, link)
		remaining = remaining[end+2:]
	}
	return links
}
