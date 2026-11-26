package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFileStartsEmpty(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "state.json"), 2026)
	require.NoError(t, err)
	assert.Equal(t, State{Year: 2026}, store.Get())
}

func TestUpdate_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Load(path, 2026)
	require.NoError(t, err)

	require.NoError(t, store.Update(func(s *State) {
		s.LeaderboardMessageID = "123"
		s.LastAnnouncedDay = 4
	}))

	reloaded, err := Load(path, 2026)
	require.NoError(t, err)
	assert.Equal(t, State{Year: 2026, LeaderboardMessageID: "123", LastAnnouncedDay: 4}, reloaded.Get())
}

func TestLoad_OtherYearStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, _ := Load(path, 2025)
	require.NoError(t, store.Update(func(s *State) { s.LastAnnouncedDay = 12 }))

	reloaded, err := Load(path, 2026)
	require.NoError(t, err)
	assert.Equal(t, State{Year: 2026}, reloaded.Get())
}

func TestLoad_CorruptFileStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))

	store, err := Load(path, 2026)
	require.NoError(t, err)
	assert.Equal(t, State{Year: 2026}, store.Get())
}

func TestUpdate_FailedWriteKeepsOldState(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "missing-dir", "state.json"), 2026)
	require.NoError(t, err)

	assert.Error(t, store.Update(func(s *State) { s.LastAnnouncedDay = 1 }))
	assert.Equal(t, 0, store.Get().LastAnnouncedDay)
}
