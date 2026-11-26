package main

import (
	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/discord"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"

	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
)

const leaderboardFile = "leaderboard.json"

func main() {
	cfg := loadConfig()

	session := createDiscordSession(cfg)

	tracker := initTracker(cfg)

	bot := discord.NewBotHandler(session, tracker, cfg, leaderboardFile)

	session.AddHandler(bot.MessageReceived)

	go pollForUpdates(bot)

	waitForShutdown(session)
}

func loadConfig() *config.Config {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("cfg is nil")
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration validation failed: %v", err)
	}
	log.Printf("Using Advent of Code year: %d", cfg.AOCYear)
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
		log.Fatalf("error opening connection to discord (is MESSAGE CONTENT INTENT enabled in the Developer Portal?): %v", err)
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
		} else if err := leaderboard.StoreLeaderboard(stored, leaderboardFile); err != nil {
			log.Printf("error storing leaderboard: %v", err)
		}
	}

	tracker := leaderboard.NewTracker(cfg, stored, client)
	tracker.LastUpdate = lastUpdate
	return tracker
}

func loadStoredLeaderboard(cfg *config.Config) (*aoc.Leaderboard, time.Time) {
	stored, modTime, err := leaderboard.LoadLeaderboard(leaderboardFile)
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
