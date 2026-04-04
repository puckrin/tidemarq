// Package notifications delivers job lifecycle events to configured targets
// (SMTP, webhook, Gotify).
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/tidemarq/tidemarq/internal/crypt"
	"github.com/tidemarq/tidemarq/internal/db"
)

// ErrNotFound is returned when a target/rule does not exist.
var ErrNotFound = errors.New("notification target not found")

// ErrConflict is returned on name uniqueness violations.
var ErrConflict = errors.New("notification target name already in use")

// SMTPConfig holds configuration for an SMTP notification target.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"` // comma-separated
}

// WebhookConfig holds configuration for an HTTP webhook target.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`  // default: POST
	Headers map[string]string `json:"headers"` // optional extra headers
}

// GotifyConfig holds configuration for a Gotify push notification target.
type GotifyConfig struct {
	URL      string `json:"url"`
	AppToken string `json:"app_token"`
	Priority int    `json:"priority"`
}

// TargetInput is the caller-facing create/update payload.
type TargetInput struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"` // type-specific config blob
	Enabled bool            `json:"enabled"`
}

// TargetView is the sanitised view returned to callers (config not included
// except as a type label).
type TargetView struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuleInput is the caller-facing rule create payload.
type RuleInput struct {
	TargetID int64  `json:"target_id"`
	Event    string `json:"event"`
	JobID    *int64 `json:"job_id,omitempty"`
}

// Service manages notification targets and rules and dispatches events.
type Service struct {
	db  *db.DB
	key [32]byte
}

// New creates a notifications Service.
func New(database *db.DB, secret string) *Service {
	return &Service{
		db:  database,
		key: crypt.KeyFromSecret(secret),
	}
}

// ─── Target CRUD ──────────────────────────────────────────────────────────────

func (s *Service) CreateTarget(ctx context.Context, in TargetInput) (*TargetView, error) {
	configEnc, err := crypt.Encrypt(s.key, in.Config)
	if err != nil {
		return nil, fmt.Errorf("encrypting config: %w", err)
	}
	t, err := s.db.CreateNotificationTarget(ctx, in.Name, in.Type, configEnc)
	if errors.Is(err, db.ErrConflict) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return targetToView(t), nil
}

func (s *Service) GetTarget(ctx context.Context, id int64) (*TargetView, error) {
	t, err := s.db.GetNotificationTarget(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return targetToView(t), nil
}

func (s *Service) ListTargets(ctx context.Context) ([]*TargetView, error) {
	targets, err := s.db.ListNotificationTargets(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]*TargetView, len(targets))
	for i, t := range targets {
		views[i] = targetToView(t)
	}
	return views, nil
}

func (s *Service) UpdateTarget(ctx context.Context, id int64, in TargetInput) (*TargetView, error) {
	existing, err := s.db.GetNotificationTarget(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	configEnc := existing.ConfigEnc
	if len(in.Config) > 0 {
		configEnc, err = crypt.Encrypt(s.key, in.Config)
		if err != nil {
			return nil, fmt.Errorf("encrypting config: %w", err)
		}
	}

	t, err := s.db.UpdateNotificationTarget(ctx, id, in.Name, configEnc, in.Enabled)
	if errors.Is(err, db.ErrConflict) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return targetToView(t), nil
}

func (s *Service) DeleteTarget(ctx context.Context, id int64) error {
	err := s.db.DeleteNotificationTarget(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// ─── Rule CRUD ────────────────────────────────────────────────────────────────

func (s *Service) CreateRule(ctx context.Context, in RuleInput) (*db.NotificationRule, error) {
	return s.db.CreateNotificationRule(ctx, in.TargetID, in.Event, in.JobID)
}

func (s *Service) ListRules(ctx context.Context, targetID int64) ([]*db.NotificationRule, error) {
	return s.db.ListNotificationRules(ctx, targetID)
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	err := s.db.DeleteNotificationRule(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// ─── Dispatch ─────────────────────────────────────────────────────────────────

// Notify fires all matching notification rules for event/jobID.
// Delivery errors are logged but not returned.
func (s *Service) Notify(ctx context.Context, event string, jobID int64, jobName, detail string) {
	targets, err := s.db.ListRulesForEvent(ctx, event, jobID)
	if err != nil {
		log.Printf("notifications: listing rules for event %q: %v", event, err)
		return
	}

	subject := fmt.Sprintf("[tidemarq] %s — %s", event, jobName)
	body := fmt.Sprintf("Event: %s\nJob: %s (id=%d)\n%s", event, jobName, jobID, detail)

	for _, t := range targets {
		cfg, err := crypt.Decrypt(s.key, t.ConfigEnc)
		if err != nil {
			log.Printf("notifications: decrypting config for target %d: %v", t.ID, err)
			continue
		}
		if err := s.dispatch(t.Type, cfg, subject, body, event, jobName, detail); err != nil {
			log.Printf("notifications: delivering to target %d (%s): %v", t.ID, t.Name, err)
		}
	}
}

func (s *Service) dispatch(typ string, cfgJSON []byte, subject, body, event, jobName, detail string) error {
	switch typ {
	case "smtp":
		var cfg SMTPConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return fmt.Errorf("unmarshal smtp config: %w", err)
		}
		return sendSMTP(cfg, subject, body)

	case "webhook":
		var cfg WebhookConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return fmt.Errorf("unmarshal webhook config: %w", err)
		}
		return sendWebhook(cfg, event, jobName, detail)

	case "gotify":
		var cfg GotifyConfig
		if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
			return fmt.Errorf("unmarshal gotify config: %w", err)
		}
		return sendGotify(cfg, subject, body)

	default:
		return fmt.Errorf("unknown notification type %q", typ)
	}
}

// ─── SMTP ─────────────────────────────────────────────────────────────────────

func sendSMTP(cfg SMTPConfig, subject, body string) error {
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	recipients := strings.Split(cfg.To, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		cfg.From, cfg.To, subject, body)

	return smtp.SendMail(addr, auth, cfg.From, recipients, []byte(msg))
}

// ─── Webhook ──────────────────────────────────────────────────────────────────

func sendWebhook(cfg WebhookConfig, event, jobName, detail string) error {
	payload := map[string]string{
		"event":    event,
		"job_name": jobName,
		"detail":   detail,
		"time":     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

// ─── Gotify ───────────────────────────────────────────────────────────────────

func sendGotify(cfg GotifyConfig, title, message string) error {
	priority := cfg.Priority
	if priority == 0 {
		priority = 5
	}

	payload := map[string]any{
		"title":    title,
		"message":  message,
		"priority": priority,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := strings.TrimRight(cfg.URL, "/") + "/message?token=" + cfg.AppToken

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gotify returned %d", resp.StatusCode)
	}
	return nil
}

func targetToView(t *db.NotificationTarget) *TargetView {
	return &TargetView{
		ID:        t.ID,
		Name:      t.Name,
		Type:      t.Type,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}
