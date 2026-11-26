package main

import (
	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/discord"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"
	"github.com/PaytonWebber/aoc-discord-bot/internal/scheduler"
	"github.com/PaytonWebber/aoc-discord-bot/internal/state"

	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg := loadConfig()

	store := loadState(cfg)

	session := createDiscordSession(cfg)

	tracker := initTracker(cfg)

	bot := discord.NewBotHandler(session, tracker, cfg, store)

	session.AddHandler(bot.MessageReceived)

	bot.RefreshLeaderboardMessage()

	go pollForUpdates(bot)

	stop := make(chan struct{})
	go newUnlockScheduler(cfg, store, bot).Run(stop)

	waitForShutdown(session)
	close(stop)
}

func loadState(cfg *config.Config) *state.Store {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("error creating DATA_DIR %s: %v", cfg.DataDir, err)
	}
	store, err := state.Load(cfg.StatePath(), cfg.AOCYear)
	if err != nil {
		log.Fatalf("error loading state: %v", err)
	}
	return store
}

func newUnlockScheduler(cfg *config.Config, store *state.Store, bot *discord.BotHandler) *scheduler.Scheduler {
	return &scheduler.Scheduler{
		Year:          cfg.AOCYear,
		Days:          cfg.AOCDays,
		LastAnnounced: func() int { return store.Get().LastAnnouncedDay },
		Announce:      bot.AnnounceDay,
		MarkAnnounced: func(day int) error {
			return store.Update(func(s *state.State) { s.LastAnnouncedDay = day })
		},
	}
}

func loadConfig() *config.Config {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("cfg is nil")
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration validation failed: %v", err)
	}
	log.Printf("Using Advent of Code year %d (%d days), time zone %s, data in %s", cfg.AOCYear, cfg.AOCDays, cfg.TimeZone, cfg.DataDir)
	if cfg.Contact == "" {
		log.Printf("CONTACT is not set; Advent of Code asks automated tools to include contact info in their User-Agent")
	}
	return cfg
}

func createDiscordSession(cfg *config.Config) *discordgo.Session {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		log.Fatalf("error creating discord session: %v", err)
	}
	session.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentMessageContent
	err = session.Open()
	if err != nil {
		log.Fatalf("error opening connection to discord: %v (if this mentions disallowed intents or 4014, enable MESSAGE CONTENT INTENT in the Developer Portal)", err)
	}
	return session
}

func initTracker(cfg *config.Config) *leaderboard.Tracker {
	client := aoc.NewClient(cfg.SessionCookie, cfg.AOCYear)
	client.UserAgent = aoc.UserAgent(cfg.Contact)

	stored, lastUpdate := loadStoredLeaderboard(cfg)
	if stored == nil {
		log.Printf("getting leaderboard from AoC")
		lastUpdate = time.Now()
		var err error
		stored, err = client.GetLeaderboard(cfg.LeaderboardID)
		if err != nil {
			log.Printf("error getting leaderboard: %v", err)
		} else if err := leaderboard.StoreLeaderboard(stored, cfg.LeaderboardPath()); err != nil {
			log.Printf("error storing leaderboard: %v", err)
		}
	}

	tracker := leaderboard.NewTracker(cfg, stored, client)
	tracker.LastUpdate = lastUpdate
	return tracker
}

func loadStoredLeaderboard(cfg *config.Config) (*aoc.Leaderboard, time.Time) {
	stored, modTime, err := leaderboard.LoadLeaderboard(cfg.LeaderboardPath())
	if err != nil {
		log.Printf("no usable stored leaderboard: %v", err)
		return nil, time.Time{}
	}
	if stored.Event != strconv.Itoa(cfg.AOCYear) {
		log.Printf("stored leaderboard is for %s, not %d; ignoring it", stored.Event, cfg.AOCYear)
		return nil, time.Time{}
	}
	return stored, modTime
}

func pollForUpdates(bot *discord.BotHandler) {
	checkForUpdates(bot)
	for range time.Tick(time.Minute) {
		checkForUpdates(bot)
	}
}

func checkForUpdates(bot *discord.BotHandler) {
	hadUpdates, err := bot.CheckForUpdates()
	switch {
	case errors.Is(err, discord.ErrTooSoon):
	case err != nil:
		log.Printf("error checking for updates: %v", err)
	case !hadUpdates:
		log.Printf("no updates")
	}
}

func waitForShutdown(session *discordgo.Session) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	log.Printf("Shutting down...")
	session.Close()
	log.Printf("Session closed")
}
