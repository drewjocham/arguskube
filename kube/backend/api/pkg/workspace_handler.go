package pkg

import (
	"net/http"
	"strings"

	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/workspace"
)

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

// WorkspaceRoutes registers the public callback endpoint on a mux.
// Called from main.go after the App is constructed.
func (a *App) WorkspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/workspace/oauth/callback", a.handleWorkspaceCallback)
	// Slack Events API + slash commands. Only registers when the bus
	// is wired (ARGUS_SLACK_SIGNING_SECRET set). When unwired the
	// route stays unregistered so a curious internet caller gets a
	// 404 rather than a misleading 401.
	if a.slackEvents != nil {
		mux.HandleFunc("/workspace/slack/events", a.handleSlackEvents)
		mux.HandleFunc("/workspace/slack/commands", a.handleSlackCommands)
	}
}

func (a *App) handleWorkspaceCallback(w http.ResponseWriter, r *http.Request) {
	if !a.workspaceAvailable() {
		auth.RenderCallback(w, false, "workspace integrations are disabled in this build mode")
		return
	}
	q := r.URL.Query()
	if errCode := q.Get("error"); errCode != "" {
		// Slack / Google can append `error=access_denied&error_description=...`
		// when the user declines consent. Surface a friendly message.
		desc := q.Get("error_description")
		if desc == "" {
			desc = errCode
		}
		auth.RenderCallback(w, false, desc)
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		auth.RenderCallback(w, false, "missing code or state in callback")
		return
	}

	svc, err := a.workspace.LookupPendingService(state)
	if err != nil {
		auth.RenderCallback(w, false, "unknown or expired connection attempt — restart from Argus")
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

// Ensure workspace import is used even if all callers vanish; the
// LookupPendingService method is the only one we touch externally.
var _ = workspace.ErrNotFound
