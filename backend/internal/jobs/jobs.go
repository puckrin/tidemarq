// Package jobs manages sync job lifecycle: CRUD, scheduling, FS watching,
// pause/resume, and progress broadcasting.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/tidemarq/tidemarq/internal/audit"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/hasher"
	"github.com/tidemarq/tidemarq/internal/mountfs"
	"github.com/tidemarq/tidemarq/internal/mounts"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/internal/watch"
	"github.com/tidemarq/tidemarq/internal/ws"
)

// Sentinel errors.
var (
	ErrAlreadyRunning = errors.New("job is already running")
	ErrNotRunning     = errors.New("job is not running")
	ErrNotFound       = errors.New("job not found")
)

// CreateParams holds the fields required to create a new job.
type CreateParams struct {
	Name             string
	SourcePath       string
	DestinationPath  string
	SourceMountID    *int64
	DestMountID      *int64
	Mode             string
	BandwidthLimitKB int64
	ConflictStrategy string
	CronSchedule     string
	WatchEnabled     bool
	FullChecksum     bool
	// HashAlgo selects the file integrity hash algorithm: "sha256" or "blake3".
	// Defaults to hasher.Default ("blake3") when empty.
	HashAlgo       string
	UseDelta       bool
	DeltaBlockSize int64
	DeltaMinBytes  int64
}

// UpdateParams holds the fields that may be updated on a job.
type UpdateParams struct {
	Name             string
	SourcePath       string
	DestinationPath  string
	SourceMountID    *int64
	DestMountID      *int64
	Mode             string
	BandwidthLimitKB int64
	ConflictStrategy string
	CronSchedule     string
	WatchEnabled     bool
	FullChecksum     bool
	// HashAlgo selects the file integrity hash algorithm: "sha256" or "blake3".
	HashAlgo       string
	UseDelta       bool
	DeltaBlockSize int64
	DeltaMinBytes  int64
}

// runContext tracks an in-progress job run.
type runContext struct {
	cancel  context.CancelFunc
	pauseCh chan struct{} // closed to signal a pause
}

// Service manages job CRUD, scheduling, watching, and execution.
type Service struct {
	db           *db.DB
	engine       *engine.Engine
	hub          *ws.Hub
	watcher      *watch.Manager
	versionsSvc  *versions.Service
	conflictsSvc *conflicts.Service
	mountsSvc    *mounts.Service // may be nil if mounts feature is disabled
	auditSvc     *audit.Service  // may be nil; used to persist job lifecycle events

	scheduler *cron.Cron

	mu       sync.Mutex
	running  map[int64]*runContext
	cronIDs  map[int64]cron.EntryID // job ID → cron entry ID
}

// New creates a Service. Call Start to activate scheduling and watching.
func New(database *db.DB, eng *engine.Engine, hub *ws.Hub, watcher *watch.Manager, versionsSvc *versions.Service, conflictsSvc *conflicts.Service, mountsSvc *mounts.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:           database,
		engine:       eng,
		hub:          hub,
		watcher:      watcher,
		versionsSvc:  versionsSvc,
		conflictsSvc: conflictsSvc,
		mountsSvc:    mountsSvc,
		auditSvc:     auditSvc,
		scheduler:    cron.New(),
		running:      make(map[int64]*runContext),
		cronIDs:      make(map[int64]cron.EntryID),
	}
}

