package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tidemarq/tidemarq/internal/audit"
	"github.com/tidemarq/tidemarq/internal/auth"
	"github.com/tidemarq/tidemarq/internal/config"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/jobs"
	"github.com/tidemarq/tidemarq/internal/mounts"
	"github.com/tidemarq/tidemarq/internal/notifications"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/internal/ws"
)

// Server holds the application's dependencies.
type Server struct {
	db           *db.DB
	authSvc      *auth.Service
	jobsSvc      *jobs.Service
	hub          *ws.Hub
	cfg          *config.Config
	conflictsSvc *conflicts.Service
	versionsSvc  *versions.Service
	mountsSvc    *mounts.Service
	notifSvc     *notifications.Service
	auditSvc     *audit.Service
	startTime    time.Time
}

// NewServer creates a Server with the given dependencies.
func NewServer(
	cfg *config.Config,
	database *db.DB,
	authSvc *auth.Service,
	jobsSvc *jobs.Service,
	hub *ws.Hub,
	conflictsSvc *conflicts.Service,
	versionsSvc *versions.Service,
	mountsSvc *mounts.Service,
	notifSvc *notifications.Service,
	auditSvc *audit.Service,
) *Server {
	return &Server{
		db:           database,
		authSvc:      authSvc,
		jobsSvc:      jobsSvc,
		hub:          hub,
		cfg:          cfg,
		conflictsSvc: conflictsSvc,
		versionsSvc:  versionsSvc,
		mountsSvc:    mountsSvc,
		notifSvc:     notifSvc,
		auditSvc:     auditSvc,
		startTime:    time.Now(),
	}
}

// Run starts the HTTP redirect listener and the HTTPS server.
// It blocks until the HTTPS server exits.
func (s *Server) Run() error {
	if err := ensureCert(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile); err != nil {
		return fmt.Errorf("ensuring TLS cert: %w", err)
	}

	// HTTP → HTTPS redirect.
	httpAddr := fmt.Sprintf(":%d", s.cfg.Server.HTTPPort)
	httpsPort := s.cfg.Server.HTTPSPort
	go func() {
		redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if host == "" {
				host = fmt.Sprintf("localhost:%d", httpsPort)
			} else {
				// Replace or append the HTTPS port.
				if h, _, err := net.SplitHostPort(host); err == nil {
					host = fmt.Sprintf("%s:%d", h, httpsPort)
				} else {
					host = fmt.Sprintf("%s:%d", host, httpsPort)
				}
			}
			http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusMovedPermanently)
		})
		http.ListenAndServe(httpAddr, redirect) //nolint:errcheck
	}()

	// HTTPS server.
	httpsAddr := fmt.Sprintf(":%d", s.cfg.Server.HTTPSPort)
	srv := &http.Server{
		Addr:    httpsAddr,
		Handler: s.Routes(),
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return srv.ListenAndServeTLS(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
}

// ensureCert generates a self-signed TLS certificate if one does not already exist.
func ensureCert(certFile, keyFile string) error {
	if _, err := os.Stat(certFile); err == nil {
		return nil // already exists
	}

	if err := os.MkdirAll(filepath.Dir(certFile), 0700); err != nil {
		return fmt.Errorf("creating cert directory: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"tidemarq"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("creating certificate: %w", err)
	}

	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening cert file: %w", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling key: %w", err)
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening key file: %w", err)
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
