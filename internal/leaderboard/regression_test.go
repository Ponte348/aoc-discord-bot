package leaderboard

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// days builds completion data where day d has part 1 at d*1000 and part 2 at d*1000+500;
// a negative d means only part 1 is complete.
func days(ds ...int) map[string]aoc.CompletionDayLevel {
	out := make(map[string]aoc.CompletionDayLevel)
	for _, d := range ds {
		partOneOnly := d < 0
		if partOneOnly {
			d = -d
		}
		level := aoc.CompletionDayLevel{Level1: &aoc.StarDetail{GetStarTs: d * 1000}}
		if !partOneOnly {
			level.Level2 = &aoc.StarDetail{GetStarTs: d*1000 + 500}
		}
		out[strconv.Itoa(d)] = level
	}
	return out
}

func member(id int, name string, completion map[string]aoc.CompletionDayLevel) aoc.Member {
	return aoc.Member{ID: id, Name: name, CompletionDayLevels: completion}
}

func board(members ...aoc.Member) *aoc.Leaderboard {
	lb := &aoc.Leaderboard{Event: "2026", Members: map[string]aoc.Member{}}
	for _, m := range members {
		lb.Members[strconv.Itoa(m.ID)] = m
	}
	return lb
}

func trackerWith(previous, current *aoc.Leaderboard) *Tracker {
	return &Tracker{PreviousLeaderboard: previous, CurrentLeaderboard: current, Config: &config.Config{}}
}

func TestCheckForNewStars_NilLeaderboardsDoNotPanic(t *testing.T) {
	for _, tr := range []*Tracker{trackerWith(nil, board()), trackerWith(board(), nil), trackerWith(nil, nil)} {
		stars, err := tr.CheckForNewStars()
		assert.NoError(t, err)
		assert.Empty(t, stars)
		members, err := tr.CheckForNewMembers()
		assert.NoError(t, err)
		assert.Empty(t, members)
	}
}

func TestCheckForNewStars_OneEventPerPartSortedByTime(t *testing.T) {
	previous := board(member(1, "A", days(-1)), member(2, "B", nil))
	current := board(member(1, "A", days(1, 2)), member(2, "B", days(-1)))

	events, err := trackerWith(previous, current).CheckForNewStars()

	require.NoError(t, err)
	got := make([]string, len(events))
	for i, e := range events {
		got[i] = e.Member.Name + " d" + strconv.Itoa(e.Day) + "p" + strconv.Itoa(e.Part)
	}
	assert.Equal(t, []string{"B d1p1", "A d1p2", "A d2p1", "A d2p2"}, got)
	assert.Equal(t, time.Unix(1000, 0), events[0].At)
}

func TestCheckForNewStars_NewMemberStarsAreNotAnnounced(t *testing.T) {
	events, err := trackerWith(board(), board(member(1, "A", days(1, 2)))).CheckForNewStars()
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestUpdateLeaderboard_FailedPollLeavesNothingToAnnounce(t *testing.T) {
	mockClient := new(MockAOCClient)
	tr := NewTracker(&config.Config{LeaderboardID: "lb"}, board(member(1, "A", nil)), mockClient)
	mockClient.On("GetLeaderboard", "lb").Return(board(member(1, "A", days(-1))), nil).Once()
	mockClient.On("GetLeaderboard", "lb").Return(nil, errors.New("boom")).Once()

	require.NoError(t, tr.UpdateLeaderboard())
	first, _ := tr.CheckForNewStars()
	assert.Len(t, first, 1)

	assert.Error(t, tr.UpdateLeaderboard())
	assert.False(t, tr.LastUpdate.IsZero(), "a failed attempt still counts for rate limiting")
}

func TestDue(t *testing.T) {
	now := time.Now()
	tr := &Tracker{LastUpdate: now.Add(-PollInterval + time.Second)}
	assert.False(t, tr.Due(now))
	tr.LastUpdate = now.Add(-PollInterval)
	assert.True(t, tr.Due(now))
}

func TestCheckForNewMembers_UsesDisplayNameInStableOrder(t *testing.T) {
	current := board(member(3, "", nil), member(2, "B", nil), member(1, "A", nil))
	members, err := trackerWith(board(member(1, "A", nil)), current).CheckForNewMembers()
	require.NoError(t, err)
	assert.Equal(t, []string{"B", "(anonymous user #3)"}, members)
}

func TestFormatLeaderboard_TiesAreStable(t *testing.T) {
	lb := board(aoc.Member{ID: 9, Name: "Z", LocalScore: 10, Stars: 2, LastStarTs: 50},
		aoc.Member{ID: 1, Name: "Y", LocalScore: 10, Stars: 2, LastStarTs: 90},
		aoc.Member{ID: 5, Name: "", LocalScore: 10, Stars: 2, LastStarTs: 50})
	for i := 0; i < 20; i++ {
		assert.Equal(t, "1. (anonymous user #5) - 10 points (2 stars)\n1. Z - 10 points (2 stars)\n1. Y - 10 points (2 stars)\n",
			FormatLeaderboard(lb).Description)
	}
}

func TestFormatStars_SkippedDayStillShowsLaterColumns(t *testing.T) {
	embed := FormatStars(board(member(1, "A", days(1, 3))))
	assert.Equal(t, "```Day  1  2  3\n     ★     ★  A```", embed.Description)
}

func TestStoreAndLoadLeaderboard_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leaderboard.json")
	lb := board(member(1, "A", days(1)))

	require.NoError(t, StoreLeaderboard(lb, path))
	require.NoError(t, StoreLeaderboard(lb, path))
	loaded, modTime, err := LoadLeaderboard(path)

	require.NoError(t, err)
	assert.Equal(t, lb, loaded)
	assert.WithinDuration(t, time.Now(), modTime, time.Minute)
	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1, "no temporary files should be left behind")
}

func TestLoadLeaderboard_Errors(t *testing.T) {
	dir := t.TempDir()
	_, _, err := LoadLeaderboard(filepath.Join(dir, "missing.json"))
	assert.Error(t, err)

	corrupt := filepath.Join(dir, "corrupt.json")
	require.NoError(t, os.WriteFile(corrupt, []byte("{"), 0o600))
	_, _, err = LoadLeaderboard(corrupt)
	assert.ErrorContains(t, err, "unmarshalling")
}
