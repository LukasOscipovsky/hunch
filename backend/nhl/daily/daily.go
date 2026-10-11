// Package daily runs NHL data refreshes once a day.
package daily

import (
	"context"
	"log"
	"time"
)

// refreshHourUTC is when the daily refresh runs. 10:00 UTC is after the last
// North American games have finished.
const refreshHourUTC = 10

// Refresher is a data set that can be refreshed from the NHL API.
type Refresher interface {
	// RefreshIfEmpty refreshes only when nothing is stored, and reports
	// whether it did.
	RefreshIfEmpty(ctx context.Context) (bool, error)
	Refresh(ctx context.Context) error
}

// Run refreshes r once immediately if nothing is stored, then every day at
// refreshHourUTC, until ctx is cancelled. Each refresh is bounded by timeout,
// and name prefixes log lines.
func Run(ctx context.Context, name string, r Refresher, timeout time.Duration) {
	initCtx, cancel := context.WithTimeout(ctx, timeout)
	if _, err := r.RefreshIfEmpty(initCtx); err != nil {
		log.Printf("%s: initial refresh: %v", name, err)
	}
	cancel()
	for {
		timer := time.NewTimer(time.Until(nextRefresh(time.Now())))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			refreshAndLog(ctx, name, r, timeout)
		}
	}
}

func refreshAndLog(ctx context.Context, name string, r Refresher, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.Refresh(ctx); err != nil {
		log.Printf("%s: refresh: %v", name, err)
		return
	}
	log.Printf("%s: refreshed", name)
}

// nextRefresh returns the next refreshHourUTC after now.
func nextRefresh(now time.Time) time.Time {
	now = now.UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), refreshHourUTC, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
