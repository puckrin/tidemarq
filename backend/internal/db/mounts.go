package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Mount represents a configured network mount (SFTP or SMB).
type Mount struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	PasswordEnc  []byte    `json:"-"`
	SSHKeyEnc    []byte    `json:"-"`
	SMBShare     string    `json:"smb_share"`
	SMBDomain    string    `json:"smb_domain"`
	SFTPHostKey  string    `json:"sftp_host_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateMountParams holds fields required to create a mount.
type CreateMountParams struct {
	Name        string
	Type        string
	Host        string
	Port        int
	Username    string
	PasswordEnc []byte
	SSHKeyEnc   []byte
	SMBShare    string
	SMBDomain   string
	SFTPHostKey string
}

// UpdateMountParams holds fields that may be updated on a mount.
type UpdateMountParams struct {
	Name        string
	Host        string
	Port        int
	Username    string
	PasswordEnc []byte // nil = do not update
	SSHKeyEnc   []byte // nil = do not update
	SMBShare    string
	SMBDomain   string
	SFTPHostKey string
}

// CreateMount inserts a new mount and returns the created record.
func (db *DB) CreateMount(ctx context.Context, p CreateMountParams) (*Mount, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO mounts (name, type, host, port, username, password_enc, ssh_key_enc, smb_share, smb_domain, sftp_host_key)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Type, p.Host, p.Port, p.Username,
		p.PasswordEnc, p.SSHKeyEnc, p.SMBShare, p.SMBDomain, p.SFTPHostKey,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetMount(ctx, id)
}

// GetMount retrieves a mount by primary key.
func (db *DB) GetMount(ctx context.Context, id int64) (*Mount, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, type, host, port, username, password_enc, ssh_key_enc,
		        smb_share, smb_domain, sftp_host_key, created_at, updated_at
		 FROM mounts WHERE id = ?`, id,
	)
	return scanMount(row)
}

// ListMounts returns all mounts ordered by name.
func (db *DB) ListMounts(ctx context.Context) ([]*Mount, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, type, host, port, username, password_enc, ssh_key_enc,
		        smb_share, smb_domain, sftp_host_key, created_at, updated_at
		 FROM mounts ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mounts []*Mount
	for rows.Next() {
		m := &Mount{}
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Type, &m.Host, &m.Port, &m.Username,
			&m.PasswordEnc, &m.SSHKeyEnc, &m.SMBShare, &m.SMBDomain,
			&m.SFTPHostKey, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		mounts = append(mounts, m)
	}
	return mounts, rows.Err()
}

// UpdateMount applies p to the mount with the given ID.
func (db *DB) UpdateMount(ctx context.Context, id int64, p UpdateMountParams) (*Mount, error) {
	existing, err := db.GetMount(ctx, id)
	if err != nil {
		return nil, err
	}

	// Keep existing encrypted blobs if callers pass nil (i.e. no change).
	passwordEnc := existing.PasswordEnc
	if p.PasswordEnc != nil {
		passwordEnc = p.PasswordEnc
	}
	sshKeyEnc := existing.SSHKeyEnc
	if p.SSHKeyEnc != nil {
		sshKeyEnc = p.SSHKeyEnc
	}

	_, err = db.ExecContext(ctx,
		`UPDATE mounts SET name = ?, host = ?, port = ?, username = ?,
		                   password_enc = ?, ssh_key_enc = ?,
		                   smb_share = ?, smb_domain = ?, sftp_host_key = ?,
		                   updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		p.Name, p.Host, p.Port, p.Username,
		passwordEnc, sshKeyEnc, p.SMBShare, p.SMBDomain, p.SFTPHostKey, id,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return db.GetMount(ctx, id)
}

// DeleteMount removes a mount by ID.
func (db *DB) DeleteMount(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM mounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanMount(row *sql.Row) (*Mount, error) {
	m := &Mount{}
	err := row.Scan(
		&m.ID, &m.Name, &m.Type, &m.Host, &m.Port, &m.Username,
		&m.PasswordEnc, &m.SSHKeyEnc, &m.SMBShare, &m.SMBDomain,
		&m.SFTPHostKey, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}
