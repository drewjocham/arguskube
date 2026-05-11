package pkg

import (
	"testing"
)

func TestGetAppMode_Default(t *testing.T) {
	a := &App{}
	mode := a.GetAppMode()
	if mode != "dashboard" {
		t.Errorf("expected 'dashboard', got %q", mode)
	}
}

func TestGetAppMode_Custom(t *testing.T) {
	a := &App{appMode: "terminal"}
	mode := a.GetAppMode()
	if mode != "terminal" {
		t.Errorf("expected 'terminal', got %q", mode)
	}
}

func TestSetPaused(t *testing.T) {
	a := &App{}
	a.SetPaused(true)
	if !a.paused.Load() {
		t.Error("expected paused to be true")
	}
	a.SetPaused(false)
	if a.paused.Load() {
		t.Error("expected paused to be false")
	}
}
