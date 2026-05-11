package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PR guidelines are short-to-medium markdown documents the user maintains per
// pipeline provider. They feed downstream AI features (PR summary, code review)
// so each provider keeps its own copy on disk under
// $HOME/.argus/pr-guidelines/<provider>.md.

const maxPRGuidelinesBytes = 256 * 1024 // 256KB cap; rules docs are prose, not data

var validProviderID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func prGuidelinesPath(provider string) (string, error) {
	if !validProviderID.MatchString(provider) {
		return "", fmt.Errorf("invalid provider id: %q", provider)
	}
	dir := filepath.Join(os.ExpandEnv("$HOME"), ".argus", "pr-guidelines")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("create pr-guidelines dir: %w", err)
	}
	return filepath.Join(dir, provider+".md"), nil
}

// GetPRGuidelines returns the markdown rules document associated with the given
// pipeline provider. An empty string is returned (without error) when no
// document has been saved yet.
func (a *App) GetPRGuidelines(provider string) (string, error) {
	path, err := prGuidelinesPath(provider)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read pr guidelines: %w", err)
	}
	return string(data), nil
}

// SavePRGuidelines persists the markdown rules document for a provider. Empty
// content removes the document on disk so the user can fully clear their rules.
func (a *App) SavePRGuidelines(provider, content string) error {
	if len(content) > maxPRGuidelinesBytes {
		return fmt.Errorf("guidelines too large: %d bytes (max %d)", len(content), maxPRGuidelinesBytes)
	}
	path, err := prGuidelinesPath(provider)
	if err != nil {
		return err
	}
	if content == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete pr guidelines: %w", err)
		}
		a.logger.Info("pr guidelines cleared", slog.String("provider", provider))
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return fmt.Errorf("write pr guidelines: %w", err)
	}
	a.logger.Info("pr guidelines saved",
		slog.String("provider", provider),
		slog.Int("bytes", len(content)),
	)
	return nil
}

// --- Code review reports ---
//
// Reports live at $HOME/.argus/code-reviews/<provider>/<id>.md, where
// <id> is "<unix-ts>-<slug>" so an alphabetical directory listing is reverse-
// chronological-friendly via sort.Reverse. Each file is fenced YAML-style
// frontmatter (title, prRef, createdAt) followed by the markdown body. A
// future AI integration will append to the same store.

// CodeReviewReport is the metadata returned to the frontend for each report.
// The body is fetched separately via GetCodeReviewReport.
type CodeReviewReport struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Provider  string    `json:"provider"`
	PRRef     string    `json:"prRef"`
	CreatedAt time.Time `json:"createdAt"`
}

const maxCodeReviewBytes = 1024 * 1024 // 1MB per report

var (
	idSafe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	slugRe = regexp.MustCompile(`[^a-z0-9]+`)
)

func codeReviewDir(provider string) (string, error) {
	if !validProviderID.MatchString(provider) {
		return "", fmt.Errorf("invalid provider id: %q", provider)
	}
	dir := filepath.Join(os.ExpandEnv("$HOME"), ".argus", "code-reviews", provider)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("create code-reviews dir: %w", err)
	}
	return dir, nil
}

func codeReviewPath(provider, id string) (string, error) {
	if !idSafe.MatchString(id) {
		return "", fmt.Errorf("invalid report id: %q", id)
	}
	dir, err := codeReviewDir(provider)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".md"), nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
	}
	if s == "" {
		s = "review"
	}
	return s
}

// splitFrontmatter parses a "---\nkey: value\n---\n<body>" block. If the file
// has no frontmatter the whole content is returned as the body and meta is nil.
func splitFrontmatter(raw string) (meta map[string]string, body string) {
	body = raw
	if !strings.HasPrefix(raw, "---\n") {
		return nil, raw
	}
	rest := raw[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, raw
	}
	header := rest[:end]
	body = rest[end+5:]
	meta = map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		v = strings.Trim(v, `"`)
		if k != "" {
			meta[k] = v
		}
	}
	return meta, body
}

func renderFrontmatter(title, prRef string, createdAt time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	if prRef != "" {
		fmt.Fprintf(&b, "prRef: %q\n", prRef)
	}
	fmt.Fprintf(&b, "createdAt: %s\n", createdAt.UTC().Format(time.RFC3339))
	b.WriteString("---\n")
	return b.String()
}

// ListCodeReviewReports returns metadata for every report stored under the
// given provider, newest first.
func (a *App) ListCodeReviewReports(provider string) ([]CodeReviewReport, error) {
	dir, err := codeReviewDir(provider)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read code-reviews dir: %w", err)
	}
	out := make([]CodeReviewReport, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			a.logger.Warn("failed to read code review report",
				slog.String("id", id), slog.String("error", err.Error()))
			continue
		}
		meta, _ := splitFrontmatter(string(raw))
		title := id
		var prRef string
		var createdAt time.Time
		if meta != nil {
			if v, ok := meta["title"]; ok && v != "" {
				title = v
			}
			prRef = meta["prRef"]
			if v, ok := meta["createdAt"]; ok {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					createdAt = t
				}
			}
		}
		if createdAt.IsZero() {
			if info, err := e.Info(); err == nil {
				createdAt = info.ModTime()
			}
		}
		out = append(out, CodeReviewReport{
			ID:        id,
			Title:     title,
			Provider:  provider,
			PRRef:     prRef,
			CreatedAt: createdAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// GetCodeReviewReport returns the markdown body of a stored report,
// frontmatter stripped.
func (a *App) GetCodeReviewReport(provider, id string) (string, error) {
	path, err := codeReviewPath(provider, id)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read code review report: %w", err)
	}
	_, body := splitFrontmatter(string(raw))
	return body, nil
}

// CreateCodeReviewReport persists a new report and returns its metadata. ID is
// derived from the current timestamp and a slug of the title so listings stay
// reverse-chronological. Title and body are required; prRef is optional.
func (a *App) CreateCodeReviewReport(provider, title, prRef, body string) (CodeReviewReport, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return CodeReviewReport{}, fmt.Errorf("title is required")
	}
	if len(body) > maxCodeReviewBytes {
		return CodeReviewReport{}, fmt.Errorf("report too large: %d bytes (max %d)", len(body), maxCodeReviewBytes)
	}
	dir, err := codeReviewDir(provider)
	if err != nil {
		return CodeReviewReport{}, err
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("%d-%s", now.Unix(), slugify(title))
	path := filepath.Join(dir, id+".md")
	content := renderFrontmatter(title, prRef, now) + body
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return CodeReviewReport{}, fmt.Errorf("write code review report: %w", err)
	}
	a.logger.Info("code review report saved",
		slog.String("provider", provider),
		slog.String("id", id),
		slog.Int("bytes", len(body)),
	)
	return CodeReviewReport{
		ID:        id,
		Title:     title,
		Provider:  provider,
		PRRef:     prRef,
		CreatedAt: now,
	}, nil
}

// DeleteCodeReviewReport removes a report file. Missing files are not an error.
func (a *App) DeleteCodeReviewReport(provider, id string) error {
	path, err := codeReviewPath(provider, id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete code review report: %w", err)
	}
	a.logger.Info("code review report deleted",
		slog.String("provider", provider),
		slog.String("id", id),
	)
	return nil
}
