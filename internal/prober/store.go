package prober

import (
	"sync"
	"time"
)

type ProxyStatus struct {
	Name           string
	Type           string
	Up             bool
	DelayMillis    float64
	DelayValid     bool
	ScrapeSuccess  bool
	ScrapeDuration time.Duration
	LastScrapeTime time.Time
}

type Store struct {
	mu       sync.RWMutex
	apiUp    bool
	statuses map[string]ProxyStatus
}

type Snapshot struct {
	APIUp    bool
	Statuses []ProxyStatus
}

func NewStore() *Store {
	return &Store{statuses: make(map[string]ProxyStatus)}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]ProxyStatus, 0, len(s.statuses))
	for _, status := range s.statuses {
		statuses = append(statuses, status)
	}
	return Snapshot{APIUp: s.apiUp, Statuses: statuses}
}

func (s *Store) ReplaceBatch(results []ProbeResult) {
	now := time.Now()
	next := make(map[string]ProxyStatus, len(results))

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, result := range results {
		status := ProxyStatus{
			Name:           result.Name,
			Type:           result.Type,
			Up:             result.Up,
			ScrapeSuccess:  result.ScrapeSuccess,
			ScrapeDuration: result.ScrapeDuration,
			LastScrapeTime: now,
		}
		if result.Up {
			status.DelayMillis = result.DelayMillis
			status.DelayValid = true
		} else if old, ok := s.statuses[result.Name]; ok {
			status.DelayMillis = old.DelayMillis
			status.DelayValid = old.DelayValid
		}
		next[result.Name] = status
	}

	s.apiUp = true
	s.statuses = next
}

func (s *Store) MarkAPIUnavailable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.apiUp = false
	for name, status := range s.statuses {
		status.Up = false
		status.ScrapeSuccess = false
		status.LastScrapeTime = time.Now()
		s.statuses[name] = status
	}
}
