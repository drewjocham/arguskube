package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setFakeHome sets HOME to a temp directory using t.Setenv so it is
// goroutine-safe and auto-restored.
func setFakeHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", t.TempDir())
	}
}

// unsetEnv is a goroutine-safe equivalent of os.Unsetenv. It sets the
// variable to the empty string, which is functionally identical to
// unsetting for os.Getenv-based code (both return "").
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
}

func TestDefaults(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")

	tests := []struct {
		name     string
		envShell string
		want     Config
	}{
		{
			name:     "uses SHELL env var when set",
			envShell: "/bin/bash",
			want: Config{
				Terminal: TerminalConfig{
					Shell:    "/bin/bash",
					FontSize: 14,
					Width:    1000,
					Height:   650,
				},
			},
		},
		{
			name:     "uses /bin/zsh on non-windows when SHELL unset",
			envShell: "",
			want: Config{
				Terminal: TerminalConfig{
					Shell:    "/bin/zsh",
					FontSize: 14,
					Width:    1000,
					Height:   650,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.envShell)
			unsetEnv(t, "ARGUS_SHELL")

			got := defaults()

			if runtime.GOOS == "windows" && tt.envShell == "" {
				tt.want.Terminal.Shell = "cmd.exe"
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEnvVarOverride(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")

	tests := []struct {
		name     string
		shellEnv string
		argusEnv string
		want     string
	}{
		{
			name:     "ARGUS_SHELL overrides SHELL",
			shellEnv: "/bin/bash",
			argusEnv: "/bin/fish",
			want:     "/bin/fish",
		},
		{
			name:     "ARGUS_SHELL alone sets shell",
			shellEnv: "",
			argusEnv: "/bin/fish",
			want:     "/bin/fish",
		},
		{
			name:     "no ARGUS_SHELL uses SHELL",
			shellEnv: "/bin/zsh",
			argusEnv: "",
			want:     "/bin/zsh",
		},
		{
			// want is computed inside the subtest after t.Setenv clears
			// SHELL — evaluating defaultShell() at table-construction
			// time would capture the runner's real $SHELL (e.g. /bin/bash
			// on ubuntu-latest) and mismatch the empty-env fallback.
			name:     "neither set uses default",
			shellEnv: "",
			argusEnv: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shellEnv)
			t.Setenv("ARGUS_SHELL", tt.argusEnv)

			want := tt.want
			if want == "" {
				want = defaultShell()
			}

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, want, cfg.Terminal.Shell)
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")

	tests := []struct {
		name       string
		configData string
		wantShell  string
		wantFont   int
		wantWidth  int
		wantHeight int
	}{
		{
			name: "loads valid TOML config",
			configData: `[terminal]
shell = "/bin/fish"
font_size = 16
width = 1200
height = 800
`,
			wantShell:  "/bin/fish",
			wantFont:   16,
			wantWidth:  1200,
			wantHeight: 800,
		},
		{
			name: "partial TOML merges with defaults",
			configData: `[terminal]
font_size = 18
`,
			wantShell:  defaultShell(),
			wantFont:   18,
			wantWidth:  1000,
			wantHeight: 650,
		},
		{
			name:       "config file with empty terminal section uses defaults",
			configData: "[terminal]\n",
			wantShell:  defaultShell(),
			wantFont:   14,
			wantWidth:  1000,
			wantHeight: 650,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, "ARGUS_SHELL")

			p, err := path()
			require.NoError(t, err)
			err = os.MkdirAll(filepath.Dir(p), 0o700)
			require.NoError(t, err)
			err = os.WriteFile(p, []byte(tt.configData), 0o600)
			require.NoError(t, err)

			cfg, err := Load()
			require.NoError(t, err)

			assert.Equal(t, tt.wantShell, cfg.Terminal.Shell)
			assert.Equal(t, tt.wantFont, cfg.Terminal.FontSize)
			assert.Equal(t, tt.wantWidth, cfg.Terminal.Width)
			assert.Equal(t, tt.wantHeight, cfg.Terminal.Height)
		})
	}
}

func TestLoadFileNotFound(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")
	unsetEnv(t, "ARGUS_SHELL")
	unsetEnv(t, "SHELL")

	cfg, err := Load()
	require.NoError(t, err)

	expected := defaults()
	assert.Equal(t, expected, cfg)
}

func TestLoadUserConfigDirError(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")

	t.Run("path returns valid path", func(t *testing.T) {
		p, err := path()
		require.NoError(t, err)
		assert.Contains(t, p, "argus-terminal")
		assert.Contains(t, p, "config.toml")
	})
}

func TestSave(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		checkFn func(t *testing.T, cfg Config)
	}{
		{
			name: "writes config to file successfully",
			cfg: Config{
				Terminal: TerminalConfig{
					Shell:    "/bin/zsh",
					FontSize: 14,
					Width:    1000,
					Height:   650,
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, cfg Config) {
				loaded, err := Load()
				require.NoError(t, err)
				assert.Equal(t, cfg, loaded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, "ARGUS_SHELL")

			err := Save(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.checkFn != nil {
				tt.checkFn(t, tt.cfg)
			}
		})
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")
	unsetEnv(t, "ARGUS_SHELL")

	cfg := Config{
		Terminal: TerminalConfig{
			Shell:    "/bin/zsh",
			FontSize: 14,
			Width:    1000,
			Height:   650,
		},
	}

	err := Save(cfg)
	require.NoError(t, err)

	p, err := path()
	require.NoError(t, err)
	dir := filepath.Dir(p)

	_, err = os.Stat(dir)
	require.NoError(t, err, "config directory should exist after Save")

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, dirInfo.IsDir())
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	_, err = os.Stat(p)
	require.NoError(t, err, "config file should exist")
}

func TestSaveOverwritesExisting(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")
	unsetEnv(t, "ARGUS_SHELL")

	initial := Config{
		Terminal: TerminalConfig{
			Shell:    "/bin/bash",
			FontSize: 12,
			Width:    800,
			Height:   600,
		},
	}
	err := Save(initial)
	require.NoError(t, err)

	updated := Config{
		Terminal: TerminalConfig{
			Shell:    "/bin/fish",
			FontSize: 18,
			Width:    1920,
			Height:   1080,
		},
	}
	err = Save(updated)
	require.NoError(t, err)

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, updated, loaded)
}

func TestSaveAtomicDoesNotLeaveTempFile(t *testing.T) {
	setFakeHome(t)
	unsetEnv(t, "KUBECONFIG")
	unsetEnv(t, "ARGUS_SHELL")

	cfg := Config{
		Terminal: TerminalConfig{
			Shell:    "/bin/zsh",
			FontSize: 14,
			Width:    1000,
			Height:   650,
		},
	}

	err := Save(cfg)
	require.NoError(t, err)

	p, err := path()
	require.NoError(t, err)
	tmpPath := p + ".tmp"

	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "temp file should not exist after atomic save")
}

func TestPath(t *testing.T) {
	t.Run("returns argus-terminal/config.toml path", func(t *testing.T) {
		p, err := path()
		require.NoError(t, err)
		assert.Contains(t, p, "argus-terminal")
		assert.Contains(t, p, "config.toml")
		assert.Equal(t, "config.toml", filepath.Base(p))

		parentDir := filepath.Base(filepath.Dir(p))
		assert.Equal(t, "argus-terminal", parentDir)
	})
}

func TestDefaultShell(t *testing.T) {
	tests := []struct {
		name     string
		envShell string
		want     string
	}{
		{
			name:     "returns SHELL env var",
			envShell: "/bin/bash",
			want:     "/bin/bash",
		},
		{
			name:     "returns /bin/zsh on unix when SHELL unset",
			envShell: "",
			want:     "/bin/zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.envShell)

			got := defaultShell()

			if runtime.GOOS == "windows" && tt.envShell == "" {
				tt.want = "cmd.exe"
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
