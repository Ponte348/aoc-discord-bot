package discord

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMessenger struct {
	mu       sync.Mutex
	messages []string
	embeds   []*discordgo.MessageEmbed
}

func (f *fakeMessenger) ChannelMessageSend(_ string, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, content)
	return &discordgo.Message{}, nil
}

func (f *fakeMessenger) ChannelMessageSendEmbed(_ string, embed *discordgo.MessageEmbed, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embeds = append(f.embeds, embed)
	return &discordgo.Message{}, nil
}

type scriptedClient struct {
	calls     atomic.Int32
	responses []func() (*aoc.Leaderboard, error)
}

func (c *scriptedClient) GetLeaderboard(string) (*aoc.Leaderboard, error) {
	i := int(c.calls.Add(1)) - 1
	time.Sleep(5 * time.Millisecond)
	return c.responses[i]()
}

func ok(lb *aoc.Leaderboard) func() (*aoc.Leaderboard, error) {
	return func() (*aoc.Leaderboard, error) { return lb, nil }
}

func fail() (*aoc.Leaderboard, error) { return nil, errors.New("aoc is down") }

func boardWithPart(parts int) *aoc.Leaderboard {
	level := aoc.CompletionDayLevel{}
	if parts >= 1 {
		level.Level1 = &aoc.StarDetail{GetStarTs: 1000}
	}
	if parts >= 2 {
		level.Level2 = &aoc.StarDetail{GetStarTs: 2000}
	}
	return &aoc.Leaderboard{Event: "2026", Members: map[string]aoc.Member{
		"1": {ID: 1, Name: "Alice", Stars: parts, LocalScore: parts, CompletionDayLevels: map[string]aoc.CompletionDayLevel{"1": level}},
	}}
}

func newTestBot(t *testing.T, client leaderboard.AOCClient, stored *aoc.Leaderboard) (*BotHandler, *fakeMessenger) {
	cfg := &config.Config{ChannelID: "channel", LeaderboardID: "lb"}
	messenger := &fakeMessenger{}
	tracker := leaderboard.NewTracker(cfg, stored, client)
	return NewBotHandler(messenger, tracker, cfg, filepath.Join(t.TempDir(), "leaderboard.json")), messenger
}

func TestCheckForUpdates_AnnouncesEachPart(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(2))}}, boardWithPart(0))

	hadUpdates, err := bot.CheckForUpdates()

	require.NoError(t, err)
	assert.True(t, hadUpdates)
	assert.Equal(t, []string{"Alice got a star! 🌟 (day 1, part 1)", "Alice got a star! 🌟 (day 1, part 2)"}, messenger.messages)
	assert.Len(t, messenger.embeds, 1)
}

func TestCheckForUpdates_FailedPollDoesNotRepeatAnnouncements(t *testing.T) {
	client := &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(1)), fail}}
	bot, messenger := newTestBot(t, client, boardWithPart(0))

	_, err := bot.CheckForUpdates()
	require.NoError(t, err)
	announced := len(messenger.messages)

	bot.Tracker.LastUpdate = time.Time{}
	hadUpdates, err := bot.CheckForUpdates()

	assert.Error(t, err)
	assert.False(t, hadUpdates)
	assert.Len(t, messenger.messages, announced, "a failed poll must not announce anything")
}

func TestCheckForUpdates_RespectsPollInterval(t *testing.T) {
	client := &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(0))}}
	bot, _ := newTestBot(t, client, boardWithPart(0))
	bot.Tracker.LastUpdate = time.Now()

	_, err := bot.CheckForUpdates()

	assert.ErrorIs(t, err, ErrTooSoon)
	assert.Equal(t, int32(0), client.calls.Load())
}

func TestCheckForUpdates_ConcurrentCallsFetchOnce(t *testing.T) {
	client := &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(1))}}
	bot, messenger := newTestBot(t, client, boardWithPart(0))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bot.CheckForUpdates()
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), client.calls.Load())
	assert.Len(t, messenger.messages, 1)
}

func TestCheckForUpdates_NoStoredLeaderboardDoesNotPanic(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(2))}}, nil)

	hadUpdates, err := bot.CheckForUpdates()

	require.NoError(t, err)
	assert.False(t, hadUpdates)
	assert.Empty(t, messenger.messages)
}

func TestHandleUpdateCommand_Replies(t *testing.T) {
	client := &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(0)), fail}}
	bot, messenger := newTestBot(t, client, boardWithPart(0))

	bot.HandleUpdateCommand()
	bot.HandleUpdateCommand()
	bot.Tracker.LastUpdate = time.Time{}
	bot.HandleUpdateCommand()

	assert.Equal(t, []string{"No updates", "You can only update once every 15 minutes", "Could not reach Advent of Code, try again later"}, messenger.messages)
}

func TestSendChannelMessageEmbed_NilEmbed(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{}, nil)
	bot.SendChannelMessageEmbed("channel", nil)
	assert.Empty(t, messenger.embeds)
	assert.Equal(t, []string{"The leaderboard is empty"}, messenger.messages)
}
