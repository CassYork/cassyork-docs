package memory

import (
	"context"
	"sync"

	"cassyork.dev/platform/internal/domain/document"
	"cassyork.dev/platform/internal/domain/ingestion"
)

// Stores are in-process dev implementations — swap for SQL without changing domain.
type DocumentStore struct {
	mu   sync.RWMutex
	byID map[string]*document.Document
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{byID: make(map[string]*document.Document)}
}

func (s *DocumentStore) Save(_ context.Context, d *document.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[d.ID()] = d
	return nil
}

type RunStore struct {
	mu   sync.RWMutex
	byID map[string]*ingestion.Run
}

func NewRunStore() *RunStore {
	return &RunStore{byID: make(map[string]*ingestion.Run)}
}

func (s *RunStore) Save(_ context.Context, r *ingestion.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[r.ID()] = r
	return nil
}
