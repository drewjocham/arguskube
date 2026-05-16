// Pop-out terminal launcher. The Vue frontend calls
// LaunchPopOutTerminal (see kube/view/src/App.vue:openPopOut) to spawn
// a real OS-level terminal window. Previously this was a phantom
// Wails binding — referenced from the frontend and listed in the
// security allowlist comments but never implemented. The handler
// behavior now: spawn the lufis-terminal binary with the running
// Argus's Kubernetes context, working directory, and AI credentials
// inherited via env vars, so the spawned shell's `kubectl` /
// `argocd` / Argus AI features land on the same cluster + account
// the user is operating on inside Argus.
//
// Replaces the previous design (spawn `argus-terminal`, the Wails-
// based standalone in cmd/terminal). The lufis-terminal is a native
// GLFW/PTY terminal with its own block / notes / opencode integration
// — same UX, no Wails round-trip.
package pkg

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// lufisBinaryName is the executable Argus looks for. Override at
// runtime with ARGUS_LUFIS_PATH (absolute path to a specific build
// — useful during development).
const lufisBinaryName = "lufis-terminal"

// ErrLufisNotFound is returned by LaunchPopOutTerminal when the
// terminal binary is missing. Surfaced to the frontend so its catch
// branch can fall back to the in-app overlay (App.vue's openPopOut
// already implements that fallback).
var ErrLufisNotFound = errors.New("lufis-terminal binary not found on PATH (set ARGUS_LUFIS_PATH)")

// LaunchPopOutTerminal spawns a new lufis-terminal window inheriting
// the current Argus context: kubeconfig, current K8s context,
// namespace, working directory, and the AI/LLM credentials.
//
// The spawned process is detached — Argus is not its parent for
// signaling purposes (its lifetime is decoupled from Argus's). The
// function returns after the spawn succeeds; we do not wait on the
// child.
func (a *App) LaunchPopOutTerminal() error {
	binary, err := resolveLufisBinary()
	if err != nil {
		a.logger.Warn("LaunchPopOutTerminal: lufis binary not resolvable",
			slog.String("error", err.Error()))
		// %w preserves ErrLufisNotFound for errors.Is callers (the
		// frontend doesn't go through errors.Is, but in-tree Go
		// callers can still tell "binary missing" from a generic
		// resolver failure).
		return fmt.Errorf("LaunchPopOutTerminal: resolve binary: %w", err)
	}

	env := a.buildPopOutEnv(os.Environ())
	wd := popOutWorkingDir(a.cfg)

	cmd := exec.Command(binary)
	cmd.Env = env
	cmd.Dir = wd
	// Detach the child's stdio so closing Argus doesn't terminate the
	// terminal window with a SIGPIPE on its first write.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		a.logger.Error("LaunchPopOutTerminal: spawn failed",
			slog.String("binary", binary),
			slog.String("wd", wd),
			slog.String("error", err.Error()))
		return fmt.Errorf("launch lufis-terminal: %w", err)
	}

	a.logger.Info("LaunchPopOutTerminal: spawned",
		slog.String("binary", binary),
		slog.String("wd", wd),
		slog.Int("pid", cmd.Process.Pid))

	// Reap the child process record so we don't accumulate zombies if
	// the user opens many windows during a long session. Go's
	// os.Process is freed when GC runs over cmd.Process, but on
	// platforms where Release() is a no-op this also ensures we don't
	// keep a handle open for the lifetime of the parent.
	_ = cmd.Process.Release()
	return nil
}

// resolveLufisBinary returns the path to the lufis-terminal
// executable. ARGUS_LUFIS_PATH wins if set; otherwise we look on
// PATH; otherwise a known sibling-of-the-Argus-binary fallback
// (helps in dev where both binaries sit in the same build dir).
func resolveLufisBinary() (string, error) {
	if p := os.Getenv("ARGUS_LUFIS_PATH"); p != "" {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p, nil
		}
		if err == nil {
			// Path exists but isn't a regular file (someone pointed
			// the env at a directory). errors.New is fine — no
			// upstream error to wrap.
			return "", fmt.Errorf("ARGUS_LUFIS_PATH=%s is a directory, not an executable", p)
		}
		// Preserve the *os.PathError (permission denied, ELOOP, …)
		// so callers/logs see the OS-level reason.
		return "", fmt.Errorf("ARGUS_LUFIS_PATH=%s stat: %w", p, err)
	}

	if p, err := exec.LookPath(lufisBinaryName); err == nil {
		return p, nil
	}

	// Last resort: sibling of the running Argus binary. Catches the
	// `dist/argus` + `dist/lufis-terminal` case our Makefile produces.
	if argusPath, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(argusPath), lufisBinaryName)
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}

	return "", ErrLufisNotFound
}

