// Package jobs manages sync job lifecycle.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
)

// ErrAlreadyRunning is returned when a job is triggered while already running.
var ErrAlreadyRunning = errors.New("job is already running")

// ErrNotFound is returned when the requested job does not exist.
var ErrNotFound = errors.New("job not found")

// CreateParams holds the fields required to create a new job.
type CreateParams struct {
	Name             string
	SourcePath       string
	DestinationPath  string
	Mode             string
	BandwidthLimitKB int64
}

// Service manages job CRUD and execution.
type Service struct {
	db     *db.DB
	engine *engine.Engine

	mu      sync.Mutex
	running map[int64]bool
}

// New creates a Service with the given dependencies.
func New(database *db.DB, eng *engine.Engine) *Service {
	return &Service{
		db:      database,
		engine:  eng,
		running: make(map[int64]bool),
	}
}

// Create inserts a new job and returns it.
func (s *Service) Create(ctx context.Context, p CreateParams) (*db.Job, error) {
	if p.Name == "" || p.SourcePath == "" || p.DestinationPath == "" {
		return nil, errors.New("name, source_path, and destination_path are required")
	}
	if !validMode(p.Mode) {
		return nil, fmt.Errorf("invalid mode %q: must be one-way-backup, one-way-mirror, or two-way", p.Mode)
	}
	return s.db.CreateJob(ctx, db.CreateJobParams{
		Name:             p.Name,
		SourcePath:       p.SourcePath,
		DestinationPath:  p.DestinationPath,
		Mode:             p.Mode,
		BandwidthLimitKB: p.BandwidthLimitKB,
	})
}

// Get returns the job with the given ID.
func (s *Service) Get(ctx context.Context, id int64) (*db.Job, error) {
	j, err := s.db.GetJobByID(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	return j, err
}

// List returns all jobs.
func (s *Service) List(ctx context.Context) ([]*db.Job, error) {
	return s.db.ListJobs(ctx)
}

// Run starts job id asynchronously in a goroutine and returns immediately.
// Returns ErrAlreadyRunning if the job is currently executing.
// Returns ErrNotFound if the job does not exist.
func (s *Service) Run(ctx context.Context, id int64) error {
	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.running[id] {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running[id] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, id)
			s.mu.Unlock()
		}()

		// Mark as running.
		_ = s.db.UpdateJobStatus(context.Background(), id, "running", nil, false)

		result, runErr := s.engine.Run(context.Background(), engine.Config{
			JobID:            id,
			SourcePath:       job.SourcePath,
			DestinationPath:  job.DestinationPath,
			BandwidthLimitKB: job.BandwidthLimitKB,
		})

		if runErr != nil {
			msg := runErr.Error()
			_ = s.db.UpdateJobStatus(context.Background(), id, "error", &msg, true)
			return
		}

		if len(result.Errors) > 0 {
			msg := fmt.Sprintf("%d file(s) failed: %v", len(result.Errors), result.Errors[0].Err)
			_ = s.db.UpdateJobStatus(context.Background(), id, "error", &msg, true)
			return
		}

		_ = s.db.UpdateJobStatus(context.Background(), id, "idle", nil, true)
	}()

	return nil
}

func validMode(mode string) bool {
	switch mode {
	case "one-way-backup", "one-way-mirror", "two-way":
		return true
	}
	return false
}
