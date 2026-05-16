package recorder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	r := New(t.TempDir(), nil)
	assert.NotNil(t, r)
	assert.False(t, r.IsRecording())
}

func TestStartStop(t *testing.T) {
	r := New(t.TempDir(), nil)
	rec, err := r.Start()
	assert.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, "recording", rec.Status)
	assert.True(t, r.IsRecording())

	stopped, err := r.Stop()
	assert.NoError(t, err)
	assert.Equal(t, "completed", stopped.Status)
	assert.False(t, r.IsRecording())
}

func TestDoubleStart(t *testing.T) {
	r := New(t.TempDir(), nil)
	_, err := r.Start()
	assert.NoError(t, err)
	_, err = r.Start()
	assert.Error(t, err)
	r.Stop()
}

func TestStopWithoutStart(t *testing.T) {
	r := New(t.TempDir(), nil)
	_, err := r.Stop()
	assert.Error(t, err)
}

func TestCancel(t *testing.T) {
	r := New(t.TempDir(), nil)
	r.Start()
	err := r.Cancel()
	assert.NoError(t, err)
	assert.False(t, r.IsRecording())
}

func TestCancelWithoutStart(t *testing.T) {
	r := New(t.TempDir(), nil)
	err := r.Cancel()
	assert.Error(t, err)
}

func TestHistory(t *testing.T) {
	r := New(t.TempDir(), nil)
	assert.Empty(t, r.History())

	r.Start()
	r.Stop()
	assert.Len(t, r.History(), 1)

	r.Start()
	r.Stop()
	assert.Len(t, r.History(), 2)
}

func TestHistoryPersistence(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, nil)
	r.Start()
	r.Stop()

	r2 := New(dir, nil)
	assert.Len(t, r2.History(), 1)
}
