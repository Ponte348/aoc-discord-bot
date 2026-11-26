package discord

import (
	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/config"
	"github.com/PaytonWebber/aoc-discord-bot/internal/leaderboard"
	"github.com/PaytonWebber/aoc-discord-bot/internal/state"
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
	ChannelMessageEditEmbed(channelID, messageID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessagePin(channelID, messageID string, options ...discordgo.RequestOption) error
}

type BotHandler struct {
	Session Messenger
	Tracker *leaderboard.Tracker
	cfg     *config.Config
	state   *state.Store
	mu      sync.Mutex
}

func NewBotHandler(session Messenger, tracker *leaderboard.Tracker, cfg *config.Config, store *state.Store) *BotHandler {
	return &BotHandler{
		Session: session,
		Tracker: tracker,
		cfg:     cfg,
		state:   store,
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
	if err := leaderboard.StoreLeaderboard(bh.Tracker.CurrentLeaderboard, bh.cfg.LeaderboardPath()); err != nil {
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
		bh.SendChannelMessage(bh.cfg.ChannelID, bh.formatStar(star))
	}

	if len(newMembers) > 0 {
		log.Printf("new members: %v", newMembers)
		bh.SendChannelMessage(bh.cfg.ChannelID, "CHALLENGER APPROACHING!")
		for _, member := range newMembers {
			bh.SendChannelMessage(bh.cfg.ChannelID, member+" has joined the leaderboard!")
		}
	}

	bh.refreshLeaderboardMessage()

	return len(newStars) > 0 || len(newMembers) > 0, nil
}

func (bh *BotHandler) formatStar(star leaderboard.StarEvent) string {
	return fmt.Sprintf("🌟 **%s** completed **Day %d – Part %d** at %s",
		star.Member.DisplayName(), star.Day, star.Part, star.At.In(bh.cfg.Location).Format("15:04 on 2 Jan"))
}

func (bh *BotHandler) RefreshLeaderboardMessage() {
	bh.mu.Lock()
	defer bh.mu.Unlock()
	bh.refreshLeaderboardMessage()
}

func (bh *BotHandler) refreshLeaderboardMessage() {
	embed := leaderboard.FormatLeaderboard(bh.Tracker.CurrentLeaderboard)
	if embed == nil {
		return
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "Updated " + bh.Tracker.LastUpdate.In(bh.cfg.Location).Format("2 Jan 15:04 MST"),
	}

	if id := bh.state.Get().LeaderboardMessageID; id != "" {
		_, err := bh.Session.ChannelMessageEditEmbed(bh.cfg.ChannelID, id, embed)
		if err == nil {
			return
		}
		if !isUnknownMessage(err) {
			log.Printf("error editing leaderboard message: %v", err)
			return
		}
		log.Printf("leaderboard message %s was deleted, posting a new one", id)
	}

	msg, err := bh.Session.ChannelMessageSendEmbed(bh.cfg.ChannelID, embed)
	if err != nil {
		log.Printf("error posting leaderboard message: %v", err)
		return
	}
	if err := bh.Session.ChannelMessagePin(bh.cfg.ChannelID, msg.ID); err != nil {
		log.Printf("error pinning leaderboard message (does the bot have the Pin Messages permission?): %v", err)
	}
	if err := bh.state.Update(func(s *state.State) { s.LeaderboardMessageID = msg.ID }); err != nil {
		log.Printf("error saving leaderboard message id: %v", err)
	}
}

func isUnknownMessage(err error) bool {
	var restErr *discordgo.RESTError
	return errors.As(err, &restErr) && restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeUnknownMessage
}

func (bh *BotHandler) AnnounceDay(day int) error {
	message := fmt.Sprintf("🎄 **Day %d** of Advent of Code %d is open! https://adventofcode.com/%d/day/%d",
		day, bh.cfg.AOCYear, bh.cfg.AOCYear, day)
	if bh.cfg.PingRoleID != "" {
		message = "<@&" + bh.cfg.PingRoleID + "> " + message
	}
	_, err := bh.Session.ChannelMessageSend(bh.cfg.ChannelID, message)
	return err
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
		sb.WriteString("!leaderboard - Shows the current leaderboard (a live copy is pinned in this channel)\n\n")
		sb.WriteString("!update - Checks for updates now (at most once every 15 minutes)\n\n")
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
