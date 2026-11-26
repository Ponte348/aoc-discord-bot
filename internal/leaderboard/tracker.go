package leaderboard

import (
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
)

const PollInterval = 15 * time.Minute

type AOCClient interface {
	GetLeaderboard(leaderboardID string) (*aoc.Leaderboard, error)
}

// Tracker manages the leaderboard state
type Tracker struct {
	PreviousLeaderboard *aoc.Leaderboard
	CurrentLeaderboard  *aoc.Leaderboard
	Client              AOCClient
	Config              *config.Config
	LastUpdate          time.Time
}

type StarEvent struct {
	Member aoc.Member
	Day    int
	Part   int
	At     time.Time
}

func NewTracker(cfg *config.Config, StoredLeaderboard *aoc.Leaderboard, client AOCClient) *Tracker {
	return &Tracker{
		Client:             client,
		Config:             cfg,
		CurrentLeaderboard: StoredLeaderboard,
	}
}

func (t *Tracker) GetLeaderboard() (*aoc.Leaderboard, error) {
	leaderboard, err := t.Client.GetLeaderboard(t.Config.LeaderboardID)
	if err != nil {
		return nil, err
	}

	return leaderboard, nil
}

func (t *Tracker) Due(now time.Time) bool {
	return now.Sub(t.LastUpdate) >= PollInterval
}

func (t *Tracker) UpdateLeaderboard() error {
	t.LastUpdate = time.Now()
	leaderboard, err := t.GetLeaderboard()
	if err != nil {
		return err
	}

	t.PreviousLeaderboard = t.CurrentLeaderboard
	t.CurrentLeaderboard = leaderboard

	return nil
}

func (t *Tracker) CheckForNewStars() ([]StarEvent, error) {
	if t.PreviousLeaderboard == nil || t.CurrentLeaderboard == nil {
		return nil, nil
	}

	var events []StarEvent
	for memberID, member := range t.CurrentLeaderboard.Members {
		previousMember, ok := t.PreviousLeaderboard.Members[memberID]
		if !ok {
			continue
		}
		for dayKey, levels := range member.CompletionDayLevels {
			day, err := strconv.Atoi(dayKey)
			if err != nil {
				continue
			}
			previous := previousMember.CompletionDayLevels[dayKey]
			if levels.Level1 != nil && previous.Level1 == nil {
				events = append(events, newStarEvent(member, day, 1, levels.Level1))
			}
			if levels.Level2 != nil && previous.Level2 == nil {
				events = append(events, newStarEvent(member, day, 2, levels.Level2))
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Member.ID != b.Member.ID {
			return a.Member.ID < b.Member.ID
		}
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return a.Part < b.Part
	})

	return events, nil
}

func newStarEvent(member aoc.Member, day, part int, star *aoc.StarDetail) StarEvent {
	return StarEvent{Member: member, Day: day, Part: part, At: time.Unix(int64(star.GetStarTs), 0)}
}

func (t *Tracker) CheckForNewMembers() ([]string, error) {
	if t.PreviousLeaderboard == nil || t.CurrentLeaderboard == nil {
		return nil, nil
	}

	var newMembers []string
	for _, member := range SortedMembers(t.CurrentLeaderboard) {
		if _, ok := t.PreviousLeaderboard.Members[strconv.Itoa(member.ID)]; !ok {
			log.Printf("New member: %s", member.DisplayName())
			newMembers = append(newMembers, member.DisplayName())
		}
	}

	return newMembers, nil
}
