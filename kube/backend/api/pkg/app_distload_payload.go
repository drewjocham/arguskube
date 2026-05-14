package pkg

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/features"
)

// app_distload_payload.go — payload sources + local-run quota for the
// Distributed Load Test feature. The frontend can pick from five
// sources (upload, paste, type, AI, file/dir); this file owns the
// AI generator + the local file resolver, plus the daily quota the
// free tier sees on local runs.

// --- AI payload generator --------------------------------------------

// payloadAISystemPrompt nudges the model toward a single self-contained
// JSON object of roughly the requested size. No markdown fences, no
// commentary — the validator below rejects anything that isn't a
// parseable JSON value and retries once after stripping fences.
const payloadAISystemPrompt = `You generate sample message bodies for load tests.
Respond with a SINGLE JSON value (object or array) that represents a realistic
message body for the user's description. Output ONLY the JSON — no markdown
fences, no commentary, no leading/trailing text.`

// GenerateLoadTestPayload asks the configured LLM for a JSON body
// matching the user's description. sizeHint is best-effort: it's
// injected into the prompt but the model isn't held to it.
//
// Returns the validated JSON string (parseable via json.Unmarshal into
// an interface{}). Errors carry the model's response head when the
// reply isn't valid JSON so the UI can show why it failed.
func (a *App) GenerateLoadTestPayload(prompt string, sizeHint int) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt required")
	}
	if a.agent == nil || !a.agent.HasClient() {
		return "", fmt.Errorf("AI agent disabled — set DEEPSEEK_API_KEY")
	}
	client := a.agent.Client()

	userMsg := prompt
	if sizeHint > 0 {
		userMsg = fmt.Sprintf("%s\n\nTarget size: roughly %d bytes.", prompt, sizeHint)
	}

	messages := []ai.Message{
		{Role: "system", Content: payloadAISystemPrompt},
		{Role: "user", Content: userMsg},
	}
	resp, err := client.Chat(a.appCtx(), messages)
	if err != nil {
		return "", fmt.Errorf("ai chat: %w", err)
	}

	if cleaned, ok := validateJSON(resp); ok {
		return cleaned, nil
	}
	// One retry after stripping ```json fences (the model often
	// disregards "no markdown" once, but rarely twice).
	stripped := stripJSONFences(resp)
	if cleaned, ok := validateJSON(stripped); ok {
		return cleaned, nil
	}
	head := resp
	if len(head) > 200 {
		head = head[:200]
	}
	return "", fmt.Errorf("ai response was not valid JSON: %s", head)
}

func validateJSON(s string) (string, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return "", false
	}
	var v any
	if err := json.Unmarshal([]byte(t), &v); err != nil {
		return "", false
	}
	return t, true
}

