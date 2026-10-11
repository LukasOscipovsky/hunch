package standings

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"hunch/backend/nhl"
	"hunch/backend/nhl/daily"
)

// refreshTimeout bounds one refresh, which is a single NHL API request.
const refreshTimeout = time.Minute

// Store persists the standings between refreshes.
type Store interface {
	// SaveStandings replaces the stored standings.
	SaveStandings(ctx context.Context, standings json.RawMessage) error
	// LoadStandings returns the stored standings, or nil if none are stored.
	LoadStandings(ctx context.Context) (json.RawMessage, error)
}

// Service fetches the NHL standings into a Store and serves them from it.
type Service struct {
	client *nhl.Client
	store  Store

	refreshMu sync.Mutex // serializes refreshes so concurrent callers share one
}

func NewService(client *nhl.Client, store Store) *Service {
	return &Service{client: client, store: store}
}

// CurrentStandings returns the stored standings as of the last refresh. If
// none are stored yet, it refreshes first.
func (s *Service) CurrentStandings(ctx context.Context) (json.RawMessage, error) {
	standings, err := s.store.LoadStandings(ctx)
	if err != nil || standings != nil {
		return standings, err
	}
	if _, err := s.RefreshIfEmpty(ctx); err != nil {
		return nil, err
	}
	return s.store.LoadStandings(ctx)
}

// RefreshIfEmpty refreshes only when no standings are stored, and reports
// whether it did.
func (s *Service) RefreshIfEmpty(ctx context.Context) (bool, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	standings, err := s.store.LoadStandings(ctx)
	if err != nil {
		return false, err
	}
	if standings != nil {
		return false, nil
	}
	return true, s.refresh(ctx)
}

// Refresh fetches the current standings and replaces the stored ones.
func (s *Service) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.refresh(ctx)
}

func (s *Service) refresh(ctx context.Context) error {
	body, err := s.client.StandingsNow(ctx)
	if err != nil {
		return err
	}
	return s.store.SaveStandings(ctx, body)
}

// RunDaily refreshes the standings once immediately if none are stored, then
// once a day, until ctx is cancelled.
func (s *Service) RunDaily(ctx context.Context) {
	daily.Run(ctx, "standings", s, refreshTimeout)
}
