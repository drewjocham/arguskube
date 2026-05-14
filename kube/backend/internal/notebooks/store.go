package notebooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "github.com/argues/argus/internal/config"
)

// FileEntry represents a notebook file or folder in the tree structure.
type FileEntry struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "folder"
	Children []FileEntry `json:"children,omitempty"`
	Modified time.Time   `json:"modified"`
}

// Store manages notebook files in S3 with local caching.
type Store struct {
	logger     *slog.Logger
	s3Client   *s3.Client
	bucket     string
	cacheDir   string
	configured bool

	// uploadCtx scopes async uploads to the store's lifetime. SaveFile
	// kicks off uploads in a detached goroutine (so the user's RPC
	// returns before the network round-trip), which means we can NOT
	// pass the caller's ctx — that ctx is canceled the moment the
	// caller returns and the upload would abort mid-PUT. But we also
	// can't use context.Background() (the previous code), because then
	// Close()/shutdown can't cancel in-flight uploads and they survive
	// the process. uploadCtx + uploadCancel split the difference:
	// detached from the caller, but cancelable by the store's owner.
	uploadCtx    context.Context
	uploadCancel context.CancelFunc
}

// Close cancels any in-flight async uploads and releases resources.
// Safe to call multiple times; Close() on a never-Closed store is a
// no-op for the cancel func too (CancelFunc is idempotent).
func (st *Store) Close() {
	if st.uploadCancel != nil {
		st.uploadCancel()
	}
}

// resolveLocal turns a frontend-supplied logical path into an absolute
// filesystem path INSIDE the cache dir, refusing any input that would
// resolve outside it. The previous code used `filepath.Clean(path)`
// directly inside `filepath.Join(st.cacheDir, ...)`, which does NOT
// strip leading `../` segments — `filepath.Join("/cache", "../etc")`
// happily resolves to `/etc`. Because `path` arrives over Wails RPC and
// is user-controlled, that was a directory-traversal sink (read /
// overwrite / delete of arbitrary user-readable files: ~/.ssh, vault,
// kubeconfig). Two-stage defense:
//
//  1. Prefix with `/` then Clean, which collapses any leading `..` to
//     the root and lets us strip it back off as a relative segment.
//  2. After Join, take Abs of both sides and verify the result is still
//     prefixed by the cache root. The trailing separator guard prevents
//     a `/home/u/.argus/notebooks-evil` from passing as if it were
//     under `/home/u/.argus/notebooks`.
//
// Returns the absolute cache path on success, or an error suitable to
// bubble straight back to the frontend.
func (st *Store) resolveLocal(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("invalid path: contains NUL")
	}
	// Absolute paths are always wrong here — even if they happen to
	// re-root inside the cache (e.g. "/etc/passwd" → "cacheDir/etc/passwd"
	// which is safe-but-confusing), the frontend should never have
	// authored one, so reject loudly instead of silently re-rooting.
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return "", fmt.Errorf("invalid path: absolute paths not allowed")
	}
	// Reject any literal ".." segment in the input even though the
	// Clean-with-leading-slash trick below would contain it inside the
	// cache root. A frontend supplying "foo/../../bad" is signaling
	// malicious or buggy intent; safer to fail loud than silently
	// rewrite to "bad" inside the cache.
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid path: contains .. segment")
		}
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+path), "/")
	full := filepath.Join(st.cacheDir, rel)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	baseAbs, err := filepath.Abs(st.cacheDir)
	if err != nil {
		return "", fmt.Errorf("resolve cache root: %w", err)
	}
	sep := string(os.PathSeparator)
	if abs != baseAbs && !strings.HasPrefix(abs+sep, baseAbs+sep) {
		return "", fmt.Errorf("path escapes cache root")
	}
	return abs, nil
}

