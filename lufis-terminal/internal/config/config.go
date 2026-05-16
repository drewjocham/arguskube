package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Terminal TerminalConfig `toml:"terminal"`
	Argus    ArgusContext   `toml:"-"`
}

type TerminalConfig struct {
	Shell    string `toml:"shell"`
	FontSize int    `toml:"font_size"`
	Width    int    `toml:"width"`
	Height   int    `toml:"height"`
	// Title overrides the OS window title. Empty falls back to
	// "Argus Terminal". Populated from $ARGUS_TERMINAL_TITLE when the
	// process is launched by Argus.
	Title string `toml:"title"`
}

// ArgusContext is the bag of values the Argus app injects via env
// when it spawns lufis-terminal via LaunchPopOutTerminal. None are
// stored in the on-disk TOML — they live for the lifetime of the
// spawned process and are reset on every launch. Empty fields mean
// "not provided"; consumers should treat them as advisory.
//
// The corresponding env var names match what
// kube/backend/api/pkg.buildPopOutEnv writes, so changes need to
// happen in lock-step across both repos.
type ArgusContext struct {
	K8sContext   string // $ARGUS_K8S_CONTEXT — current cluster context
	K8sNamespace string // $ARGUS_K8S_NAMESPACE — default namespace
	Kubeconfig   string // $KUBECONFIG — path to the kubeconfig
}

func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/zsh"
}

func defaults() Config {
	return Config{
		Terminal: TerminalConfig{
			Shell:    defaultShell(),
			FontSize: 14,
			Width:    1000,
			Height:   650,
		},
	}
}

func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	p := filepath.Join(dir, "argus-terminal", "config.toml")
	return p, nil
}

func Load() (Config, error) {
	cfg := defaults()

	if v := os.Getenv("ARGUS_SHELL"); v != "" {
		cfg.Terminal.Shell = v
	}
	if v := os.Getenv("ARGUS_TERMINAL_TITLE"); v != "" {
		cfg.Terminal.Title = v
	}
	// Argus context — populated only when the Argus app spawned us.
	cfg.Argus = ArgusContext{
		K8sContext:   os.Getenv("ARGUS_K8S_CONTEXT"),
		K8sNamespace: os.Getenv("ARGUS_K8S_NAMESPACE"),
		Kubeconfig:   os.Getenv("KUBECONFIG"),
	}

	p, err := path()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", p, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", p, err)
	}

	return cfg, nil
}

func Save(cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	p, err := path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return os.Rename(tmp, p)
}
