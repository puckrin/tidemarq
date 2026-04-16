// Package mounts manages network mount configurations (SFTP, SMB).
package mounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tidemarq/tidemarq/internal/crypt"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/mountfs"
)

// ErrNotFound is returned when a mount record does not exist.
var ErrNotFound = errors.New("mount not found")

// ErrConflict is returned on name uniqueness violations.
var ErrConflict = errors.New("mount name already in use")

// Service manages mount records and opens live connections.
type Service struct {
	db  *db.DB
	key [32]byte
}

// New creates a Service. secret is used to derive the encryption key for
// stored credentials.
func New(database *db.DB, secret string) *Service {
	return &Service{
		db:  database,
		key: crypt.KeyFromSecret(secret),
	}
}

// MountInput is the caller-facing create/update payload.
type MountInput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`    // plain; encrypted before storage
	SSHKey      string `json:"ssh_key"`     // PEM private key; encrypted before storage
	SMBShare    string `json:"smb_share"`
	SMBDomain   string `json:"smb_domain"`
	SFTPHostKey string `json:"sftp_host_key"`
}

// MountView is the sanitised view returned to callers (no credentials).
type MountView struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	SMBShare    string    `json:"smb_share"`
	SMBDomain   string    `json:"smb_domain"`
	SFTPHostKey string    `json:"sftp_host_key"`
	HasPassword bool      `json:"has_password"`
	HasSSHKey   bool      `json:"has_ssh_key"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toView(m *db.Mount) *MountView {
	return &MountView{
		ID:          m.ID,
		Name:        m.Name,
		Type:        m.Type,
		Host:        m.Host,
		Port:        m.Port,
		Username:    m.Username,
		SMBShare:    m.SMBShare,
		SMBDomain:   m.SMBDomain,
		SFTPHostKey: m.SFTPHostKey,
		HasPassword: len(m.PasswordEnc) > 0,
		HasSSHKey:   len(m.SSHKeyEnc) > 0,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// Create persists a new mount.
func (s *Service) Create(ctx context.Context, in MountInput) (*MountView, error) {
	passwordEnc, sshKeyEnc, err := s.encryptCredentials(in.Password, in.SSHKey)
	if err != nil {
		return nil, err
	}
	m, err := s.db.CreateMount(ctx, db.CreateMountParams{
		Name:        in.Name,
		Type:        in.Type,
		Host:        in.Host,
		Port:        in.Port,
		Username:    in.Username,
		PasswordEnc: passwordEnc,
		SSHKeyEnc:   sshKeyEnc,
		SMBShare:    in.SMBShare,
		SMBDomain:   in.SMBDomain,
		SFTPHostKey: in.SFTPHostKey,
	})
	if errors.Is(err, db.ErrConflict) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return toView(m), nil
}

// Get returns a sanitised view of one mount.
func (s *Service) Get(ctx context.Context, id int64) (*MountView, error) {
	m, err := s.db.GetMount(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toView(m), nil
}

// List returns all mounts.
func (s *Service) List(ctx context.Context) ([]*MountView, error) {
	mounts, err := s.db.ListMounts(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]*MountView, len(mounts))
	for i, m := range mounts {
		views[i] = toView(m)
	}
	return views, nil
}

// Update applies changes to an existing mount.
// Passing an empty string for Password/SSHKey preserves the existing value.
func (s *Service) Update(ctx context.Context, id int64, in MountInput) (*MountView, error) {
	var passwordEnc, sshKeyEnc []byte
	if in.Password != "" {
		enc, _, err := s.encryptCredentials(in.Password, "")
		if err != nil {
			return nil, err
		}
		passwordEnc = enc
	}
	if in.SSHKey != "" {
		_, enc, err := s.encryptCredentials("", in.SSHKey)
		if err != nil {
			return nil, err
		}
		sshKeyEnc = enc
	}

	m, err := s.db.UpdateMount(ctx, id, db.UpdateMountParams{
		Name:        in.Name,
		Host:        in.Host,
		Port:        in.Port,
		Username:    in.Username,
		PasswordEnc: passwordEnc,
		SSHKeyEnc:   sshKeyEnc,
		SMBShare:    in.SMBShare,
		SMBDomain:   in.SMBDomain,
		SFTPHostKey: in.SFTPHostKey,
	})
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if errors.Is(err, db.ErrConflict) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return toView(m), nil
}

// Delete removes a mount record.
func (s *Service) Delete(ctx context.Context, id int64) error {
	err := s.db.DeleteMount(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// TestConnectivity opens a fully authenticated connection to the mount and
// immediately closes it. This verifies host reachability, credentials, and
// (for SFTP) the host key.
func (s *Service) TestConnectivity(ctx context.Context, id int64) error {
	fs, err := s.Open(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return fs.Close()
}

// OpenAt returns a live MountFS connection for the given mount, rooted at
// rootPath within the mount. For SFTP/SMB this sets the Root field so all
// relative paths are resolved under rootPath on the remote server.
// The caller is responsible for calling Close() when done.
func (s *Service) OpenAt(ctx context.Context, id int64, rootPath string) (mountfs.MountFS, error) {
	m, err := s.db.GetMount(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	password, sshKey, err := s.decryptCredentials(m)
	if err != nil {
		return nil, fmt.Errorf("decrypting credentials: %w", err)
	}

	switch m.Type {
	case "sftp":
		return mountfs.NewSFTP(mountfs.SFTPConfig{
			Host:       m.Host,
			Port:       m.Port,
			Username:   m.Username,
			Password:   password,
			PrivateKey: []byte(sshKey),
			HostKey:    m.SFTPHostKey,
			Root:       rootPath,
		})
	case "smb":
		return mountfs.NewSMB(mountfs.SMBConfig{
			Host:     m.Host,
			Port:     m.Port,
			Domain:   m.SMBDomain,
			Username: m.Username,
			Password: password,
			Share:    m.SMBShare,
			Root:     rootPath,
		})
	default:
		return nil, fmt.Errorf("unknown mount type %q", m.Type)
	}
}

// Open returns a live MountFS connection for the given mount.
// The caller is responsible for calling Close() when done.
func (s *Service) Open(ctx context.Context, id int64) (mountfs.MountFS, error) {
	m, err := s.db.GetMount(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	password, sshKey, err := s.decryptCredentials(m)
	if err != nil {
		return nil, fmt.Errorf("decrypting credentials: %w", err)
	}

	switch m.Type {
	case "sftp":
		return mountfs.NewSFTP(mountfs.SFTPConfig{
			Host:       m.Host,
			Port:       m.Port,
			Username:   m.Username,
			Password:   password,
			PrivateKey: []byte(sshKey),
			HostKey:    m.SFTPHostKey,
		})
	case "smb":
		return mountfs.NewSMB(mountfs.SMBConfig{
			Host:     m.Host,
			Port:     m.Port,
			Domain:   m.SMBDomain,
			Username: m.Username,
			Password: password,
			Share:    m.SMBShare,
		})
	default:
		return nil, fmt.Errorf("unknown mount type %q", m.Type)
	}
}

// encryptCredentials encrypts password and sshKey, returning nil blobs for empty inputs.
func (s *Service) encryptCredentials(password, sshKey string) (passwordEnc, sshKeyEnc []byte, err error) {
	if password != "" {
		passwordEnc, err = crypt.Encrypt(s.key, []byte(password))
		if err != nil {
			return nil, nil, fmt.Errorf("encrypting password: %w", err)
		}
	}
	if sshKey != "" {
		sshKeyEnc, err = crypt.Encrypt(s.key, []byte(sshKey))
		if err != nil {
			return nil, nil, fmt.Errorf("encrypting SSH key: %w", err)
		}
	}
	return passwordEnc, sshKeyEnc, nil
}

func (s *Service) decryptCredentials(m *db.Mount) (password, sshKey string, err error) {
	if len(m.PasswordEnc) > 0 {
		plain, err := crypt.Decrypt(s.key, m.PasswordEnc)
		if err != nil {
			return "", "", fmt.Errorf("decrypting password: %w", err)
		}
		password = string(plain)
	}
	if len(m.SSHKeyEnc) > 0 {
		plain, err := crypt.Decrypt(s.key, m.SSHKeyEnc)
		if err != nil {
			return "", "", fmt.Errorf("decrypting SSH key: %w", err)
		}
		sshKey = string(plain)
	}
	return password, sshKey, nil
}

