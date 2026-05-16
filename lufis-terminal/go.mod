module github.com/argus/terminal

go 1.26.2

require github.com/argues/argus-pty v0.0.0-00010101000000-000000000000

// See docs/lufis-terminal-audit-plan.md PR-3: argus-pty is the
// shared PTY core; go.work at the repo root resolves it locally,
// but the replace keeps non-workspace tooling happy.
replace github.com/argues/argus-pty => ../pkg/pty

require (
	github.com/creack/pty v1.1.24
	github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20260406072232-3ac4aa2bb164
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/pelletier/go-toml/v2 v2.3.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/image v0.40.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
