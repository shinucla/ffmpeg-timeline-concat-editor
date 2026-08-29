package store

import (
	"sync"

	"github.com/shin/web-load-time-cut-concat/backend/models"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*models.Job
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*models.Job)}
}

func (s *JobStore) Set(job *models.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*models.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}
