package leaderboard

import (
	"github.com/PaytonWebber/aoc-discord-bot/internal/aoc"
	"github.com/PaytonWebber/aoc-discord-bot/internal/fileutil"

	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func SortedMembers(leaderboard *aoc.Leaderboard) []aoc.Member {
	members := make([]aoc.Member, 0, len(leaderboard.Members))
	for _, member := range leaderboard.Members {
		members = append(members, member)
	}

	sort.Slice(members, func(i, j int) bool {
		a, b := members[i], members[j]
		if a.LocalScore != b.LocalScore {
			return a.LocalScore > b.LocalScore
		}
		if a.Stars != b.Stars {
			return a.Stars > b.Stars
		}
		if a.LastStarTs != b.LastStarTs {
			return a.LastStarTs < b.LastStarTs
		}
		return a.ID < b.ID
	})

	return members
}

func FormatLeaderboard(leaderboard *aoc.Leaderboard) *discordgo.MessageEmbed {
	if leaderboard == nil || len(leaderboard.Members) == 0 {
		return nil
	}

	var sb strings.Builder

	var prevScore int
	var rank int
	for i, member := range SortedMembers(leaderboard) {
		// Handle ties and first place
		if i == 0 || member.LocalScore < prevScore {
			rank = i + 1
			prevScore = member.LocalScore
		}
		line := fmt.Sprintf("%d. %s - %d points (%d stars)\n", rank, member.DisplayName(), member.LocalScore, member.Stars)
		sb.WriteString(line)
	}

	// Create the embed
	embed := &discordgo.MessageEmbed{
		Title:       "AoC Leaderboard:",
		Description: sb.String(),
		Color:       0x034F20,
	}

	return embed
}

func FormatStars(leaderboard *aoc.Leaderboard) *discordgo.MessageEmbed {
	if leaderboard == nil {
		return nil
	}

	members := SortedMembers(leaderboard)

	var sb strings.Builder

	maxDays := 0
	longestNameLength := 0
	for _, member := range members {
		for dayKey := range member.CompletionDayLevels {
			if day, err := strconv.Atoi(dayKey); err == nil && day > maxDays {
				maxDays = day
			}
		}
		if len(member.DisplayName()) > longestNameLength {
			longestNameLength = len(member.DisplayName())
		}
	}

	sb.WriteString("Day")
	for i := 1; i <= maxDays; i++ {
		sb.WriteString(fmt.Sprintf(" %2d", i))
	}

	for _, member := range members {
		sb.WriteString("\n")
		sb.WriteString("    ")
		for i := 1; i <= maxDays; i++ {
			stars := 0
			day, exists := member.CompletionDayLevels[fmt.Sprint(i)]
			if exists {
				if day.Level2 != nil {
					stars++
				}
				if day.Level1 != nil {
					stars++
				}
			}

			switch stars {
			case 2:
				sb.WriteString(" ★ ")
			case 1:
				sb.WriteString(" ☆ ")
			default:
				sb.WriteString("   ")
			}
		}
		sb.WriteString(fmt.Sprintf(" %-*s", longestNameLength, member.DisplayName()))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "AoC Stars:",
		Description: "```" + sb.String() + "```",
		Color:       0xB22222,
	}

	return embed
}

func StoreLeaderboard(leaderboard *aoc.Leaderboard, path string) error {
	log.Println("Storing leaderboard")
	leaderboardJson, err := json.Marshal(leaderboard)
	if err != nil {
		return fmt.Errorf("error marshalling leaderboard: %w", err)
	}
	return fileutil.WriteFileAtomic(path, leaderboardJson)
}

func LoadLeaderboard(path string) (*aoc.Leaderboard, time.Time, error) {
	log.Println("Getting leaderboard from file")
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("error reading leaderboard file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("error reading leaderboard file: %w", err)
	}

	var lb aoc.Leaderboard
	if err := json.Unmarshal(data, &lb); err != nil {
		return nil, time.Time{}, fmt.Errorf("error unmarshalling leaderboard: %w", err)
	}

	return &lb, info.ModTime(), nil
}
