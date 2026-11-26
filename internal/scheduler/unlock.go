package scheduler

import (
	"log"
	"time"
)

const (
	unlockHourUTC = 5
	MissedGrace   = time.Hour
	retryDelay    = time.Minute
)

func UnlockTime(year, day int) time.Time {
	return time.Date(year, time.December, day, unlockHourUTC, 0, 0, 0, time.UTC)
}

// NextUnlock returns the first day after lastAnnounced that is still worth announcing.
// Days that unlocked more than MissedGrace ago (e.g. while the bot was down) are skipped.
func NextUnlock(now time.Time, year, days, lastAnnounced int) (int, time.Time, bool) {
	for day := lastAnnounced + 1; day <= days; day++ {
		at := UnlockTime(year, day)
		if now.Sub(at) <= MissedGrace {
			return day, at, true
		}
	}
	return 0, time.Time{}, false
}

type Scheduler struct {
	Year          int
	Days          int
	LastAnnounced func() int
	Announce      func(day int) error
	MarkAnnounced func(day int) error
	Now           func() time.Time
	After         func(time.Duration) <-chan time.Time
}

func (s *Scheduler) Run(stop <-chan struct{}) {
	now, after := s.Now, s.After
	if now == nil {
		now = time.Now
	}
	if after == nil {
		after = time.After
	}

	for {
		day, at, ok := NextUnlock(now(), s.Year, s.Days, s.LastAnnounced())
		if !ok {
			log.Printf("no more puzzles to announce for %d", s.Year)
			return
		}
		log.Printf("next puzzle announcement: day %d at %s", day, at.Format(time.RFC3339))

		select {
		case <-stop:
			return
		case <-after(at.Sub(now())):
		}
		if now().Before(at) {
			continue
		}

		if err := s.Announce(day); err != nil {
			log.Printf("error announcing day %d: %v", day, err)
			select {
			case <-stop:
				return
			case <-after(retryDelay):
			}
			continue
		}
		if err := s.MarkAnnounced(day); err != nil {
			log.Printf("error saving announced day %d: %v", day, err)
		}
	}
}