func stripJSONFences(s string) string {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```JSON")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	return strings.TrimSpace(t)
}

// --- Local file/dir resolver -----------------------------------------

// PayloadFileInfo is one entry the resolver returns for a directory
// listing (or the single file when path was a file).
type PayloadFileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// PayloadPathResolution is the response shape for ResolveLocalPayloadPath.
// Files lists at most resolvePayloadMaxFiles entries, sorted by name.
// Sample previews the first file (up to resolvePayloadSampleBytes).
type PayloadPathResolution struct {
	Kind   string            `json:"kind"`
	Files  []PayloadFileInfo `json:"files"`
	Sample string            `json:"sample,omitempty"`
}

const (
	resolvePayloadMaxFiles    = 100
	resolvePayloadSampleBytes = 2048
	resolvePayloadMaxFileSize = 10 << 20 // 10 MiB
)

// ResolveLocalPayloadPath inspects a local file or directory the user
// pointed at as a payload source. Bounded + sandboxed — see safety
// rules in checkPayloadPath. Only .json and .txt files are listed.
func (a *App) ResolveLocalPayloadPath(path string) (PayloadPathResolution, error) {
	clean, err := checkPayloadPath(path)
	if err != nil {
		return PayloadPathResolution{}, err
	}
	info, err := os.Stat(clean)
	if err != nil {
		return PayloadPathResolution{}, fmt.Errorf("stat %s: %w", clean, err)
	}
	if info.IsDir() {
		return resolveDir(clean)
	}
	return resolveFile(clean, info)
}

// checkPayloadPath is the safety gate: rejects empty, root, relative
// traversal, or paths outside the user's home / mac scratch dirs. The
// home dir is the trust root because that's where the user's own data
// lives; /tmp and /var/folders are mac scratch the GUI commonly uses.
func checkPayloadPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	// Reject ".." as a *segment* on the raw input — both to catch
	// traversal attempts like /etc/../tmp (which Clean would silently
	// flatten to /tmp) and to leave legitimate filenames like
	// "myfile..bak" alone.
	for _, seg := range strings.Split(path, string(filepath.Separator)) {
		if seg == ".." {
			return "", fmt.Errorf("path must not contain '..'")
		}
	}
	clean := filepath.Clean(path)
	if clean == "/" || clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("path %q is not allowed", path)
	}
	// Windows root drives (best-effort — Argus desktop is mac-first).
	if len(clean) == 3 && clean[1] == ':' && (clean[2] == '\\' || clean[2] == '/') {
		return "", fmt.Errorf("path %q is not allowed", path)
	}
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be absolute")
	}
	allowed := false
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if hasPrefixDir(clean, home) {
			allowed = true
		}
	}
	if !allowed && (hasPrefixDir(clean, "/tmp") || hasPrefixDir(clean, "/var/folders") || hasPrefixDir(clean, "/private/var/folders") || hasPrefixDir(clean, "/private/tmp")) {
		allowed = true
	}
	if !allowed {
		return "", fmt.Errorf("path %q is outside the allowed roots (home, /tmp, /var/folders)", path)
	}
	// Resolve symlinks so a path like ~/link-to-etc can't escape the
	// trust root. EvalSymlinks errors on a non-existent path — fine,
	// the caller's os.Stat will surface a clearer error than ours.
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		allowed = false
		if home, herr := os.UserHomeDir(); herr == nil && home != "" {
			if resolvedHome, herr2 := filepath.EvalSymlinks(home); herr2 == nil {
				if hasPrefixDir(resolved, resolvedHome) {
					allowed = true
				}
			}
		}
		if !allowed && (hasPrefixDir(resolved, "/tmp") || hasPrefixDir(resolved, "/var/folders") || hasPrefixDir(resolved, "/private/var/folders") || hasPrefixDir(resolved, "/private/tmp")) {
			allowed = true
		}
		if !allowed {
			return "", fmt.Errorf("path %q resolves outside the allowed roots", path)
		}
		clean = resolved
	}
	return clean, nil
}

// hasPrefixDir returns true if p is at or under base, by path segment
// (so "/foo" is NOT a prefix of "/foobar").
func hasPrefixDir(p, base string) bool {
	cb := filepath.Clean(base)
	if p == cb {
		return true
	}
	return strings.HasPrefix(p, cb+string(filepath.Separator))
}

func resolveFile(path string, info os.FileInfo) (PayloadPathResolution, error) {
	res := PayloadPathResolution{
		Kind: "file",
		Files: []PayloadFileInfo{{
			Name: filepath.Base(path),
			Size: info.Size(),
			Path: path,
		}},
	}
	if info.Size() > 0 && info.Size() <= resolvePayloadMaxFileSize {
		sample, _ := readSample(path, resolvePayloadSampleBytes)
		res.Sample = sample
	}
	return res, nil
}

func resolveDir(dir string) (PayloadPathResolution, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return PayloadPathResolution{}, fmt.Errorf("read dir %s: %w", dir, err)
	}
	files := make([]PayloadFileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".txt") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() > resolvePayloadMaxFileSize {
			// Skip silently — if the user picks this dir as a pool,
			// these would fail anyway.
			continue
		}
		files = append(files, PayloadFileInfo{
			Name: name,
			Size: info.Size(),
			Path: filepath.Join(dir, name),
		})
		if len(files) >= resolvePayloadMaxFiles {
			break
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	res := PayloadPathResolution{Kind: "dir", Files: files}
	if len(files) > 0 {
		sample, _ := readSample(files[0].Path, resolvePayloadSampleBytes)
		res.Sample = sample
	}
	return res, nil
}

func readSample(path string, max int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, max)
	n, _ := f.Read(buf)
	return string(buf[:n]), nil
}

// --- Daily local-run quota -------------------------------------------

// localQuotaFreeLimit caps local runs/day for the free tier. Pro is
// uncapped. The window rolls forward (started_at > now - 24h), not
// midnight-aligned — keeps the math simple and the UX predictable.
const localQuotaFreeLimit = 5

// LocalQuotaStatus is the Wails-exposed view of the user's remaining
// local-run budget. resetAt is the unix-seconds time when the oldest
// run in the current window falls out, i.e. when used drops by one.
type LocalQuotaStatus struct {
	Used    int   `json:"used"`
	Limit   int   `json:"limit"`
	ResetAt int64 `json:"resetAt"`
	IsPro   bool  `json:"isPro"`
}

// localQuotaStatus returns the current usage + the resolved cap. The
// cap is math.MaxInt32 for Pro (the JSON encoder still renders this as
// a plain number, but we keep it small enough to avoid overflow
// surprises in the frontend).
func (a *App) localQuotaStatus() (used int, limit int, resetAt int64, isPro bool, err error) {
	isPro = a.gate != nil && a.gate.Allowed(features.FeatureDistributedLoadTest)
	if isPro {
		limit = math.MaxInt32
	} else {
		limit = localQuotaFreeLimit
	}
	if a.db == nil {
		return 0, limit, 0, isPro, nil
	}
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	if scanErr := a.db.QueryRow(
		`SELECT COUNT(*) FROM distload_local_runs WHERE started_at > ?`, cutoff,
	).Scan(&used); scanErr != nil {
		return 0, limit, 0, isPro, fmt.Errorf("query quota: %w", scanErr)
	}
	// resetAt is "oldest in-window + 24h" — when the oldest row exits
	// the rolling window. 0 if the user has no runs in the window.
	if used > 0 {
		var oldest int64
		if scanErr := a.db.QueryRow(
			`SELECT started_at FROM distload_local_runs WHERE started_at > ? ORDER BY started_at ASC LIMIT 1`, cutoff,
		).Scan(&oldest); scanErr == nil {
			resetAt = oldest + int64(24*time.Hour/time.Second)
		}
	}
	return used, limit, resetAt, isPro, nil
}

// GetLocalDistLoadQuota is the Wails RPC: returns the user's remaining
// daily budget so the UI can render a counter.
func (a *App) GetLocalDistLoadQuota() (LocalQuotaStatus, error) {
	used, limit, resetAt, isPro, err := a.localQuotaStatus()
	if err != nil {
		return LocalQuotaStatus{}, err
	}
	return LocalQuotaStatus{Used: used, Limit: limit, ResetAt: resetAt, IsPro: isPro}, nil
}

// recordLocalDistLoadRun appends a row to the quota table. Called after
// a successful start so a rejected start doesn't burn the user's
// allowance.
func (a *App) recordLocalDistLoadRun(runID string, startedAt time.Time) error {
	if a.db == nil {
		return nil
	}
	_, err := a.db.Exec(
		`INSERT INTO distload_local_runs (run_id, started_at) VALUES (?, ?)`,
		runID, startedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("record local run: %w", err)
	}
	return nil
}

// reserveLocalQuotaSlot atomically checks remaining capacity and, if
// available, inserts a quota row in the same transaction. This is the
// concurrency-safe variant of "localQuotaStatus + recordLocalDistLoadRun"
// — two parallel Start calls at the 4→5 boundary cannot both succeed
// because the SELECT…INSERT runs inside a SQLite IMMEDIATE transaction
// (which acquires the write lock at BEGIN, not on first write).
//
// Returns ErrLocalQuotaExceeded when the cap is hit; the caller renders
// a human-readable message that includes resetAt.
func (a *App) reserveLocalQuotaSlot(runID string, startedAt time.Time) (used, limit int, resetAt int64, err error) {
	isPro := a.gate != nil && a.gate.Allowed(features.FeatureDistributedLoadTest)
	if isPro {
		limit = math.MaxInt32
	} else {
		limit = localQuotaFreeLimit
	}
	if a.db == nil {
		// In-memory / tests without a DB: skip enforcement.
		return 0, limit, 0, nil
	}

	tx, err := a.db.BeginTx(a.appCtx(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, limit, 0, fmt.Errorf("reserve quota: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	cutoff := startedAt.Add(-24 * time.Hour).Unix()
	if err = tx.QueryRow(
		`SELECT COUNT(*) FROM distload_local_runs WHERE started_at > ?`, cutoff,
	).Scan(&used); err != nil {
		return 0, limit, 0, fmt.Errorf("reserve quota: count: %w", err)
	}
	if used >= limit {
		// Read resetAt so the caller can render a useful error.
		if used > 0 {
			var oldest int64
			if scanErr := tx.QueryRow(
				`SELECT started_at FROM distload_local_runs WHERE started_at > ? ORDER BY started_at ASC LIMIT 1`, cutoff,
			).Scan(&oldest); scanErr == nil {
				resetAt = oldest + int64(24*time.Hour/time.Second)
			}
		}
		_ = tx.Rollback()
		return used, limit, resetAt, ErrLocalQuotaExceeded
	}
	if _, err = tx.Exec(
		`INSERT INTO distload_local_runs (run_id, started_at) VALUES (?, ?)`,
		runID, startedAt.Unix(),
	); err != nil {
		return 0, limit, 0, fmt.Errorf("reserve quota: insert: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, limit, 0, fmt.Errorf("reserve quota: commit: %w", err)
	}
	return used + 1, limit, 0, nil
}

// ErrLocalQuotaExceeded is the sentinel returned by reserveLocalQuotaSlot
// when the daily cap is reached. The caller wraps it with the resetAt
// timestamp for display.
var ErrLocalQuotaExceeded = fmt.Errorf("local load tests are limited to %d/day on the free tier", localQuotaFreeLimit)

// refundLocalQuotaSlot deletes a previously-reserved quota row. Called
// when a Start request passes the quota check but then fails (e.g. the
// engine registry says another run is already active). Best-effort —
// failures here are logged by the caller, not returned.
func (a *App) refundLocalQuotaSlot(runID string) {
	if a.db == nil {
		return
	}
	_, _ = a.db.Exec(`DELETE FROM distload_local_runs WHERE run_id = ?`, runID)
}
