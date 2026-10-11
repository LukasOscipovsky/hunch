package rosters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"hunch/backend/nhl"
	"hunch/backend/nhl/daily"
)

const (
	// maxConcurrent limits parallel roster requests to the NHL API.
	maxConcurrent = 4
	// refreshTimeout bounds one refresh. The NHL API rate limits after ~25
	// requests and a refresh makes 33, so it can take over a minute.
	refreshTimeout = 5 * time.Minute
)

// TeamRoster is one team's roster as returned by the NHL API.
type TeamRoster struct {
	Team   string          `json:"team"`
	Roster json.RawMessage `json:"roster,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Store persists rosters between refreshes.
type Store interface {
	SaveRosters(ctx context.Context, teams []TeamRoster) error
	LoadRosters(ctx context.Context) ([]TeamRoster, error)
}

type standingsResponse struct {
	Standings []struct {
		TeamAbbrev struct {
			Default string `json:"default"`
		} `json:"teamAbbrev"`
	} `json:"standings"`
}

// Service fetches NHL rosters into a Store and serves them from it.
type Service struct {
	client *nhl.Client
	store  Store

	refreshMu sync.Mutex // serializes refreshes so concurrent callers share one
}

func NewService(client *nhl.Client, store Store) *Service {
	return &Service{client: client, store: store}
}

// CurrentRosters returns the stored rosters, in standings order as of the
// last refresh. If none are stored yet, it refreshes first.
func (s *Service) CurrentRosters(ctx context.Context) ([]TeamRoster, error) {
	teams, err := s.store.LoadRosters(ctx)
	if err != nil || len(teams) > 0 {
		return teams, err
	}
	if _, err := s.RefreshIfEmpty(ctx); err != nil {
		return nil, err
	}
	return s.store.LoadRosters(ctx)
}

// RefreshIfEmpty refreshes only when no rosters are stored, and reports
// whether it did.
func (s *Service) RefreshIfEmpty(ctx context.Context) (bool, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	teams, err := s.store.LoadRosters(ctx)
	if err != nil {
		return false, err
	}
	if len(teams) > 0 {
		return false, nil
	}
	return true, s.refresh(ctx)
}

// Refresh fetches the current standings and every team's roster, then saves
// them. Teams whose roster failed to fetch are not saved, so their previous
// roster is kept.
func (s *Service) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.refresh(ctx)
}

func (s *Service) refresh(ctx context.Context) error {
	results, err := s.fetch(ctx)
	if err != nil {
		return err
	}
	ok := make([]TeamRoster, 0, len(results))
	for _, r := range results {
		if r.Error != "" {
			log.Printf("rosters: fetch %s: %s", r.Team, r.Error)
			continue
		}
		ok = append(ok, r)
	}
	return s.store.SaveRosters(ctx, ok)
}

// RunDaily refreshes rosters once immediately if none are stored, then once
// a day, until ctx is cancelled.
func (s *Service) RunDaily(ctx context.Context) {
	daily.Run(ctx, "rosters", s, refreshTimeout)
}

func (s *Service) fetch(ctx context.Context) ([]TeamRoster, error) {
	body, err := s.client.StandingsNow(ctx)
	if err != nil {
		return nil, err
	}
	var standings standingsResponse
	if err := json.Unmarshal(body, &standings); err != nil {
		return nil, fmt.Errorf("decode standings: %w", err)
	}

	results := make([]TeamRoster, len(standings.Standings))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for i, team := range standings.Standings {
		abbrev := team.TeamAbbrev.Default
		results[i].Team = abbrev
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			roster, err := s.client.CurrentRoster(ctx, abbrev)
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			results[i].Roster = roster
		}()
	}
	wg.Wait()
	return results, nil
}
