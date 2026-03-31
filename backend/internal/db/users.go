package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a uniqueness constraint is violated.
var ErrConflict = errors.New("conflict")

// User represents a row in the users table.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserCount returns the total number of users.
func (db *DB) UserCount(ctx context.Context) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser inserts a new user and returns the created record.
func (db *DB) CreateUser(ctx context.Context, username, passwordHash, role string) (*User, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		username, passwordHash, role,
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
	return db.GetUserByID(ctx, id)
}

// GetUserByID retrieves a user by primary key.
func (db *DB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return db.scanUser(db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at, updated_at FROM users WHERE id = ?`, id,
	))
}

// GetUserByUsername retrieves a user by username.
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return db.scanUser(db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at, updated_at FROM users WHERE username = ?`, username,
	))
}

// ListUsers returns all users ordered by username.
func (db *DB) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, username, password_hash, role, created_at, updated_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUserParams holds the optional fields that may be updated.
type UpdateUserParams struct {
	Username     *string
	PasswordHash *string
	Role         *string
}

// UpdateUser applies non-nil fields to the user record.
func (db *DB) UpdateUser(ctx context.Context, id int64, p UpdateUserParams) (*User, error) {
	u, err := db.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if p.Username != nil {
		u.Username = *p.Username
	}
	if p.PasswordHash != nil {
		u.PasswordHash = *p.PasswordHash
	}
	if p.Role != nil {
		u.Role = *p.Role
	}

	_, err = db.ExecContext(ctx,
		`UPDATE users SET username = ?, password_hash = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		u.Username, u.PasswordHash, u.Role, id,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return db.GetUserByID(ctx, id)
}

// DeleteUser removes a user by ID.
func (db *DB) DeleteUser(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
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

func (db *DB) scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// isUniqueConstraintError reports whether err is a SQLite UNIQUE violation.
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
