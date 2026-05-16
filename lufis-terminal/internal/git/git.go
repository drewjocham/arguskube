package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

type StatusEntry struct {
	IndexStatus byte
	WorkStatus  byte
	Path        string
}

type Branch struct {
	Name   string
	IsHead bool
}

type LogEntry struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

type BlameLine struct {
	Line    int
	Hash    string
	Author  string
	Date    string
	Content string
}

func Status(repo string) ([]StatusEntry, error) {
	cmd := exec.Command("git", "-C", repo, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	var entries []StatusEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		entries = append(entries, StatusEntry{
			IndexStatus: line[0],
			WorkStatus:  line[1],
			Path:        strings.TrimSpace(line[3:]),
		})
	}
	return entries, nil
}

func Branches(repo string) ([]Branch, error) {
	cmd := exec.Command("git", "-C", repo, "branch")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	var branches []Branch
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		isHead := line[0] == '*'
		name := strings.TrimSpace(line[2:])
		branches = append(branches, Branch{Name: name, IsHead: isHead})
	}
	return branches, nil
}

func Log(repo string, count int) ([]LogEntry, error) {
	args := []string{"-C", repo, "log", fmt.Sprintf("--max-count=%d", count),
		"--format=%H%n%an%n%ad%n%s%n---"}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var entries []LogEntry
	lines := strings.Split(string(out), "\n")
	for i := 0; i+4 < len(lines); i += 5 {
		entries = append(entries, LogEntry{
			Hash:    lines[i],
			Author:  lines[i+1],
			Date:    lines[i+2],
			Message: lines[i+3],
		})
	}
	return entries, nil
}

func Blame(repo, path string) ([]BlameLine, error) {
	cmd := exec.Command("git", "-C", repo, "blame", "--porcelain", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame: %w", err)
	}
	return parseBlame(string(out)), nil
}

func Diff(repo string, args ...string) (string, error) {
	gitArgs := append([]string{"-C", repo, "diff"}, args...)
	cmd := exec.Command("git", gitArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func Commit(repo, message string) error {
	cmd := exec.Command("git", "-C", repo, "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func parseBlame(raw string) []BlameLine {
	var lines []BlameLine
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var pending map[string]string
	lineNum := 0

	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		if text[0] == '\t' {
			lineNum++
			lines = append(lines, BlameLine{
				Line:    lineNum,
				Hash:    pending["hash"],
				Author:  pending["author"],
				Date:    pending["date"],
				Content: text[1:],
			})
			continue
		}
		parts := strings.SplitN(text, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "author":
				pending["author"] = parts[1]
			case "author-time":
				pending["date"] = parts[1]
			default:
				if len(parts[0]) == 40 {
					pending = map[string]string{"hash": parts[0]}
				}
			}
		}
	}
	return lines
}