// resolveKey validates a logical path before it's used as an S3 object
// key. Leading slashes, backslashes (Windows-style), `..` segments, and
// NULs are rejected. We accept the same set of paths resolveLocal
// would, expressed as forward-slash keys (no leading slash).
func (st *Store) resolveKey(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.ContainsRune(path, '\x00') || strings.ContainsRune(path, '\\') {
		return "", fmt.Errorf("invalid path: contains NUL or backslash")
	}
	cleaned := strings.TrimPrefix(filepath.Clean("/"+path), "/")
	// After Clean+TrimPrefix, any remaining `..` segment means traversal.
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path escapes root")
		}
	}
	return cleaned, nil
}

// New creates a notebook store with optional S3 support.
func New(cfg *appconfig.OnlineDataConfig, logger *slog.Logger) (*Store, error) {
	uploadCtx, uploadCancel := context.WithCancel(context.Background())
	store := &Store{
		logger:       logger,
		cacheDir:     filepath.Join(os.ExpandEnv("$HOME"), ".argus", "notebooks"),
		configured:   false,
		uploadCtx:    uploadCtx,
		uploadCancel: uploadCancel,
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(store.cacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Only configure S3 if credentials are provided
	if cfg.S3.Bucket == "" {
		logger.Info("S3 notebooks disabled — no bucket configured")
		return store, nil
	}

	if cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
		logger.Warn("S3 notebooks disabled — missing credentials")
		return store, nil
	}

	// Initialize AWS SDK v2 client
	cfg2, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		),
		config.WithRegion(cfg.S3.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Set custom endpoint if provided (for S3-compatible services)
	if cfg.S3.Endpoint != "" {
		cfg2.BaseEndpoint = aws.String(cfg.S3.Endpoint)
	}

	store.s3Client = s3.NewFromConfig(cfg2)
	store.bucket = cfg.S3.Bucket
	store.configured = true

	logger.Info("S3 notebooks enabled",
		slog.String("bucket", store.bucket),
		slog.String("region", cfg.S3.Region),
	)

	return store, nil
}

// GetCacheDir returns the local cache directory path.
func (st *Store) GetCacheDir() string {
	return st.cacheDir
}

// IsConfigured returns whether S3 is configured.
func (st *Store) IsConfigured() bool {
	return st.configured
}

// ListFiles returns all markdown files organized in a tree structure.
// When S3 is configured, lists from S3. Otherwise, lists from local cache.
func (st *Store) ListFiles(ctx context.Context) ([]FileEntry, error) {
	if st.configured {
		return st.listFilesS3(ctx)
	}
	return st.listFilesLocal()
}

// listFilesLocal walks the local cache directory for .md files.
func (st *Store) listFilesLocal() ([]FileEntry, error) {
	files := make(map[string]*FileEntry)
	folders := make(map[string]*FileEntry)

	err := filepath.Walk(st.cacheDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip broken entries
		}
		relPath, _ := filepath.Rel(st.cacheDir, absPath)
		if relPath == "." {
			return nil
		}

		// Normalize to forward-slash keys (same as S3).
		key := filepath.ToSlash(relPath)

		if info.IsDir() {
			folders[key] = &FileEntry{
				ID:       key,
				Name:     info.Name(),
				Path:     key,
				Type:     "folder",
				Children: []FileEntry{},
			}
			return nil
		}

		if !strings.HasSuffix(key, ".md") {
			return nil
		}

		files[key] = &FileEntry{
			ID:       strings.TrimSuffix(key, ".md"),
			Name:     info.Name(),
			Path:     key,
			Type:     "file",
			Modified: info.ModTime(),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk cache directory: %w", err)
	}

	return buildTree(files, folders), nil
}

// listFilesS3 lists markdown files from the S3 bucket.
func (st *Store) listFilesS3(ctx context.Context) ([]FileEntry, error) {
	files := make(map[string]*FileEntry)
	folders := make(map[string]*FileEntry)

	paginator := s3.NewListObjectsV2Paginator(st.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(st.bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := *obj.Key
			if !strings.HasSuffix(key, ".md") {
				continue
			}

			parts := strings.Split(key, "/")
			name := parts[len(parts)-1]
			id := strings.TrimSuffix(key, ".md")

			fe := &FileEntry{
				ID:       id,
				Name:     name,
				Path:     key,
				Type:     "file",
				Modified: *obj.LastModified,
			}
			files[key] = fe

			for i := 0; i < len(parts)-1; i++ {
				folderPath := strings.Join(parts[:i+1], "/")
				if _, exists := folders[folderPath]; !exists {
					folders[folderPath] = &FileEntry{
						ID:       folderPath,
						Name:     parts[i],
						Path:     folderPath,
						Type:     "folder",
						Children: []FileEntry{},
					}
				}
			}
		}
	}

	return buildTree(files, folders), nil
}

// GetFile retrieves file content, checking local cache first.
func (st *Store) GetFile(ctx context.Context, path string) (string, error) {
	cachePath, err := st.resolveLocal(path)
	if err != nil {
		return "", err
	}
	key, err := st.resolveKey(path)
	if err != nil {
		return "", err
	}
	if data, err := os.ReadFile(cachePath); err == nil {
		st.logger.DebugContext(ctx, "retrieved file from cache", slog.String("path", path))
		return string(data), nil
	}

	// In local-only mode, cache miss means file doesn't exist.
	if !st.configured {
		return "", fmt.Errorf("file not found: %s", path)
	}

	result, err := st.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read S3 object: %w", err)
	}

	// Cache locally
	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		st.logger.WarnContext(ctx, "failed to create cache directory", slog.String("error", err.Error()))
	} else if err := os.WriteFile(cachePath, data, 0640); err != nil {
		st.logger.WarnContext(ctx, "failed to write to cache", slog.String("error", err.Error()))
	}

	return string(data), nil
}

