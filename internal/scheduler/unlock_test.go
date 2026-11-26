package scheduler

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnlockTime(t *testing.T) {
	assert.Equal(t, "2026-12-01T05:00:00Z", UnlockTime(2026, 1).Format(time.RFC3339))
}

func TestNextUnlock(t *testing.T) {
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, time.December, day, hour, minute, 0, 0, time.UTC)
	}
	tests := []struct {
		name          string
		now           time.Time
		lastAnnounced int
		wantDay       int
		wantOK        bool
	}{
		{"before the event", time.Date(2026, time.November, 26, 12, 0, 0, 0, time.UTC), 0, 1, true},
		{"one second before unlock", at(3, 4, 59).Add(59 * time.Second), 2, 3, true},
		{"exactly at unlock", at(3, 5, 0), 2, 3, true},
		{"within grace after unlock", at(3, 6, 0), 2, 3, true},
		{"missed day is skipped", at(3, 6, 1), 2, 4, true},
		{"restart with empty state mid-event", at(5, 12, 0), 0, 6, true},
		{"already announced today", at(3, 5, 30), 3, 4, true},
		{"last day announced", at(12, 6, 0), 12, 0, false},
		{"after the event", at(20, 0, 0), 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, when, ok := NextUnlock(tt.now, 2026, 12, tt.lastAnnounced)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantDay, day)
			if ok {
				assert.Equal(t, UnlockTime(2026, day), when)
			}
		})
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d > 0 {
		c.now = c.now.Add(d)
	}
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

func TestRun_AnnouncesEachDayOnceThenStops(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.November, 30, 0, 0, 0, 0, time.UTC)}
	var announced []int
	last := 0
	failOnce := true

	s := &Scheduler{
		Year:          2026,
		Days:          3,
		LastAnnounced: func() int { return last },
		Announce: func(day int) error {
			if day == 2 && failOnce {
				failOnce = false
				return errors.New("discord down")
			}
			announced = append(announced, day)
			return nil
		},
		MarkAnnounced: func(day int) error { last = day; return nil },
		Now:           clock.Now,
		After:         clock.After,
	}

	done := make(chan struct{})
	go func() {
		s.Run(make(chan struct{}))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not finish")
	}
	assert.Equal(t, []int{1, 2, 3}, announced)
	assert.Equal(t, 3, last)
}

func TestRun_StopsWhenAsked(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	s := &Scheduler{
		Year:          2026,
		Days:          12,
		LastAnnounced: func() int { return 0 },
		Announce:      func(int) error { t.Fatal("should not announce"); return nil },
		MarkAnnounced: func(int) error { return nil },
		Now:           func() time.Time { return time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC) },
		After:         func(time.Duration) <-chan time.Time { return make(chan time.Time) },
	}
	s.Run(stop)
}
