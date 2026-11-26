package aoc

import "fmt"

type StarDetail struct {
	GetStarTs int `json:"get_star_ts"`
	StarIndex int `json:"star_index"`
}

type CompletionDayLevel struct {
	Level1 *StarDetail `json:"1"`
	Level2 *StarDetail `json:"2"`
}

type Member struct {
	ID                  int                           `json:"id"`
	LastStarTs          int                           `json:"last_star_ts"`
	GlobalScore         int                           `json:"global_score"`
	LocalScore          int                           `json:"local_score"`
	Name                string                        `json:"name"`
	CompletionDayLevels map[string]CompletionDayLevel `json:"completion_day_level"`
	Stars               int                           `json:"stars"`
}

func (m Member) DisplayName() string {
	if m.Name == "" {
		return fmt.Sprintf("(anonymous user #%d)", m.ID)
	}
	return m.Name
}

// Leaderboard represents the entire JSON structure
type Leaderboard struct {
	Members map[string]Member `json:"members"`
	Event   string            `json:"event"`
	OwnerID int               `json:"owner_id"`
}
