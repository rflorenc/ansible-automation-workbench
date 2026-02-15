package models

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Job represents an async operation (cleanup, populate, export, cac-apply).
type Job struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`          // "awx-populate", "aap-cleanup", "cac-apply", etc.
	ConnectionID string    `json:"connection_id"`
	Status       string    `json:"status"`        // "running", "completed", "failed"
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Error        string    `json:"error,omitempty"`
	Output       []string  `json:"output"`
	mu           sync.Mutex
}

// AppendLog adds a log line to the job output.
func (j *Job) AppendLog(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Output = append(j.Output, line)
}

// LogsSince returns log lines starting from the given index.
func (j *Job) LogsSince(offset int) []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if offset >= len(j.Output) {
		return nil
	}
	lines := make([]string, len(j.Output)-offset)
	copy(lines, j.Output[offset:])
	return lines
}

// Complete marks the job as completed.
func (j *Job) Complete() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "completed"
	now := time.Now()
	j.FinishedAt = &now
}

// Fail marks the job as failed with an error message.
func (j *Job) Fail(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "failed"
	j.Error = err
	now := time.Now()
	j.FinishedAt = &now
}

// JobStore is an in-memory thread-safe store for jobs.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewJobStore creates an empty job store.
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*Job)}
}

// Create adds a new job, assigning it a UUID.
func (s *JobStore) Create(jobType, connectionID string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := &Job{
		ID:           uuid.New().String(),
		Type:         jobType,
		ConnectionID: connectionID,
		Status:       "running",
		StartedAt:    time.Now(),
		Output:       []string{},
	}
	s.jobs[j.ID] = j
	return j
}

// Get returns a job by ID.
func (s *JobStore) Get(id string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

// List returns all jobs, most recent first.
func (s *JobStore) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		result = append(result, j)
	}
	// Sort by started_at descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].StartedAt.After(result[i].StartedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
