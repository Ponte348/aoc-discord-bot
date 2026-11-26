package discord

import (
	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"
	"github.com/bwmarrin/discordgo"

	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

var ErrTooSoon = errors.New("updates are limited to once every 15 minutes")

type Messenger interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

type BotHandler struct {
	Session   Messenger
	Tracker   *leaderboard.Tracker
	cfg       *config.Config
	storePath string
	mu        sync.Mutex
}

func NewBotHandler(session Messenger, tracker *leaderboard.Tracker, cfg *config.Config, storePath string) *BotHandler {
	return &BotHandler{
		Session:   session,
		Tracker:   tracker,
		cfg:       cfg,
		storePath: storePath,
	}
}

func (bh *BotHandler) CheckForUpdates() (bool, error) {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	if !bh.Tracker.Due(time.Now()) {
		return false, ErrTooSoon
	}

	log.Println("Checking for updates...")
	if err := bh.Tracker.UpdateLeaderboard(); err != nil {
		return false, fmt.Errorf("error updating leaderboard: %w", err)
	}
	if err := leaderboard.StoreLeaderboard(bh.Tracker.CurrentLeaderboard, bh.storePath); err != nil {
		log.Printf("error storing leaderboard: %v", err)
	}

	newStars, err := bh.Tracker.CheckForNewStars()
	if err != nil {
		return false, err
	}

	newMembers, err := bh.Tracker.CheckForNewMembers()
	if err != nil {
		return false, err
	}

	for _, star := range newStars {
		bh.SendChannelMessage(bh.cfg.ChannelID, fmt.Sprintf("%s got a star! 🌟 (day %d, part %d)", star.Member.DisplayName(), star.Day, star.Part))
	}

	if len(newMembers) > 0 {
		log.Printf("new members: %v", newMembers)
		bh.SendChannelMessage(bh.cfg.ChannelID, "CHALLENGER APPROACHING!")
		for _, member := range newMembers {
			bh.SendChannelMessage(bh.cfg.ChannelID, member+" has joined the leaderboard!")
		}
	}

	hadUpdates := len(newStars) > 0 || len(newMembers) > 0
	if hadUpdates {
		bh.SendChannelMessageEmbed(bh.cfg.ChannelID, leaderboard.FormatLeaderboard(bh.Tracker.CurrentLeaderboard))
	}

	return hadUpdates, nil
}

func (bh *BotHandler) MessageReceived(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || m.ChannelID != bh.cfg.ChannelID {
		return
	}

	switch strings.ToLower(strings.TrimSpace(m.Content)) {
	case "!update":
		log.Println("Update command received")
		bh.HandleUpdateCommand()
	case "!leaderboard":
		log.Println("Leaderboard command received")
		bh.SendChannelMessageEmbed(bh.cfg.ChannelID, bh.withLock(leaderboard.FormatLeaderboard))
	case "!stars":
		log.Println("Stars command received")
		bh.SendChannelMessageEmbed(bh.cfg.ChannelID, bh.withLock(leaderboard.FormatStars))
	case "!help":
		sb := strings.Builder{}
		sb.WriteString("```")
		sb.WriteString("Commands:\n\n")
		sb.WriteString("!leaderboard - Shows the current leaderboard\n\n")
		sb.WriteString("!update - Checks for updates and shows the updated leaderboard\n\n")
		sb.WriteString("!stars - Shows the current stars\n\n")
		sb.WriteString("!help - Shows this message\n")
		sb.WriteString("```")
		bh.SendChannelMessage(bh.cfg.ChannelID, sb.String())
	}
}

func (bh *BotHandler) HandleUpdateCommand() {
	hadUpdates, err := bh.CheckForUpdates()
	switch {
	case errors.Is(err, ErrTooSoon):
		bh.SendChannelMessage(bh.cfg.ChannelID, "You can only update once every 15 minutes")
	case err != nil:
		log.Printf("error checking for updates: %v", err)
		bh.SendChannelMessage(bh.cfg.ChannelID, "Could not reach Advent of Code, try again later")
	case !hadUpdates:
		bh.SendChannelMessage(bh.cfg.ChannelID, "No updates")
	}
}

func (bh *BotHandler) withLock(format func(*aoc.Leaderboard) *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	bh.mu.Lock()
	defer bh.mu.Unlock()
	return format(bh.Tracker.CurrentLeaderboard)
}

func (bh *BotHandler) SendChannelMessage(channelID, message string) {
	_, err := bh.Session.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Printf("error sending message: %v", err)
	}
}

func (bh *BotHandler) SendChannelMessageEmbed(channelID string, embed *discordgo.MessageEmbed) {
	if embed == nil {
		bh.SendChannelMessage(channelID, "The leaderboard is empty")
		return
	}
	_, err := bh.Session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Printf("error sending message: %v", err)
	}
}