// SaveFile writes to local cache immediately and uploads to S3 asynchronously.
func (st *Store) SaveFile(ctx context.Context, path, content string) error {
	// Ensure path has .md extension
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}
	cachePath, err := st.resolveLocal(path)
	if err != nil {
		return err
	}
	key, err := st.resolveKey(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.WriteFile(cachePath, []byte(content), 0640); err != nil {
		return fmt.Errorf("failed to write to cache: %w", err)
	}

	// Upload to S3 asynchronously if configured. Use the store-scoped
	// uploadCtx (NOT the caller's ctx or context.Background()) so the
	// upload outlives the RPC but is still cancellable by store.Close()
	// when the process shuts down — otherwise pending uploads would
	// either abort mid-PUT or leak past shutdown.
	if st.configured {
		go func() {
			_, err := st.s3Client.PutObject(st.uploadCtx, &s3.PutObjectInput{
				Bucket:      aws.String(st.bucket),
				Key:         aws.String(key),
				Body:        bytes.NewReader([]byte(content)),
				ContentType: aws.String("text/markdown"),
			})
			if err != nil {
				st.logger.Warn("failed to upload file to S3",
					slog.String("path", path),
					slog.String("error", err.Error()),
				)
			} else {
				st.logger.DebugContext(st.uploadCtx, "uploaded file to S3", slog.String("path", path))
			}
		}()
	}

	return nil
}

// DeleteFile removes a file from both S3 and local cache.
func (st *Store) DeleteFile(ctx context.Context, path string) error {
	cachePath, err := st.resolveLocal(path)
	if err != nil {
		return err
	}
	key, err := st.resolveKey(path)
	if err != nil {
		return err
	}
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	// Delete from S3 if configured
	if st.configured {
		_, err := st.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to delete from S3: %w", err)
		}
	}

	return nil
}

// CreateFolder creates a folder marker in S3 (empty object with trailing slash).
func (st *Store) CreateFolder(ctx context.Context, path string) error {
	// Ensure path ends with /
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	cachePath, err := st.resolveLocal(path)
	if err != nil {
		return err
	}
	key, err := st.resolveKey(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cachePath, 0750); err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	// Create marker in S3 if configured
	if st.configured {
		// Restore trailing slash that filepath.Clean stripped.
		if !strings.HasSuffix(key, "/") {
			key += "/"
		}
		_, err := st.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte("")),
		})
		if err != nil {
			return fmt.Errorf("failed to create folder in S3: %w", err)
		}
	}

	return nil
}

