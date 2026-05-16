package shell

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatExitCode(t *testing.T) {
	assert.Contains(t, New().FormatExitCode(0), "✓")
	assert.Contains(t, New().FormatExitCode(1), "✗")
	assert.Contains(t, New().FormatExitCode(1), "1")
}

func TestFormatTiming(t *testing.T) {
	i := New()
	assert.Equal(t, "500ms", i.FormatTiming(500*time.Millisecond))
	assert.Equal(t, "1.5s", i.FormatTiming(1500*time.Millisecond))
	assert.Contains(t, i.FormatTiming(90*time.Second), "m")
	assert.Contains(t, i.FormatTiming(90*time.Second), "s")
}

func TestWrapPrompt(t *testing.T) {
	i := New()
	result := i.WrapPrompt("$", CommandInfo{ExitCode: 0, Duration: time.Second})
	assert.Contains(t, result, "$")
	assert.Contains(t, result, "1.0s")
	assert.Contains(t, result, "✓")
}

func TestWrapPromptNoDuration(t *testing.T) {
	i := New()
	result := i.WrapPrompt("$", CommandInfo{ExitCode: 1})
	assert.Equal(t, "$", result)
}

func TestParsePS0(t *testing.T) {
	i := New()
	result := i.ParsePS0("data\x1b[?2004h")
	assert.NotContains(t, result, "\x1b[?2004h")
}
