package storage

import (
	"sync"

	"github.com/omarkamali/semango/internal/ingest"
)

// RepresentationStore defines an interface for storing and retrieving representations.
// This is a generic interface that could be backed by in-memory, BoltDB, etc.
type RepresentationStore interface {
	Add(rep ingest.Representation) error
	Get(id string) (ingest.Representation, bool)
	GetAll() []ingest.Representation
	Count() int
}

// InMemoryStore is a simple in-memory implementation of RepresentationStore.

type InMemoryStore struct {
	mu    sync.RWMutex
	reps  map[string]ingest.Representation
	order []string // To maintain insertion order for GetAll, if needed
}

// NewInMemoryStore creates a new InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		reps:  make(map[string]ingest.Representation),
		order: []string{},
	}
}

// Add stores a representation. It overwrites if the ID already exists.
func (s *InMemoryStore) Add(rep ingest.Representation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if ID already exists to avoid appending to order multiple times
	if _, exists := s.reps[rep.ID]; !exists {
		s.order = append(s.order, rep.ID)
	}
	s.reps[rep.ID] = rep
	return nil
}

// Get retrieves a representation by its ID.
func (s *InMemoryStore) Get(id string) (ingest.Representation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rep, found := s.reps[id]
	return rep, found
}

// GetAll retrieves all representations in the order they were added.
func (s *InMemoryStore) GetAll() []ingest.Representation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ingest.Representation, len(s.order))
	for i, id := range s.order {
		result[i] = s.reps[id]
	}
	return result
}

// Count returns the number of representations in the store.
func (s *InMemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.reps)
}
