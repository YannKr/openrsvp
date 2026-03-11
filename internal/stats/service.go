package stats

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const cacheTTL = 5 * time.Minute

// Service provides instance statistics with in-memory caching.
type Service struct {
	store  *Store
	logger zerolog.Logger

	mu        sync.Mutex
	cached    *InstanceStats
	cachedAt  time.Time
}

// NewService creates a new stats Service.
func NewService(store *Store, logger zerolog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// GetInstanceStats returns cached aggregate statistics, refreshing if stale.
func (s *Service) GetInstanceStats(ctx context.Context) (*InstanceStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cached != nil && time.Since(s.cachedAt) < cacheTTL {
		return s.cached, nil
	}

	stats, err := s.store.GetInstanceStats(ctx)
	if err != nil {
		return nil, err
	}

	s.cached = stats
	s.cachedAt = time.Now()
	return stats, nil
}
