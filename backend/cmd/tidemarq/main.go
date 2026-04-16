package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tidemarq/tidemarq/internal/api"
	"github.com/tidemarq/tidemarq/internal/audit"
	"github.com/tidemarq/tidemarq/internal/auth"
	"github.com/tidemarq/tidemarq/internal/config"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/jobs"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/internal/mounts"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/internal/watch"
	"github.com/tidemarq/tidemarq/internal/ws"
	"github.com/tidemarq/tidemarq/migrations"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: TIDEMARQ_CONFIG env var or tidemarq.yaml)")
	flag.Parse()

	if *configPath == "" {
		if v := os.Getenv("TIDEMARQ_CONFIG"); v != "" {
			*configPath = v
		}
	}

	if err := run(*configPath); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := config.EnsureJWTSecret(cfg); err != nil {
		return fmt.Errorf("ensuring jwt secret: %w", err)
	}

	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(migrations.FS); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if err := seedAdmin(database, cfg); err != nil {
		return fmt.Errorf("seeding admin: %w", err)
	}

	// Derive storage directories from the database path.
	dataDir := filepath.Dir(cfg.Database.Path)
	versionsDir := filepath.Join(dataDir, "versions")

	watcher, err := watch.New()
	if err != nil {
		return fmt.Errorf("creating file watcher: %w", err)
	}

	hub := ws.New()
	authSvc := auth.NewService(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)
	manifestStore := manifest.New(database)
	syncEngine := engine.New(manifestStore)
	conflictsSvc := conflicts.New(database)
	versionsSvc := versions.New(database, versionsDir, 30)
	mountsSvc := mounts.New(database, cfg.Auth.JWTSecret)
	auditSvc := audit.New(database)
	jobsSvc := jobs.New(database, syncEngine, hub, watcher, versionsSvc, conflictsSvc, mountsSvc, auditSvc)

	if err := jobsSvc.Start(context.Background()); err != nil {
		return fmt.Errorf("starting job service: %w", err)
	}
	defer jobsSvc.Stop()

	srv := api.NewServer(cfg, database, authSvc, jobsSvc, hub, conflictsSvc, versionsSvc, mountsSvc, auditSvc)

	log.Printf("tidemarq %s starting — https://localhost:%d", api.Version, cfg.Server.HTTPSPort)
	return srv.Run()
}

func seedAdmin(database *db.DB, cfg *config.Config) error {
	count, err := database.UserCount(context.Background())
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Printf("no users found — creating default admin account (%q)", cfg.Admin.Username)

	hash, err := auth.HashPassword(cfg.Admin.Password)
	if err != nil {
		return err
	}

	_, err = database.CreateUser(context.Background(), cfg.Admin.Username, hash, "admin")
	return err
}
