package pkg

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace_slack.go — per-service Wails methods for the Slack
// adapter. Lives next to app_workspace.go so the Vue side has one
// callable surface per integration without bloating the foundation
// file.
//
// All methods require a valid sessionToken (resolved via the auth
// store) plus a connection ID. The token decryption lives behind
// workspace.Manager.Token; we never log or echo the bearer.

// SlackChannelView is the redacted shape the UI consumes. Identical
// to workspace.Channel but JSON-tagged with snake_case to match the
// other workspace views.
type SlackChannelView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// slackAdapter is shared across calls because the adapter holds only
// the HTTP client + base URL. Lazy-init guarded by sync.Once so a
// build that never connects Slack doesn't allocate one.
var (
	slackAdapterOnce sync.Once
	slackAdapter     *workspace.SlackAdapter
)

func getSlackAdapter() *workspace.SlackAdapter {
	slackAdapterOnce.Do(func() {
		slackAdapter = workspace.NewSlackAdapter()
	})
	return slackAdapter
}

// resolveSlackConnection finds + validates the caller's connection.
// Centralized so every Slack method has one place to reject the four
// failure modes (workspace off, bad session, unknown connection,
// wrong service).
func (a *App) resolveSlackConnection(sessionToken, connectionID string) (workspace.Token, error) {
	if !a.workspaceAvailable() {
		return workspace.Token{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return workspace.Token{}, err
	}
	conns, err := a.workspace.List(a.appCtx(), userID)
	if err != nil {
		return workspace.Token{}, err
	}
	for _, c := range conns {
		if c.ID == connectionID {
			if c.Service != workspace.ServiceSlack {
				return workspace.Token{}, fmt.Errorf("workspace: connection %q is %s, not slack", c.DisplayName, c.Service)
			}
			tok, err := a.workspace.Token(a.appCtx(), c.ID)
			if err != nil {
				return workspace.Token{}, fmt.Errorf("workspace: load token: %w", err)
			}
			return tok, nil
		}
	}
	return workspace.Token{}, errors.New("workspace: connection not found for this user")
}

// ListSlackChannels returns the non-archived channels the bot is in.
// Used by the SlackPanel's channel picker.
func (a *App) ListSlackChannels(sessionToken, connectionID string) ([]SlackChannelView, error) {
	tok, err := a.resolveSlackConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	channels, err := getSlackAdapter().ListChannels(a.appCtx(), tok)
	if err != nil {
		return nil, err
	}
	out := make([]SlackChannelView, 0, len(channels))
	for _, c := range channels {
		out = append(out, SlackChannelView{ID: c.ID, Name: c.Name})
	}
	return out, nil
}

// ListSlackEvents returns the bus's ring buffer (most recent first).
// SaaS-only — when the bus isn't wired (no signing secret) this
// returns an empty slice so the UI can render "events disabled" rather
// than 500. Wails-only: events contain channel names and user IDs we
// don't want to fan out over the SaaS HTTP shim.
func (a *App) ListSlackEvents(sessionToken string) ([]workspace.RecentEvent, error) {
	if a.slackEvents == nil {
		return []workspace.RecentEvent{}, nil
	}
	if _, err := a.workspaceUserID(sessionToken); err != nil {
		return nil, err
	}
	return a.slackEvents.RecentEvents(), nil
}

// SendSlackMessage posts plain text to a channel via chat.postMessage.
// Slack auto-renders mrkdwn so backticks/asterisks are formatted.
//
// Cap at 40 KB to dodge Slack's 40,000-character cap on body — text
// is intentionally truncated rather than failing the whole call so an
// agent rendering a verbose summary doesn't lose the message entirely.
func (a *App) SendSlackMessage(sessionToken, connectionID, channelID, text string) error {
	if strings.TrimSpace(channelID) == "" {
		return errors.New("slack: channel is required")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("slack: text is required")
	}
	tok, err := a.resolveSlackConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	if len(text) > 40000 {
		text = text[:40000-12] + "\n\n…truncated"
	}
	return getSlackAdapter().Send(a.appCtx(), tok, channelID, text)
}
