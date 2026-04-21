// Package audit provides helpers for writing and querying the persistent audit log.
package audit

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

// Service writes and queries the audit log.
type Service struct {
	db *db.DB
}

// New creates an audit Service.
func New(database *db.DB) *Service {
	return &Service{db: database}
}

// Log writes a single audit entry.
func (s *Service) Log(ctx context.Context, p db.CreateAuditEntryParams) {
	// Best-effort: audit log failures must never crash or block the caller.
	_, _ = s.db.CreateAuditEntry(ctx, p)
}

// LogJob is a convenience wrapper for job lifecycle events.
func (s *Service) LogJob(ctx context.Context, jobID int64, jobName, actor, event, message, detail string) {
	s.Log(ctx, db.CreateAuditEntryParams{
		JobID:   &jobID,
		JobName: jobName,
		Actor:   actor,
		Event:   event,
		Message: message,
		Detail:  detail,
	})
}

// Query returns entries matching the filter.
func (s *Service) Query(ctx context.Context, f db.AuditFilter) ([]*db.AuditEntry, error) {
	return s.db.ListAuditEntries(ctx, f)
}

// PruneAuditLog deletes audit log entries older than the configured retention
// period. It reads the current setting from the database on each call so that
// changes take effect at the next scheduled sweep without a restart.
func (s *Service) PruneAuditLog(ctx context.Context) error {
	settings, err := s.db.GetSettings(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.DeleteExpiredAuditEntries(ctx, settings.AuditLogRetentionDays)
	return err
}

// ExportCSV writes entries matching the filter as RFC 4180 CSV to a new buffer
// and returns it. Header row: id, job_id, job_name, actor, event, message, detail, created_at.
func (s *Service) ExportCSV(ctx context.Context, f db.AuditFilter) ([]byte, error) {
	// Raise limit for exports.
	f.Limit = 100_000

	entries, err := s.db.ListAuditEntries(ctx, f)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"id", "job_id", "job_name", "actor", "event", "message", "detail", "created_at"}); err != nil {
		return nil, err
	}

	for _, e := range entries {
		jobIDStr := ""
		if e.JobID != nil {
			jobIDStr = fmt.Sprintf("%d", *e.JobID)
		}
		if err := w.Write([]string{
			fmt.Sprintf("%d", e.ID),
			jobIDStr,
			e.JobName,
			e.Actor,
			e.Event,
			e.Message,
			e.Detail,
			e.CreatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

// ExportJSON returns entries as a JSON array.
func (s *Service) ExportJSON(ctx context.Context, f db.AuditFilter) ([]byte, error) {
	f.Limit = 100_000

	entries, err := s.db.ListAuditEntries(ctx, f)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(entries, "", "  ")
}
