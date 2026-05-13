package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type RegistryTag struct {
	Tag      string `json:"tag"`
	Size     int64  `json:"size,omitempty"`
	PushedAt string `json:"pushedAt,omitempty"`
}

type RegistryClient interface {
	ListTags(ctx context.Context, image string) ([]RegistryTag, error)
}

type dockerHubClient struct {
	http   *http.Client
	logger *slog.Logger
	cache  *tagCache
}

type ecrClient struct {
	http   *http.Client
	logger *slog.Logger
	cache  *tagCache
}

type gcrClient struct {
	http   *http.Client
	logger *slog.Logger
	cache  *tagCache
}

type tagCacheEntry struct {
	tags  []RegistryTag
	expry time.Time
}

type tagCache struct {
	mu    sync.RWMutex
	store map[string]*tagCacheEntry
	ttl   time.Duration
}

func newTagCache(ttl time.Duration) *tagCache {
	return &tagCache{
		store: make(map[string]*tagCacheEntry),
		ttl:   ttl,
	}
}

func (c *tagCache) get(key string) ([]RegistryTag, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	if !ok || time.Now().After(e.expry) {
		return nil, false
	}
	out := make([]RegistryTag, len(e.tags))
	copy(out, e.tags)
	return out, true
}

func (c *tagCache) set(key string, tags []RegistryTag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = &tagCacheEntry{
		tags:  tags,
		expry: time.Now().Add(c.ttl),
	}
}

type dockerHubResponse struct {
	Results []struct {
		Name       string `json:"name"`
		FullSize   int64  `json:"full_size"`
		LastUpated string `json:"last_updated"`
	} `json:"results"`
}

type gcrTagsResponse struct {
	Child  []string `json:"child"`
	Manifest map[string]struct {
		Tags      []string `json:"tags"`
		TimeUploaded string `json:"timeUploadedMs"`
	} `json:"manifest"`
}

func newDockerHubClient(logger *slog.Logger) *dockerHubClient {
	return &dockerHubClient{
		http:   &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		cache:  newTagCache(5 * time.Minute),
	}
}

func newECRClient(logger *slog.Logger) *ecrClient {
	return &ecrClient{
		http:   &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		cache:  newTagCache(5 * time.Minute),
	}
}

func newGCRClient(logger *slog.Logger) *gcrClient {
	return &gcrClient{
		http:   &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		cache:  newTagCache(5 * time.Minute),
	}
}

func (c *dockerHubClient) ListTags(ctx context.Context, image string) ([]RegistryTag, error) {
	if cached, ok := c.cache.get(image); ok {
		return cached, nil
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=25", image)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("docker hub request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker hub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker hub: status %d", resp.StatusCode)
	}

	var dhResp dockerHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&dhResp); err != nil {
		return nil, fmt.Errorf("docker hub decode: %w", err)
	}

	tags := make([]RegistryTag, 0, len(dhResp.Results))
	for _, r := range dhResp.Results {
		t := RegistryTag{Tag: r.Name, Size: r.FullSize, PushedAt: r.LastUpated}
		if t.Tag == "" {
			continue
		}
		tags = append(tags, t)
	}

	c.cache.set(image, tags)
	return tags, nil
}

func (c *ecrClient) ListTags(ctx context.Context, image string) ([]RegistryTag, error) {
	if cached, ok := c.cache.get(image); ok {
		return cached, nil
	}

	// For ECR, the image format is: <account>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>
	// We extract the registry and repo from the image name.
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", extractRegistry(image), extractRepo(image))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ecr request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ecr: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecr: status %d", resp.StatusCode)
	}

	var ecrResp struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ecrResp); err != nil {
		return nil, fmt.Errorf("ecr decode: %w", err)
	}

	tags := make([]RegistryTag, 0, len(ecrResp.Tags))
	for _, t := range ecrResp.Tags {
		tags = append(tags, RegistryTag{Tag: t})
	}

	c.cache.set(image, tags)
	return tags, nil
}

func (c *gcrClient) ListTags(ctx context.Context, image string) ([]RegistryTag, error) {
	if cached, ok := c.cache.get(image); ok {
		return cached, nil
	}

	// GCR image format: <location>.gcr.io/<project>/<repo> or gcr.io/<project>/<repo>
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", extractRegistry(image), extractRepo(image))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gcr request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcr: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcr: status %d", resp.StatusCode)
	}

	var gcrResp struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gcrResp); err != nil {
		return nil, fmt.Errorf("gcr decode: %w", err)
	}

	tags := make([]RegistryTag, 0, len(gcrResp.Tags))
	for _, t := range gcrResp.Tags {
		tags = append(tags, RegistryTag{Tag: t})
	}

	c.cache.set(image, tags)
	return tags, nil
}

func extractRegistry(image string) string {
	for i := 0; i < len(image); i++ {
		if image[i] == '/' {
			return image[:i]
		}
	}
	return ""
}

func extractRepo(image string) string {
	for i := 0; i < len(image); i++ {
		if image[i] == '/' {
			repo := image[i+1:]
			// Strip tag if present.
			for j := 0; j < len(repo); j++ {
				if repo[j] == ':' {
					return repo[:j]
				}
			}
			return repo
		}
	}
	return image
}

// NewRegistryClient returns the appropriate registry client based on the image registry.
func NewRegistryClient(image string, logger *slog.Logger) RegistryClient {
	switch {
	case containsSubstr(image, "docker.io"), containsSubstr(image, "dockerhub"):
		return newDockerHubClient(logger)
	case containsSubstr(image, "amazonaws.com"), containsSubstr(image, "ecr"):
		return newECRClient(logger)
	case containsSubstr(image, "gcr.io"), containsSubstr(image, "pkg.dev"):
		return newGCRClient(logger)
	default:
		return newDockerHubClient(logger)
	}
}

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
