package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub Pull Requests + Branches: thin REST wrappers around api.github.com.
// We hit the public REST API directly (no SDK) so the user sees a real,
// observable network round-trip when they click into Pipelines → PRs/Branches
// — that's what makes "github is not working" actionable: the call now
// either succeeds with real data or fails with a real error code we can
// surface in the UI.

const (
	githubAPIBase    = "https://api.github.com"
	githubAPIVersion = "2022-11-28"
	githubFetchHTTPTimeout = 12 * time.Second
)

// GitHubPullRequest is the slimmed-down PR shape we hand to the frontend.
// We deliberately don't expose the full GitHub payload — keeps the binding
// surface small and the JSON predictable.
type GitHubPullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Author  string `json:"author"`
	Branch  string `json:"branch"`
	Base    string `json:"base"`
	URL     string `json:"url"`
	Draft   bool   `json:"draft"`
	UpdatedAt string `json:"updatedAt"`
}

// GitHubBranch is the slimmed branch shape.
type GitHubBranch struct {
	Name      string `json:"name"`
	Sha       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// GitHubError gives the frontend enough information to render the right
// follow-up. ConfigMissing means the user hasn't filled the owner/repo/token
// triplet in Settings (so the UI can render a "Configure GitHub" button
// pointing at Settings); AuthFailed means the credentials were rejected by
// GitHub itself; Other is everything else (network, rate limit, repo gone).
type GitHubErrorKind string

const (
	GitHubErrConfigMissing GitHubErrorKind = "config_missing"
	GitHubErrAuthFailed    GitHubErrorKind = "auth_failed"
	GitHubErrNotFound      GitHubErrorKind = "not_found"
	GitHubErrRateLimited   GitHubErrorKind = "rate_limited"
	GitHubErrOther         GitHubErrorKind = "other"
)

// configError lets us bubble up a typed kind without leaking it into the
// generic error chain.
type configError struct {
	Kind GitHubErrorKind
	Msg  string
}

func (e *configError) Error() string { return e.Msg }

// ListGitHubPullRequests fetches open PRs for the configured owner/repo.
// The Wails-level error string gets a `[<kind>] ` prefix so the frontend
// can switch on it without parsing the human-readable message.
func (a *App) ListGitHubPullRequests() ([]GitHubPullRequest, error) {
	owner, repo, token, err := a.githubCreds()
	if err != nil {
		return nil, kindedError(err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=50",
		githubAPIBase, url.PathEscape(owner), url.PathEscape(repo))

	a.logger.InfoContext(a.ctx, "github: fetching pull requests",
		slog.String("owner", owner), slog.String("repo", repo))

	body, err := a.githubGET(a.ctx, endpoint, token)
	if err != nil {
		return nil, kindedError(err)
	}

	// Decode into a permissive intermediate that mirrors GitHub's actual
	// payload only as far as we need it.
	type ghUser struct{ Login string `json:"login"` }
	type ghBranch struct{ Ref string `json:"ref"` }
	type ghPR struct {
		Number    int      `json:"number"`
		Title     string   `json:"title"`
		State     string   `json:"state"`
		Draft     bool     `json:"draft"`
		User      ghUser   `json:"user"`
		Head      ghBranch `json:"head"`
		Base      ghBranch `json:"base"`
		HTMLURL   string   `json:"html_url"`
		UpdatedAt string   `json:"updated_at"`
	}
	var raw []ghPR
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, kindedError(&configError{Kind: GitHubErrOther,
			Msg: fmt.Sprintf("decode pull-requests response: %v", err)})
	}

	out := make([]GitHubPullRequest, 0, len(raw))
	for _, p := range raw {
		out = append(out, GitHubPullRequest{
			Number:    p.Number,
			Title:     p.Title,
			State:     p.State,
			Draft:     p.Draft,
			Author:    p.User.Login,
			Branch:    p.Head.Ref,
			Base:      p.Base.Ref,
			URL:       p.HTMLURL,
			UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

// ListGitHubBranches fetches branches for the configured owner/repo.
func (a *App) ListGitHubBranches() ([]GitHubBranch, error) {
	owner, repo, token, err := a.githubCreds()
	if err != nil {
		return nil, kindedError(err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100",
		githubAPIBase, url.PathEscape(owner), url.PathEscape(repo))

	a.logger.InfoContext(a.ctx, "github: fetching branches",
		slog.String("owner", owner), slog.String("repo", repo))

	body, err := a.githubGET(a.ctx, endpoint, token)
	if err != nil {
		return nil, kindedError(err)
	}

	type ghCommit struct{ Sha string `json:"sha"` }
	type ghBranch struct {
		Name      string   `json:"name"`
		Commit    ghCommit `json:"commit"`
		Protected bool     `json:"protected"`
	}
	var raw []ghBranch
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, kindedError(&configError{Kind: GitHubErrOther,
			Msg: fmt.Sprintf("decode branches response: %v", err)})
	}

	out := make([]GitHubBranch, 0, len(raw))
	for _, b := range raw {
		out = append(out, GitHubBranch{
			Name:      b.Name,
			Sha:       b.Commit.Sha,
			Protected: b.Protected,
		})
	}
	return out, nil
}

// --- internals --------------------------------------------------------------

func (a *App) githubCreds() (owner, repo, token string, err error) {
	if a.cfg == nil {
		return "", "", "", &configError{Kind: GitHubErrConfigMissing,
			Msg: "no configuration loaded"}
	}
	owner = strings.TrimSpace(a.cfg.Pipelines.GitHubOwner)
	repo = strings.TrimSpace(a.cfg.Pipelines.GitHubRepo)
	token = strings.TrimSpace(a.cfg.Pipelines.GitHubToken)
	missing := []string{}
	if owner == "" {
		missing = append(missing, "owner")
	}
	if repo == "" {
		missing = append(missing, "repository")
	}
	if token == "" {
		missing = append(missing, "token")
	}
	if len(missing) > 0 {
		return "", "", "", &configError{
			Kind: GitHubErrConfigMissing,
			Msg:  "GitHub config incomplete: missing " + strings.Join(missing, ", "),
		}
	}
	return owner, repo, token, nil
}

func (a *App) githubGET(ctx context.Context, endpoint, token string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, githubFetchHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &configError{Kind: GitHubErrOther, Msg: err.Error()}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "argus")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &configError{Kind: GitHubErrOther,
			Msg: "github request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return body, nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		// 403 from GitHub also covers rate limit; we distinguish via the
		// X-RateLimit-Remaining header (zero ⇒ rate limited).
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining == "0" {
			return nil, &configError{Kind: GitHubErrRateLimited,
				Msg: "github rate limit exceeded — retry after " + resp.Header.Get("X-RateLimit-Reset")}
		}
		return nil, &configError{Kind: GitHubErrAuthFailed,
			Msg: fmt.Sprintf("github rejected the token (HTTP %d): %s", resp.StatusCode, snippet(body))}
	case http.StatusNotFound:
		return nil, &configError{Kind: GitHubErrNotFound,
			Msg: "github says the repo or owner does not exist (HTTP 404)"}
	default:
		return nil, &configError{Kind: GitHubErrOther,
			Msg: fmt.Sprintf("github returned %d: %s", resp.StatusCode, snippet(body))}
	}
}

func snippet(b []byte) string {
	const max = 240
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

// kindedError wraps a *configError so the Wails string carries a parseable
// `[<kind>] ` prefix the frontend uses to switch on. Other errors fall
// through with the "other" kind.
func kindedError(err error) error {
	if err == nil {
		return nil
	}
	var ce *configError
	if errors.As(err, &ce) {
		return fmt.Errorf("[%s] %s", string(ce.Kind), ce.Msg)
	}
	return fmt.Errorf("[%s] %s", string(GitHubErrOther), err.Error())
}
