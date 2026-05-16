package pkg

// Canary test for CR P3.19. Every exported method on *App is reachable
// via the HTTP reflector (server.go's ServeHTTP). The allowlist
// `httpExposedMethods` decides what's actually callable from SaaS;
// methods not in the allowlist are 403'd. The risk this test covers:
// someone adds a new method (likely a mutating one) and nobody puts
// it through the security-review allowlist decision — silently
// shipping a SaaS-callable action.
//
// This test enumerates every exported *App method via reflection and
// fails if any method is neither in the allowlist nor in the explicit
// `acknowledgedDenied` set below. New methods land with a one-line
// addition to one of the two sets and the rationale in code review.
//
// The test does NOT validate whether the allowlist decision is correct
// — only that A decision was made. That's the right granularity for a
// CI gate: humans review the routing; CI catches the omission.

import (
	"reflect"
	"strings"
	"testing"
)

// acknowledgedDenied is the explicit "we know about this method and
// deliberately keep it Wails-only" list. Keep alphabetical so PRs
// adding rows don't merge-conflict on the order.
//
// Anything in this set is INTENTIONALLY NOT in httpExposedMethods.
// Examples already-classified:
//   * DeletePod / DeleteResource / ApplyYaml / DeleteSecret —
//     cluster mutations
//   * StartTerminal / StartTerminalSession / LaunchPopOutTerminal —
//     spawn arbitrary processes
//   * UpdateSettings / SaveAuthCredentials — write user state
//   * SwitchContext — mutates the loaded kubeconfig
//
// The canary doesn't care WHY a method is denied — only that the
// author of the method made a deliberate "not for SaaS" call. The
// rationale belongs in the surrounding code review.
var acknowledgedDenied = map[string]struct{}{
	"ApplyYaml":                     {},
	"DeletePod":                     {},
	"DeleteResource":                {},
	"DeleteSecret":                  {},
	"ExecInPod":                     {},
	"ExplainTerminalOutput":         {},
	"GenerateCommand":               {},
	"LaunchPopOutTerminal":          {},
	"PatchResource":                 {},
	"RestartDeployment":             {},
	"SaveAuthCredentials":           {},
	"StartTerminal":                 {},
	"StartTerminalSession":          {},
	"StopTerminalSession":           {},
	"SwitchContext":                 {},
	"UpdateSettings":                {},
	"UpgradeAlertSeverity":          {},
	"WriteTerminalInput":            {},
}

// implementationExempt lists method names that aren't user-callable
// surface — they're either lifecycle hooks Wails invokes itself, Go-
// internal hooks, or interface implementations the reflector never
// dispatches to. Keeping them here prevents false positives.
var implementationExempt = map[string]struct{}{
	// Wails lifecycle.
	"Startup":  {},
	"Shutdown": {},
	"DOMReady": {},

	// http.Handler — used by the reflector itself, not callable.
	"ServeHTTP": {},

	// Bare emit helper used as a Wails binding signal.
	"EmitLogLine": {},
}

func TestExportedAppMethodsAreClassified(t *testing.T) {
	// Don't t.Parallel: this test is pure reflection; parallelism
	// would be noise.
	appType := reflect.TypeOf((*App)(nil))
	if appType.Kind() != reflect.Ptr {
		t.Fatalf("expected *App reflect.Ptr, got %v", appType.Kind())
	}

	missing := []string{}

	for i := 0; i < appType.NumMethod(); i++ {
		m := appType.Method(i)
		name := m.Name

		// Skip un-exported methods (reflection only sees exported on
		// *App anyway, but make the intent explicit).
		if name == "" || !isExportedIdent(name) {
			continue
		}

		// Skip implementation-level exemptions.
		if _, ok := implementationExempt[name]; ok {
			continue
		}

		// A method must be in EXACTLY one of the two sets — neither
		// (no decision made) is a failure; both is a config drift
		// failure that we want to flag too.
		_, allowed := httpExposedMethods[name]
		_, denied := acknowledgedDenied[name]

		if !allowed && !denied {
			missing = append(missing, name)
		}
		if allowed && denied {
			t.Errorf("method %q is in BOTH httpExposedMethods and acknowledgedDenied; pick one", name)
		}
	}

	if len(missing) > 0 {
		// PHASE 1 of this canary: report-only. The baseline at first
		// landing is ~130 unclassified methods; failing the test
		// immediately would turn CI red on every PR until the entire
		// surface is triaged. Instead, this prints the list to the
		// test log so a reviewer can see what's left. Subsequent PRs
		// drain it row-by-row and the eventual flip to t.Fatalf is a
		// one-line change.
		//
		// Each row should be classified as ONE of:
		//   - SaaS-callable → add to httpExposedMethods (server.go)
		//   - Wails-only    → add to acknowledgedDenied (this file)
		t.Logf(
			"\n%d *App method(s) have no allowlist decision yet:\n  %s\n"+
				"Flip this t.Logf to t.Fatalf once the list is empty to lock in the canary.",
			len(missing),
			strings.Join(missing, "\n  "),
		)
	}
}

func isExportedIdent(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return c >= 'A' && c <= 'Z'
}

// TestAcknowledgedDeniedNotInAllowlist is a sanity check on the two
// sets — they must be disjoint. Caught here in addition to the
// per-method check above so the diagnosis is clear when the canary
// fires (mis-classification vs. missing classification).
func TestAcknowledgedDeniedNotInAllowlist(t *testing.T) {
	for name := range acknowledgedDenied {
		if _, also := httpExposedMethods[name]; also {
			t.Errorf("method %q is in both acknowledgedDenied AND httpExposedMethods", name)
		}
	}
}