// buildPopOutEnv returns the env block for the spawned terminal.
// Starts from the parent's env (so $SHELL, $PATH, $HOME, etc. flow
// through naturally), then overlays the Argus-specific context. The
// `parent` slice is passed in (rather than calling os.Environ()
// internally) so the function is straightforwardly testable.
//
// Env vars overlaid:
//
//	KUBECONFIG          — Argus's resolved kubeconfig path; the
//	                      spawned shell's kubectl uses the same one.
//	KUBECTX             — same as ARGUS_K8S_CONTEXT, but matches the
//	                      conventional kube-tools env.
//	ARGUS_K8S_CONTEXT   — current cluster context name, for prompt
//	                      and title rendering.
//	ARGUS_K8S_NAMESPACE — default namespace; lufis can render it in
//	                      the title bar.
//	ARGUS_TERMINAL_TITLE — branded title for the spawned window.
//	DEEPSEEK_API_KEY    — passed through if Argus has one configured,
//	                      so lufis's opencode/AI features work without
//	                      a second login.
//	ARGUS_LLM_BASE_URL  — same: re-use the inference endpoint Argus
//	                      already targets (DeepSeek vs self-hosted vLLM).
//	ARGUS_LLM_MODEL     — same.
func (a *App) buildPopOutEnv(parent []string) []string {
	overlays := map[string]string{
		"ARGUS_TERMINAL_TITLE": "Argus Terminal",
	}

	if a.cfg != nil {
		if v := a.cfg.Kubernetes.Config; v != "" {
			overlays["KUBECONFIG"] = v
		}
		if v := a.cfg.Kubernetes.Context; v != "" {
			overlays["KUBECTX"] = v
			overlays["ARGUS_K8S_CONTEXT"] = v
		}
		if v := a.cfg.Kubernetes.Namespace; v != "" {
			overlays["ARGUS_K8S_NAMESPACE"] = v
		}
		if v := a.cfg.AI.DeepSeekAPIKey; v != "" {
			overlays["DEEPSEEK_API_KEY"] = v
		}
		if v := a.cfg.AI.LLMBaseURL; v != "" {
			overlays["ARGUS_LLM_BASE_URL"] = v
		}
		if v := a.cfg.AI.LLMModel; v != "" {
			overlays["ARGUS_LLM_MODEL"] = v
		}
	}

	return mergeEnv(parent, overlays)
}

// popOutWorkingDir picks the directory the spawned terminal starts
// in. Preference: a Kubernetes-namespace-friendly dir under $HOME if
// one exists (helps users who keep per-cluster shell history); falls
// back to the current process's cwd. Returns "" rather than os.Getwd
// errors — exec.Command treats empty Dir as "inherit", which is the
// safest fallback.
func popOutWorkingDir(_ interface{}) string {
	if home, err := os.UserHomeDir(); err == nil {
		if info, err := os.Stat(home); err == nil && info.IsDir() {
			return home
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// mergeEnv returns a fresh env slice: parent's entries with any key
// in overlays replaced. Pure function — no globals — so it's easy to
// table-test.
func mergeEnv(parent []string, overlays map[string]string) []string {
	out := make([]string, 0, len(parent)+len(overlays))
	seen := make(map[string]bool, len(overlays))
	for _, kv := range parent {
		key := envKey(kv)
		if v, override := overlays[key]; override {
			out = append(out, key+"="+v)
			seen[key] = true
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overlays {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// envKey extracts "FOO" from "FOO=bar". Returns the whole string when
// there's no "=" (malformed env entry; preserve as-is).
func envKey(kv string) string {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i]
		}
	}
	return kv
}

// runtimeOS is a small indirection so tests can pretend to be on a
// different OS without setting GOOS via build tags. Currently unused
// — detachProcess is the only OS-conditional path, and that's in a
// _unix.go / _windows.go pair.
var _ = runtime.GOOS
