package opencode

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const safePATH = "/usr/bin:/usr/local/bin:/opt/homebrew/bin:/bin:/usr/libexec"

func secureBin(name string) (string, []string) {
	paths := filepath.SplitList(safePATH)
	for _, dir := range paths {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode().Perm()&0111 != 0 {
			return p, append(os.Environ(), "PATH="+safePATH)
		}
	}
	return name, append(os.Environ(), "PATH="+safePATH)
}

type Agent struct {
	client  *Client
	session *Session
	logger  *slog.Logger
}

func NewAgent(cfg ModelConfig, logger *slog.Logger) *Agent {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Agent{
		client: NewClient(cfg, logger),
		logger: logger,
	}
}

func (a *Agent) Spawn(ctx context.Context, workdir, model string) (*Session, error) {
	session := &Session{
		ID:      fmt.Sprintf("session-%d", time.Now().UnixNano()),
		Status:  StatusIdle,
		Model:   model,
		Workdir: workdir,
		Messages: []Message{
			{Role: RoleSystem, Content: systemPrompt(workdir)},
		},
	}
	a.session = session
	return session, nil
}

func (a *Agent) Prompt(ctx context.Context, message string) (string, error) {
	if a.session == nil {
		return "", fmt.Errorf("no active session, call Spawn first")
	}

	a.session.Status = StatusThinking
	a.session.Messages = append(a.session.Messages, Message{Role: RoleUser, Content: message})

	response, err := a.client.Chat(ctx, a.session.Messages)
	if err != nil {
		a.session.Status = StatusError
		return "", fmt.Errorf("chat: %w", err)
	}

	a.session.Messages = append(a.session.Messages, Message{Role: RoleAssistant, Content: response})
	a.session.Status = StatusDone
	return response, nil
}

func (a *Agent) Session() *Session { return a.session }

func systemPrompt(workdir string) string {
	return fmt.Sprintf(`You are an AI coding agent running inside the Argus Terminal.
You can read, write, and edit files, execute commands, search code, and use git.

Working directory: %s

Follow these rules:
1. Always explain what you are doing before executing
2. Use the available tools to complete the task
3. Verify your work after making changes
4. If a command fails, diagnose and fix`, workdir)
}

type ToolSet struct {
	Workdir string
	Logger  *slog.Logger
}

func NewToolSet(workdir string, logger *slog.Logger) *ToolSet {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ToolSet{
		Workdir: workdir,
		Logger:  logger,
	}
}

func (ts *ToolSet) Read(path string) (string, error) {
	fullPath := ts.resolve(path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func (ts *ToolSet) Write(path, content string) error {
	fullPath := ts.resolve(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (ts *ToolSet) Edit(path, old, new string) error {
	content, err := ts.Read(path)
	if err != nil {
		return err
	}
	updated := strings.Replace(content, old, new, 1)
	if updated == content {
		return fmt.Errorf("old string not found in %s", path)
	}
	return ts.Write(path, updated)
}

func (ts *ToolSet) Exec(command string) (string, error) {
	shPath, env := secureBin("sh")
	cmd := exec.Command(shPath, "-c", command)
	cmd.Env = env
	cmd.Dir = ts.Workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("exec %s: %w", command, err)
	}
	return string(output), nil
}

func (ts *ToolSet) Search(pattern string) ([]string, error) {
	files, err := filepath.Glob(ts.resolve(pattern))
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	return files, nil
}

func (ts *ToolSet) Grep(pattern string) ([]string, error) {
	grepPath, env := secureBin("grep")
	cmd := exec.Command(grepPath, "-rn", pattern, ts.Workdir)
	cmd.Env = env
	cmd.Dir = ts.Workdir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}
	var results []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		results = append(results, scanner.Text())
	}
	return results, nil
}

func (ts *ToolSet) Git(args ...string) (string, error) {
	gitPath, env := secureBin("git")
	cmd := exec.Command(gitPath, args...)
	cmd.Env = env
	cmd.Dir = ts.Workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %v: %w", args, err)
	}
	return string(output), nil
}

func (ts *ToolSet) Diff(path string) (string, error) {
	return ts.Exec(fmt.Sprintf("git diff %s", path))
}

func (ts *ToolSet) Glob(pattern string) ([]string, error) {
	return ts.Search(pattern)
}

func (ts *ToolSet) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(ts.Workdir, path)
}