// Start loads all jobs from the database, registers cron and watch triggers,
// and starts the cron scheduler. Call once at application startup.
func (s *Service) Start(ctx context.Context) error {
	jobs, err := s.db.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}
	for _, j := range jobs {
		if err := s.registerTriggers(j); err != nil {
			log.Printf("jobs: failed to register triggers for job %d (%s): %v", j.ID, j.Name, err)
		}
		// Reset any jobs that were left in "running" state by a prior crash.
		if j.Status == "running" {
			_ = s.db.UpdateJobStatus(ctx, j.ID, "idle", nil, false)
		}
	}

	// Daily maintenance sweep: expire quarantine files and prune the audit log.
	// Runs at midnight; also fires immediately on startup so a freshly-started
	// instance enforces retention without waiting up to 24 hours.
	sweep := func() {
		bg := context.Background()
		if s.versionsSvc != nil {
			if err := s.versionsSvc.ExpireQuarantine(bg); err != nil {
				log.Printf("maintenance: expire quarantine: %v", err)
			}
		}
		if s.auditSvc != nil {
			if err := s.auditSvc.PruneAuditLog(bg); err != nil {
				log.Printf("maintenance: prune audit log: %v", err)
			}
		}
	}
	if _, err := s.scheduler.AddFunc("@midnight", sweep); err != nil {
		return fmt.Errorf("registering maintenance sweep: %w", err)
	}
	go sweep() // run once at startup

	s.scheduler.Start()
	return nil
}

// Stop gracefully shuts down the scheduler and watcher.
func (s *Service) Stop() {
	s.scheduler.Stop()
	if s.watcher != nil {
		s.watcher.Close()
	}
}

