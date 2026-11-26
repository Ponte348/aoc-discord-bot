package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"

	"github.com/PaytonWebber/aoc-discord-bot/internal/fileutil"
)

type State struct {
	Year                 int    `json:"year"`
	LeaderboardMessageID string `json:"leaderboard_message_id"`
	LastAnnouncedDay     int    `json:"last_announced_day"`
}

type Store struct {
	path  string
	mu    sync.Mutex
	state State
}

func Load(path string, year int) (*Store, error) {
	store := &Store{path: path, state: State{Year: year}}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading state file: %w", err)
	}

	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("state file %s is corrupt, starting fresh: %v", path, err)
		return store, nil
	}
	if loaded.Year != year {
		log.Printf("state file is for %d, not %d; starting fresh", loaded.Year, year)
		return store, nil
	}

	store.state = loaded
	return store, nil
}

func (s *Store) Get() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Store) Update(change func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.state
	change(&next)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling state: %w", err)
	}
	if err := fileutil.WriteFileAtomic(s.path, data); err != nil {
		return err
	}
	s.state = next
	return nil
}
