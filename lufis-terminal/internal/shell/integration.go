package shell

import (
	"fmt"
	"strings"
	"time"
)

type Integration struct {
	PromptStart string
}

type CommandInfo struct {
	Command  string
	ExitCode int
	Duration time.Duration
	Dir      string
}

func New() *Integration {
	return &Integration{
		PromptStart: "\x1b]0;argus-terminal\x07",
	}
}

func (i *Integration) ParsePS0(data string) string {
	return strings.ReplaceAll(data, "\x1b[?2004h", "")
}

func (i *Integration) FormatExitCode(code int) string {
	if code == 0 {
		return "✓"
	}
	return fmt.Sprintf("✗ %d", code)
}

func (i *Integration) FormatTiming(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

func (i *Integration) WrapPrompt(prompt string, info CommandInfo) string {
	if info.Duration > 0 {
		return fmt.Sprintf("%s %s %s", prompt, i.FormatTiming(info.Duration), i.FormatExitCode(info.ExitCode))
	}
	return prompt
}