// TestConnection verifies S3 credentials and bucket access.
func (st *Store) TestConnection(ctx context.Context) error {
	if !st.configured {
		return fmt.Errorf("S3 not configured")
	}

	// Try to list one object to verify credentials
	result, err := st.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(st.bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("S3 connection failed: %w", err)
	}

	if result == nil {
		return fmt.Errorf("S3 returned empty response")
	}

	return nil
}

// SyncAll pulls all files from S3 to local cache.
func (st *Store) SyncAll(ctx context.Context) error {
	if !st.configured {
		st.logger.Info("S3 sync skipped — not configured")
		return nil
	}

	paginator := s3.NewListObjectsV2Paginator(st.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(st.bucket),
	})

	count := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list S3 objects during sync: %w", err)
		}

		for _, obj := range page.Contents {
			key := *obj.Key

			// Skip folder markers
			if strings.HasSuffix(key, "/") {
				continue
			}

			result, err := st.s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(st.bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				st.logger.WarnContext(ctx, "failed to sync file", slog.String("key", key), slog.String("error", err.Error()))
				continue
			}

			data, err := io.ReadAll(result.Body)
			result.Body.Close()
			if err != nil {
				st.logger.WarnContext(ctx, "failed to read synced file", slog.String("key", key), slog.String("error", err.Error()))
				continue
			}

			// S3 keys are server-supplied but a tampered/shared bucket
			// could include "../" segments that would write outside the
			// cache. resolveLocal applies the same traversal check we use
			// for frontend-supplied paths.
			cachePath, err := st.resolveLocal(key)
			if err != nil {
				st.logger.WarnContext(ctx, "skipping S3 key that escapes cache root", slog.String("key", key), slog.String("error", err.Error()))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
				st.logger.WarnContext(ctx, "failed to create cache dir during sync", slog.String("error", err.Error()))
				continue
			}
			if err := os.WriteFile(cachePath, data, 0640); err != nil {
				st.logger.WarnContext(ctx, "failed to write cache during sync", slog.String("error", err.Error()))
				continue
			}

			count++
		}
	}

	st.logger.InfoContext(ctx, "S3 sync completed", slog.Int("files", count))
	return nil
}

// buildTree constructs the tree structure from flat file/folder maps.
func buildTree(files map[string]*FileEntry, folders map[string]*FileEntry) []FileEntry {
	// First pass: assign files to folders
	for filePath, file := range files {
		parts := strings.Split(filePath, "/")
		if len(parts) > 1 {
			parentPath := strings.Join(parts[:len(parts)-1], "/")
			if parent, exists := folders[parentPath]; exists {
				parent.Children = append(parent.Children, *file)
			}
		}
	}

	// Assign folders to their parents
	for folderPath, folder := range folders {
		parts := strings.Split(folderPath, "/")
		if len(parts) > 1 {
			parentPath := strings.Join(parts[:len(parts)-1], "/")
			if parent, exists := folders[parentPath]; exists {
				parent.Children = append(parent.Children, *folder)
			}
		}
	}

	// Root-level entries (no parent)
	var roots []FileEntry
	for _, file := range files {
		if !strings.Contains(file.Path, "/") {
			roots = append(roots, *file)
		}
	}
	for _, folder := range folders {
		if !strings.Contains(folder.Path, "/") {
			roots = append(roots, *folder)
		}
	}

	// Sort by name
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Type != roots[j].Type {
			return roots[i].Type == "folder" // folders first
		}
		return roots[i].Name < roots[j].Name
	})

	// Sort children recursively
	sortChildren(roots)

	return roots
}

func sortChildren(entries []FileEntry) {
	for i := range entries {
		if entries[i].Children != nil {
			sort.Slice(entries[i].Children, func(a, b int) bool {
				if entries[i].Children[a].Type != entries[i].Children[b].Type {
					return entries[i].Children[a].Type == "folder"
				}
				return entries[i].Children[a].Name < entries[i].Children[b].Name
			})
			sortChildren(entries[i].Children)
		}
	}
}