// Create inserts a new job, registers its triggers, and returns it.
func (s *Service) Create(ctx context.Context, p CreateParams) (*db.Job, error) {
	// Source and destination paths are required for local FS jobs.
	// For mount-based jobs, the path is the sub-path within the mount (may be empty for mount root).
	if p.Name == "" {
		return nil, errors.New("name is required")
	}
	if p.SourceMountID == nil && p.SourcePath == "" {
		return nil, errors.New("source_path is required for local filesystem jobs")
	}
	if p.DestMountID == nil && p.DestinationPath == "" {
		return nil, errors.New("destination_path is required for local filesystem jobs")
	}
	if !validMode(p.Mode) {
		return nil, fmt.Errorf("invalid mode %q: must be one-way-backup, one-way-mirror, or two-way", p.Mode)
	}
	if p.Mode != "two-way" {
		p.ConflictStrategy = ""
	}
	if p.CronSchedule != "" {
		if _, err := cron.ParseStandard(p.CronSchedule); err != nil {
			return nil, fmt.Errorf("invalid cron_schedule: %w", err)
		}
	}
	if p.HashAlgo == "" {
		p.HashAlgo = hasher.Default
	}
	if _, err := hasher.New(p.HashAlgo); err != nil {
		return nil, fmt.Errorf("invalid hash_algo %q: must be \"sha256\" or \"blake3\"", p.HashAlgo)
	}
	j, err := s.db.CreateJob(ctx, db.CreateJobParams{
		Name:             p.Name,
		SourcePath:       p.SourcePath,
		DestinationPath:  p.DestinationPath,
		SourceMountID:    p.SourceMountID,
		DestMountID:      p.DestMountID,
		Mode:             p.Mode,
		BandwidthLimitKB: p.BandwidthLimitKB,
		ConflictStrategy: p.ConflictStrategy,
		CronSchedule:     p.CronSchedule,
		WatchEnabled:     p.WatchEnabled,
		FullChecksum:     p.FullChecksum,
		HashAlgo:         p.HashAlgo,
		UseDelta:         p.UseDelta,
		DeltaBlockSize:   p.DeltaBlockSize,
		DeltaMinBytes:    p.DeltaMinBytes,
	})
	if err != nil {
		return nil, err
	}
	if err := s.registerTriggers(j); err != nil {
		log.Printf("jobs: registering triggers for job %d: %v", j.ID, err)
	}
	return j, nil
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

// Update applies p to the job, re-registers triggers, and returns the updated job.
func (s *Service) Update(ctx context.Context, id int64, p UpdateParams) (*db.Job, error) {
	if p.Name == "" {
		return nil, errors.New("name is required")
	}
	if p.SourceMountID == nil && p.SourcePath == "" {
		return nil, errors.New("source_path is required for local filesystem jobs")
	}
	if p.DestMountID == nil && p.DestinationPath == "" {
		return nil, errors.New("destination_path is required for local filesystem jobs")
	}
	if !validMode(p.Mode) {
		return nil, fmt.Errorf("invalid mode %q", p.Mode)
	}
	if p.Mode != "two-way" {
		p.ConflictStrategy = ""
	}
	if p.CronSchedule != "" {
		if _, err := cron.ParseStandard(p.CronSchedule); err != nil {
			return nil, fmt.Errorf("invalid cron_schedule: %w", err)
		}
	}
	if p.HashAlgo == "" {
		p.HashAlgo = hasher.Default
	}
	if _, err := hasher.New(p.HashAlgo); err != nil {
		return nil, fmt.Errorf("invalid hash_algo %q: must be \"sha256\" or \"blake3\"", p.HashAlgo)
	}

	j, err := s.db.UpdateJob(ctx, id, db.UpdateJobParams{
		Name:             p.Name,
		SourcePath:       p.SourcePath,
		DestinationPath:  p.DestinationPath,
		SourceMountID:    p.SourceMountID,
		DestMountID:      p.DestMountID,
		Mode:             p.Mode,
		BandwidthLimitKB: p.BandwidthLimitKB,
		ConflictStrategy: p.ConflictStrategy,
		CronSchedule:     p.CronSchedule,
		WatchEnabled:     p.WatchEnabled,
		FullChecksum:     p.FullChecksum,
		HashAlgo:         p.HashAlgo,
		UseDelta:         p.UseDelta,
		DeltaBlockSize:   p.DeltaBlockSize,
		DeltaMinBytes:    p.DeltaMinBytes,
	})
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	s.deregisterTriggers(id)
	if err := s.registerTriggers(j); err != nil {
		log.Printf("jobs: re-registering triggers for job %d: %v", id, err)
	}
	return j, nil
}

// Delete removes the job and cleans up its triggers.
func (s *Service) Delete(ctx context.Context, id int64) error {
	s.deregisterTriggers(id)
	err := s.db.DeleteJob(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// Run starts job id asynchronously. Returns ErrAlreadyRunning if already executing.
func (s *Service) Run(ctx context.Context, id int64) error {
	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if _, ok := s.running[id]; ok {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(context.Background())
	pauseCh := make(chan struct{})
	s.running[id] = &runContext{cancel: cancel, pauseCh: pauseCh}
	s.mu.Unlock()

	go s.execRun(runCtx, job, pauseCh, cancel)
	return nil
}

// Pause signals a running job to stop gracefully after its current file.
func (s *Service) Pause(ctx context.Context, id int64) error {
	s.mu.Lock()
	rc, ok := s.running[id]
	s.mu.Unlock()
	if !ok {
		return ErrNotRunning
	}
	// Close the pause channel exactly once.
	select {
	case <-rc.pauseCh:
		// Already closed.
	default:
		close(rc.pauseCh)
	}
	return nil
}

// Resume triggers a new run for a paused job. Since the manifest tracks all
// completed files, the engine automatically skips already-synced files.
func (s *Service) Resume(ctx context.Context, id int64) error {
	return s.Run(ctx, id)
}

// openJobFS opens the source and destination MountFS for a job.
// Returns nil, nil for local paths. The caller must close any non-nil FS when done.
func (s *Service) openJobFS(ctx context.Context, job *db.Job) (srcFS, dstFS mountfs.MountFS, err error) {
	if s.mountsSvc == nil {
		return nil, nil, nil
	}
	if job.SourceMountID != nil {
		srcFS, err = s.mountsSvc.OpenAt(ctx, *job.SourceMountID, job.SourcePath)
		if err != nil {
			return nil, nil, fmt.Errorf("opening source mount: %w", err)
		}
	}
	if job.DestMountID != nil {
		dstFS, err = s.mountsSvc.OpenAt(ctx, *job.DestMountID, job.DestinationPath)
		if err != nil {
			if srcFS != nil {
				srcFS.Close()
			}
			return nil, nil, fmt.Errorf("opening destination mount: %w", err)
		}
	}
	return srcFS, dstFS, nil
}

// execRun is the goroutine body for a single job execution.
func (s *Service) execRun(ctx context.Context, job *db.Job, pauseCh chan struct{}, cancel context.CancelFunc) {
	defer cancel()
	defer func() {
		s.mu.Lock()
		delete(s.running, job.ID)
		s.mu.Unlock()
	}()

	_ = s.db.UpdateJobStatus(ctx, job.ID, "running", nil, false)
	s.hub.Broadcast(ws.Event{JobID: job.ID, Event: "started"})
	if s.auditSvc != nil {
		s.auditSvc.LogJob(ctx, job.ID, job.Name, "system", "job_started", "Job started", "")
	}

	// Open mount FS connections if the job uses network mounts.
	srcFS, dstFS, fsErr := s.openJobFS(ctx, job)
	if fsErr != nil {
		msg := fsErr.Error()
		_ = s.db.UpdateJobStatus(ctx, job.ID, "error", &msg, true)
		s.hub.Broadcast(ws.Event{JobID: job.ID, Event: "error", Message: msg})
		return
	}
	if srcFS != nil {
		defer srcFS.Close()
	}
	if dstFS != nil {
		defer dstFS.Close()
	}

	// Rate-limit "scanning" events to at most 10 per second.
	var (
		lastScanEventMu sync.Mutex
		lastScanEvent   time.Time
	)

	result, runErr := s.engine.Run(ctx, engine.Config{
		JobID:            job.ID,
		Mode:             job.Mode,
		ConflictStrategy: job.ConflictStrategy,
		SourcePath:       job.SourcePath,
		DestinationPath:  job.DestinationPath,
		SourceFS:         srcFS,
		DestFS:           dstFS,
		BandwidthLimitKB: job.BandwidthLimitKB,
		FullChecksum:     job.FullChecksum,
		HashAlgo:         job.HashAlgo,
		UseDelta:         job.UseDelta,
		DeltaBlockSize:   int(job.DeltaBlockSize),
		DeltaMinBytes:    job.DeltaMinBytes,
		PauseCh:          pauseCh,
		VersionsSvc:      s.versionsSvc,
		ConflictsSvc:     s.conflictsSvc,
		OnFileStart: func(relPath string) {
			lastScanEventMu.Lock()
			defer lastScanEventMu.Unlock()
			if time.Since(lastScanEvent) < 100*time.Millisecond {
				return
			}
			lastScanEvent = time.Now()
			s.hub.Broadcast(ws.Event{
				JobID:       job.ID,
				Event:       "progress",
				CurrentFile: relPath,
				FileAction:  "scanning",
			})
		},
		// OnFileCopyStart fires when bytes are about to move. Not throttled —
		// copies are infrequent and the user needs to see this immediately.
		OnFileCopyStart: func(relPath string) {
			s.hub.Broadcast(ws.Event{
				JobID:       job.ID,
				Event:       "progress",
				CurrentFile: relPath,
				FileAction:  "copying",
			})
		},
		OnProgress: func(p engine.Progress) {
			eta := 0
			if p.RateKBs > 0 && p.BytesDone > 0 && p.FilesTotal > p.FilesDone {
				remaining := int64(p.FilesTotal-p.FilesDone) * (p.BytesDone / int64(p.FilesDone+1))
				eta = int(float64(remaining) / 1024 / p.RateKBs)
			}
			s.hub.Broadcast(ws.Event{
				JobID:        job.ID,
				Event:        "progress",
				FilesDone:    p.FilesDone,
				FilesTotal:   p.FilesTotal,
				FilesSkipped: p.FilesSkipped,
				BytesDone:    p.BytesDone,
				RateKBs:      p.RateKBs,
				ETASecs:      eta,
				CurrentFile:  p.CurrentFile,
				FileAction:   p.FileAction,
			})
		},
	})

	if runErr != nil {
		msg := runErr.Error()
		_ = s.db.UpdateJobStatus(context.Background(), job.ID, "error", &msg, true)
		s.hub.Broadcast(ws.Event{JobID: job.ID, Event: "error", Message: msg})
		if s.auditSvc != nil {
			s.auditSvc.LogJob(context.Background(), job.ID, job.Name, "system", "job_failed", "Job error", msg)
		}
		return
	}

	if result.Paused {
		_ = s.db.UpdateJobStatus(context.Background(), job.ID, "paused", nil, false)
		s.hub.Broadcast(ws.Event{JobID: job.ID, Event: "paused"})
		if s.auditSvc != nil {
			s.auditSvc.LogJob(context.Background(), job.ID, job.Name, "system", "job_stopped", "Job stopped", "")
		}
		return
	}

	if len(result.Errors) > 0 {
		msg := fmt.Sprintf("%d file(s) failed: %v", len(result.Errors), result.Errors[0].Err)
		_ = s.db.UpdateJobStatus(context.Background(), job.ID, "error", &msg, true)
		s.hub.Broadcast(ws.Event{JobID: job.ID, Event: "error", Message: msg})
		if s.auditSvc != nil {
			s.auditSvc.LogJob(context.Background(), job.ID, job.Name, "system", "job_failed", "Job error", msg)
		}
		return
	}

	total := result.FilesCopied + result.FilesSkipped
	_ = s.db.UpdateJobStatus(context.Background(), job.ID, "idle", nil, true)
	s.hub.Broadcast(ws.Event{
		JobID:      job.ID,
		Event:      "completed",
		FilesDone:  total,
		FilesTotal: total,
	})
	if s.auditSvc != nil {
		detail := fmt.Sprintf("%d files processed (%d copied, %d skipped)", total, result.FilesCopied, result.FilesSkipped)
		s.auditSvc.LogJob(context.Background(), job.ID, job.Name, "system", "job_completed", "Job completed", detail)
	}
}

// registerTriggers sets up cron and/or watch triggers for j.
func (s *Service) registerTriggers(j *db.Job) error {
	if j.CronSchedule != "" {
		entryID, err := s.scheduler.AddFunc(j.CronSchedule, func() {
			if err := s.Run(context.Background(), j.ID); err != nil && !errors.Is(err, ErrAlreadyRunning) {
				log.Printf("jobs: cron trigger for job %d failed: %v", j.ID, err)
			}
		})
		if err != nil {
			return fmt.Errorf("adding cron schedule: %w", err)
		}
		s.mu.Lock()
		s.cronIDs[j.ID] = entryID
		s.mu.Unlock()
	}

	if j.WatchEnabled && s.watcher != nil {
		if _, statErr := os.Stat(j.SourcePath); os.IsNotExist(statErr) {
			// Source path doesn't exist yet (e.g. external drive not mounted).
			// Watch is skipped — the job still runs on cron or manual trigger.
			log.Printf("jobs: watch for job %d (%s) skipped — source path not found: %s", j.ID, j.Name, j.SourcePath)
		} else if err := s.watcher.Add(j.ID, j.SourcePath, func(jobID int64) {
			if err := s.Run(context.Background(), jobID); err != nil && !errors.Is(err, ErrAlreadyRunning) {
				log.Printf("jobs: watch trigger for job %d failed: %v", jobID, err)
			}
		}); err != nil {
			return fmt.Errorf("adding watch: %w", err)
		}
	}
	return nil
}

// deregisterTriggers removes cron and watch triggers for jobID.
func (s *Service) deregisterTriggers(jobID int64) {
	s.mu.Lock()
	if entryID, ok := s.cronIDs[jobID]; ok {
		s.scheduler.Remove(entryID)
		delete(s.cronIDs, jobID)
	}
	s.mu.Unlock()

	if s.watcher != nil {
		s.watcher.Remove(jobID)
	}
}

func validMode(mode string) bool {
	switch mode {
	case "one-way-backup", "one-way-mirror", "two-way":
		return true
	}
	return false
}
