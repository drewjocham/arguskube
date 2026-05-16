package pkg

import (
	"log/slog"
	"net/http"
	"strings"

	gochi "github.com/go-chi/chi/v5"

	"github.com/argues/argus/internal/workspace"
)

const workspacePathOAuthCallback = "/workspace/oauth/callback"

// workspace_handler.go — the OAuth callback page for workspace
// providers. Mirrors the auth /auth/*/callback pattern: a tiny HTML
// page the user's browser lands on after the upstream provider's
// consent flow. The page posts the state + code back to the
// frontend's opener window (the Argus desktop UI) so the bridge can
// call CompleteWorkspaceConnect via Wails.
//
// Why postMessage rather than server-side Complete + Poll? The
// workspace store already implements postMessage; the OIDC pattern's
// Poll endpoint can be added later if we want browser-window-less
// flows. The state nonce is server-issued (Manager.Start) so the
// callback can't be forged by an unrelated browser.

// WorkspaceRoutes builds the /workspace/* router and returns it as a
// chi sub-tree the parent server mounts. Matches the pim-agl
// "Routes() http.Handler" composition pattern.
func (a *App) WorkspaceRoutes() http.Handler {
	r := gochi.NewRouter()
	r.Get(workspacePathOAuthCallback, a.handleWorkspaceCallback)
	return r
}

func (a *App) handleWorkspaceCallback(w http.ResponseWriter, r *http.Request) {
	if !a.workspaceAvailable() {
		a.logger.Warn("workspace callback: integrations disabled in this build mode")
		a.renderWorkspaceError(w, "", "workspace integrations are disabled in this build mode")
		return
	}
	q := r.URL.Query()
	if errCode := q.Get("error"); errCode != "" {
		// Slack / Google can append `error=access_denied&error_description=...`
		// when the user declines consent. Surface a friendly message AND
		// postMessage the failure back to the opener so the Argus UI
		// updates immediately — previously the popup just rendered the
		// error and the parent window timed out without ever learning
		// why.
		desc := q.Get("error_description")
		if desc == "" {
			desc = errCode
		}
		a.logger.Warn("workspace callback: upstream provider rejected the consent",
			slog.String("error", errCode),
			slog.String("description", desc),
		)
		a.renderWorkspaceError(w, "", desc)
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		a.logger.Warn("workspace callback: missing code or state",
			slog.Bool("has_state", state != ""),
			slog.Bool("has_code", code != ""),
		)
		a.renderWorkspaceError(w, "", "missing code or state in callback")
		return
	}

	svc, err := a.workspace.LookupPendingService(state)
	if err != nil {
		a.logger.Warn("workspace callback: unknown or expired state",
			slog.String("state_prefix", redactState(state)),
			slog.String("error", err.Error()),
		)
		a.renderWorkspaceError(w, "", "unknown or expired connection attempt — restart from Argus")
		return
	}

	// Render a self-closing page that posts the trio back to the
	// opener (the Argus app). We escape the values to keep an XSS
	// vector closed — `state` is server-issued but `code` is from the
	// browser's query string and the upstream provider could in
	// principle echo arbitrary text.
	page := strings.ReplaceAll(workspaceCallbackTemplate, "{{SERVICE}}", htmlEscape(string(svc)))
	page = strings.ReplaceAll(page, "{{STATE}}", htmlEscape(state))
	page = strings.ReplaceAll(page, "{{CODE}}", htmlEscape(code))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}

// workspaceCallbackTemplate posts the trio back through window.opener
// then closes itself. If the opener is gone (user closed the Argus
// window first) the page just shows a static message.
//
// The targetOrigin is intentionally "*" — the desktop's webview origin
// is non-deterministic (file:// for the bundled build, http://localhost
// during `wails dev`) and the state nonce is the integrity check. A
// cross-origin attacker who reads this postMessage payload still can't
// complete the OAuth flow without also controlling the manager's
// in-memory state map (which they can't).
const workspaceCallbackTemplate = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<title>Argus — Workspace connect</title>
<style>
body { font-family: -apple-system, system-ui, sans-serif; background: #0f172a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
.card { max-width: 28rem; padding: 2rem; border: 1px solid #334155; border-radius: 12px; background: #1e293b; text-align: center; }
h1 { margin: 0 0 0.5rem; font-size: 1.1rem; }
p { margin: 0; color: #94a3b8; font-size: 0.9rem; }
</style>
</head><body>
<div class="card">
  <h1>Connecting to {{SERVICE}}…</h1>
  <p>You can close this tab and return to Argus.</p>
</div>
<script>
(function () {
  var payload = {
    type: 'workspace:complete',
    service: '{{SERVICE}}',
    state: '{{STATE}}',
    code: '{{CODE}}'
  };
  try {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, '*');
    }
  } catch (e) {}
  setTimeout(function () { window.close(); }, 600);
})();
</script>
</body></html>`

// htmlEscape is the same shape auth.htmlEscape uses; we duplicate
// the four-line helper rather than export the internal one because
// it's tiny and the workspace handler shouldn't reach into auth's
// internals for a trivial utility.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// redactState returns the first 8 characters of the OAuth state
// nonce for logging. The full state is a secret the upstream provider
// echoes back — we want to identify which attempt failed without
// putting the whole token in logs.
func redactState(state string) string {
	if len(state) <= 8 {
		return state + "…"
	}
	return state[:8] + "…"
}

// renderWorkspaceError renders the SAME-shaped callback page as the
// happy path, but with type: 'workspace:error' so the parent window
// learns about the failure synchronously. Previously the error page
// rendered a static "Sign-in failed" body and the parent window had
// no way to know why — it just polled until the popup closed and
// guessed "Connection canceled". That misled users who had actually
// hit an upstream consent rejection, a misconfigured redirect URI,
// or an expired state.
//
// The service field may be empty when the failure happened before we
// could resolve the state to a service (early validation paths).
func (a *App) renderWorkspaceError(w http.ResponseWriter, service, message string) {
	page := strings.ReplaceAll(workspaceErrorCallbackTemplate, "{{SERVICE}}", htmlEscape(service))
	page = strings.ReplaceAll(page, "{{MESSAGE}}", htmlEscape(message))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(page))
}

// workspaceErrorCallbackTemplate matches the success-path template's
// shape: same styling so the user gets visual continuity, plus the
// postMessage that's the actual bug fix. The parent window listens
// for either type ('workspace:complete' or 'workspace:error') and
// updates the UI accordingly.
const workspaceErrorCallbackTemplate = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<title>Argus — Workspace connect failed</title>
<style>
body { font-family: -apple-system, system-ui, sans-serif; background: #0f172a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
.card { max-width: 28rem; padding: 2rem; border: 1px solid #ef4444; border-radius: 12px; background: #1e293b; text-align: center; }
h1 { margin: 0 0 0.5rem; font-size: 1.1rem; color: #fca5a5; }
p { margin: 0; color: #94a3b8; font-size: 0.9rem; word-break: break-word; }
</style>
</head><body>
<div class="card">
  <h1>Connection failed</h1>
  <p>{{MESSAGE}}</p>
</div>
<script>
(function () {
  var payload = {
    type: 'workspace:error',
    service: '{{SERVICE}}',
    error: '{{MESSAGE}}'
  };
  try {
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(payload, '*');
    }
  } catch (e) {}
  setTimeout(function () { window.close(); }, 1800);
})();
</script>
</body></html>`

// Ensure workspace import is used even if all callers vanish; the
// LookupPendingService method is the only one we touch externally.
var _ = workspace.ErrNotFound
