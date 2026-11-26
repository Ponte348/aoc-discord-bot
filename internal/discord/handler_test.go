package discord

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"
	"github.com/PaytonWebber/aoc-discord-bot/internal/state"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMessenger struct {
	mu       sync.Mutex
	messages []string
	embeds   []*discordgo.MessageEmbed
	edits    []string
	pins     []string
	editErr  error
	nextID   int
}

func (f *fakeMessenger) ChannelMessageEditEmbed(_ string, messageID string, embed *discordgo.MessageEmbed, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.editErr != nil {
		return nil, f.editErr
	}
	f.edits = append(f.edits, messageID)
	return &discordgo.Message{ID: messageID}, nil
}

func (f *fakeMessenger) ChannelMessagePin(_ string, messageID string, _ ...discordgo.RequestOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pins = append(f.pins, messageID)
	return nil
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
	f.nextID++
	return &discordgo.Message{ID: "msg-" + strconv.Itoa(f.nextID)}, nil
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
	cfg := &config.Config{ChannelID: "channel", LeaderboardID: "lb", AOCYear: 2026, Location: time.UTC, DataDir: t.TempDir()}
	store, err := state.Load(cfg.StatePath(), cfg.AOCYear)
	require.NoError(t, err)
	messenger := &fakeMessenger{}
	tracker := leaderboard.NewTracker(cfg, stored, client)
	return NewBotHandler(messenger, tracker, cfg, store), messenger
}

func TestCheckForUpdates_AnnouncesEachPart(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(2))}}, boardWithPart(0))

	hadUpdates, err := bot.CheckForUpdates()

	require.NoError(t, err)
	assert.True(t, hadUpdates)
	assert.Equal(t, []string{
		"🌟 **Alice** completed **Day 1 – Part 1** at 00:16 on 1 Jan",
		"🌟 **Alice** completed **Day 1 – Part 2** at 00:33 on 1 Jan",
	}, messenger.messages)
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

func TestLeaderboardMessage_PostedPinnedThenEdited(t *testing.T) {
	client := &scriptedClient{responses: []func() (*aoc.Leaderboard, error){ok(boardWithPart(1)), ok(boardWithPart(2))}}
	bot, messenger := newTestBot(t, client, boardWithPart(0))

	_, err := bot.CheckForUpdates()
	require.NoError(t, err)
	bot.Tracker.LastUpdate = time.Time{}
	_, err = bot.CheckForUpdates()
	require.NoError(t, err)

	assert.Len(t, messenger.embeds, 1, "only one leaderboard message is ever posted")
	assert.Equal(t, []string{"msg-1"}, messenger.pins)
	assert.Equal(t, []string{"msg-1"}, messenger.edits)
	assert.Equal(t, "msg-1", bot.state.Get().LeaderboardMessageID)
	assert.Contains(t, messenger.embeds[0].Footer.Text, "Updated ")

	reloaded, err := state.Load(bot.cfg.StatePath(), 2026)
	require.NoError(t, err)
	assert.Equal(t, "msg-1", reloaded.Get().LeaderboardMessageID, "message id survives a restart")
}

func TestLeaderboardMessage_DeletedMessageIsRecreated(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{}, boardWithPart(1))
	require.NoError(t, bot.state.Update(func(s *state.State) { s.LeaderboardMessageID = "gone" }))
	messenger.editErr = &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownMessage}}

	bot.RefreshLeaderboardMessage()

	assert.Len(t, messenger.embeds, 1)
	assert.Equal(t, "msg-1", bot.state.Get().LeaderboardMessageID)
}

func TestLeaderboardMessage_TransientEditErrorDoesNotDuplicate(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{}, boardWithPart(1))
	require.NoError(t, bot.state.Update(func(s *state.State) { s.LeaderboardMessageID = "existing" }))
	messenger.editErr = errors.New("discord 502")

	bot.RefreshLeaderboardMessage()

	assert.Empty(t, messenger.embeds)
	assert.Equal(t, "existing", bot.state.Get().LeaderboardMessageID)
}

func TestLeaderboardMessage_EmptyLeaderboardPostsNothing(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{}, nil)
	bot.RefreshLeaderboardMessage()
	assert.Empty(t, messenger.embeds)
}

func TestAnnounceDay(t *testing.T) {
	bot, messenger := newTestBot(t, &scriptedClient{}, nil)

	require.NoError(t, bot.AnnounceDay(3))
	bot.cfg.PingRoleID = "42"
	require.NoError(t, bot.AnnounceDay(4))

	assert.Equal(t, []string{
		"🎄 **Day 3** of Advent of Code 2026 is open! https://adventofcode.com/2026/day/3",
		"<@&42> 🎄 **Day 4** of Advent of Code 2026 is open! https://adventofcode.com/2026/day/4",
	}, messenger.messages)
}

func TestFormatStar_UsesConfiguredTimeZone(t *testing.T) {
	bot, _ := newTestBot(t, &scriptedClient{}, nil)
	lisbon, err := time.LoadLocation("Europe/Lisbon")
	require.NoError(t, err)
	bot.cfg.Location = lisbon

	star := leaderboard.StarEvent{Member: aoc.Member{ID: 1, Name: "Alice"}, Day: 2, Part: 1,
		At: time.Date(2026, time.December, 2, 5, 12, 0, 0, time.UTC)}

	assert.Equal(t, "🌟 **Alice** completed **Day 2 – Part 1** at 05:12 on 2 Dec", bot.formatStar(star))
}
