package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultDays     = 12
	defaultTimeZone = "Europe/Lisbon"
	defaultDataDir  = "."
)

type Config struct {
	LeaderboardID string
	SessionCookie string
	DiscordToken  string
	ChannelID     string
	AOCYear       int
	Contact       string
	AOCDays       int
	PingRoleID    string
	TimeZone      string
	Location      *time.Location
	DataDir       string
}

func NewConfig() *Config {
	// Default to current year if AOC_YEAR is not set
	year := time.Now().Year()
	if yearStr := os.Getenv("AOC_YEAR"); yearStr != "" {
		if parsedYear, err := strconv.Atoi(yearStr); err == nil && parsedYear >= 2015 {
			year = parsedYear
		}
	}

	days := defaultDays
	if daysStr := os.Getenv("AOC_DAYS"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil {
			days = parsedDays
		}
	}

	timeZone := envOr("TIMEZONE", defaultTimeZone)
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		location = nil
	}

	return &Config{
		LeaderboardID: os.Getenv("LEADERBOARD_ID"),
		SessionCookie: os.Getenv("SESSION_COOKIE"),
		DiscordToken:  os.Getenv("DISCORD_TOKEN"),
		ChannelID:     os.Getenv("CHANNEL_ID"),
		AOCYear:       year,
		Contact:       os.Getenv("CONTACT"),
		AOCDays:       days,
		PingRoleID:    os.Getenv("PING_ROLE_ID"),
		TimeZone:      timeZone,
		Location:      location,
		DataDir:       envOr("DATA_DIR", defaultDataDir),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (c *Config) LeaderboardPath() string {
	return filepath.Join(c.DataDir, "leaderboard.json")
}

func (c *Config) StatePath() string {
	return filepath.Join(c.DataDir, "state.json")
}

// Validate checks that all required configuration values are present.
func (c *Config) Validate() error {
	if c.LeaderboardID == "" {
		return fmt.Errorf("LEADERBOARD_ID environment variable is required")
	}
	if c.SessionCookie == "" {
		return fmt.Errorf("SESSION_COOKIE environment variable is required")
	}
	if c.DiscordToken == "" {
		return fmt.Errorf("DISCORD_TOKEN environment variable is required")
	}
	if c.ChannelID == "" {
		return fmt.Errorf("CHANNEL_ID environment variable is required")
	}
	if c.AOCYear < 2015 {
		return fmt.Errorf("AOC_YEAR must be 2015 or later (Advent of Code started in 2015)")
	}
	if c.AOCDays < 1 || c.AOCDays > 25 {
		return fmt.Errorf("AOC_DAYS must be between 1 and 25")
	}
	if c.Location == nil {
		return fmt.Errorf("TIMEZONE %q is not a valid IANA time zone (e.g. Europe/Lisbon)", c.TimeZone)
	}
	return nil
}
