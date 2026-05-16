package layout

import (
	"fmt"
)

type Direction int

const (
	DirHorizontal Direction = iota
	DirVertical
)

type Pane struct {
	ID       string    `json:"id"`
	X        int       `json:"x"`
	Y        int       `json:"y"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Children []Pane    `json:"children,omitempty"`
	SplitDir Direction `json:"split_dir,omitempty"`
	Shell    string    `json:"shell,omitempty"`
	Label    string    `json:"label,omitempty"`
}

type Manager struct {
	panes []Pane
	maxID int
}

func New() *Manager {
	return &Manager{
		panes: []Pane{{
			ID: "root", X: 0, Y: 0,
			Width: 80, Height: 24,
			Shell: "zsh", Label: "main",
		}},
	}
}

func (m *Manager) Panes() []Pane { return m.panes }

func (m *Manager) Resize(width, height int) {
	for i := range m.panes {
		m.panes[i].Width = width
		m.panes[i].Height = height
	}
}

func (m *Manager) Split(id string, dir Direction) (*Pane, error) {
	parent, idx := m.find(id)
	if parent == nil {
		return nil, fmt.Errorf("pane %s not found", id)
	}
	m.maxID++
	childID := fmt.Sprintf("pane-%d", m.maxID)

	switch dir {
	case DirHorizontal:
		parent.Width /= 2
		child := Pane{
			ID: childID, X: parent.X + parent.Width, Y: parent.Y,
			Width: parent.Width, Height: parent.Height, Shell: parent.Shell,
		}
		parent.SplitDir = dir
		m.panes = append(m.panes[:idx+1], append([]Pane{child}, m.panes[idx+1:]...)...)
	case DirVertical:
		parent.Height /= 2
		child := Pane{
			ID: childID, X: parent.X, Y: parent.Y + parent.Height,
			Width: parent.Width, Height: parent.Height, Shell: parent.Shell,
		}
		parent.SplitDir = dir
		m.panes = append(m.panes[:idx+1], append([]Pane{child}, m.panes[idx+1:]...)...)
	}
	return parent, nil
}

func (m *Manager) Close(id string) {
	for i, p := range m.panes {
		if p.ID == id {
			m.panes = append(m.panes[:i], m.panes[i+1:]...)
			return
		}
	}
}

func (m *Manager) Visible() []Pane { return m.panes }

func (m *Manager) find(id string) (*Pane, int) {
	for i := range m.panes {
		if m.panes[i].ID == id {
			return &m.panes[i], i
		}
	}
	return nil, -1
}
