package aoc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultUserAgent = "github.com/PaytonWebber/aoc-discord-bot"
	requestTimeout   = 30 * time.Second
)

type Client struct {
	SessionCookie string
	HTTPClient    *http.Client
	Year          int
	UserAgent     string
}

// NewClient creates a new AOC client with the provided session cookie and year.
func NewClient(sessionCookie string, year int) *Client {
	return &Client{
		SessionCookie: sessionCookie,
		HTTPClient: &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Year:      year,
		UserAgent: defaultUserAgent,
	}
}

func UserAgent(contact string) string {
	if contact == "" {
		return defaultUserAgent
	}
	return defaultUserAgent + " by " + contact
}

// For testing purposes, SetHTTPClient allows you to set the HTTP client used by the client.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.HTTPClient = client
}

func (c *Client) GetLeaderboard(leaderboardID string) (*Leaderboard, error) {
	url := fmt.Sprintf("https://adventofcode.com/%d/leaderboard/private/view/%s.json", c.Year, leaderboardID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("cookie", fmt.Sprintf("session=%s", c.SessionCookie))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from Advent of Code (is SESSION_COOKIE valid and are you a member of leaderboard %s?)", resp.StatusCode, leaderboardID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var leaderboard Leaderboard
	err = json.Unmarshal(body, &leaderboard)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response body: %w", err)
	}

	return &leaderboard, nil
}
