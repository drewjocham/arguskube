package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	m := New()
	assert.Len(t, m.Panes(), 1)
	assert.Equal(t, "root", m.Panes()[0].ID)
}

func TestResize(t *testing.T) {
	m := New()
	m.Resize(120, 40)
	assert.Equal(t, 120, m.Panes()[0].Width)
	assert.Equal(t, 40, m.Panes()[0].Height)
}

func TestSplitHorizontal(t *testing.T) {
	m := New()
	p, err := m.Split("root", DirHorizontal)
	assert.NoError(t, err)
	assert.Len(t, m.Panes(), 2)
	assert.NotEqual(t, m.Panes()[0].ID, m.Panes()[1].ID)
	_ = p
}

func TestSplitVertical(t *testing.T) {
	m := New()
	p, err := m.Split("root", DirVertical)
	assert.NoError(t, err)
	assert.Len(t, m.Panes(), 2)
	_ = p
}

func TestSplitNonexistent(t *testing.T) {
	m := New()
	_, err := m.Split("nonexistent", DirHorizontal)
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	m := New()
	m.Split("root", DirHorizontal)
	assert.Len(t, m.Panes(), 2)
	m.Close(m.Panes()[1].ID)
	assert.Len(t, m.Panes(), 1)
}

func TestVisible(t *testing.T) {
	m := New()
	m.Split("root", DirHorizontal)
	assert.Len(t, m.Visible(), 2)
}

func TestVisibleAfterClose(t *testing.T) {
	m := New()
	m.Split("root", DirHorizontal)
	childID := m.Panes()[1].ID
	m.Close(childID)
	assert.Len(t, m.Visible(), 1)
}
