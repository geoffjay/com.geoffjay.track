package auth

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// User is a row of the users table.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

// ErrBadCredentials is returned when the username is unknown or the password
// does not match; the message is deliberately identical for both cases.
var ErrBadCredentials = errors.New("invalid username or password")

// Store provides user persistence.
type Store struct{ db *sql.DB }

// NewStore wraps db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Hash returns the bcrypt hash of password at the given cost.
func Hash(password string, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), cost)
}

// Verify reports whether password matches the stored bcrypt hash.
func Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// EnsureUsers upserts the predefined users on every boot: the username row is
// created when missing, and the bcrypt hash is refreshed whenever the
// configured password (TRACK_GEOFF_PW / TRACK_MISTY_PW) no longer verifies.
// This makes the environment the source of truth — `fly secrets set` plus a
// redeploy rotates a password — while unchanged passwords cost one bcrypt
// compare per boot (a few ms each).
func (s *Store) EnsureUsers(ctx context.Context, users []User, cost int) error {
	for _, u := range users {
		var current string
		err := s.db.QueryRowContext(ctx,
			`SELECT password_hash FROM users WHERE username = ?`, u.Username).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			hash, err := Hash(u.PasswordHash, cost)
			if err != nil {
				return err
			}
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO users (username, password_hash) VALUES (?, ?)`,
				u.Username, string(hash))
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !Verify(current, u.PasswordHash) {
			hash, err := Hash(u.PasswordHash, cost)
			if err != nil {
				return err
			}
			_, err = s.db.ExecContext(ctx,
				`UPDATE users SET password_hash = ? WHERE username = ?`,
				string(hash), u.Username)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Authenticate validates username/password against the users table. The lookup
// and compare are constant-time-ish: an unknown user still runs one bcrypt
// compare against a fixed dummy hash so response timing does not reveal
// whether a username exists.
func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	const dummyHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5B0G8S5fU7i6jK0Q0a2n1E9t8sD7e"
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return User{}, ErrBadCredentials
	}
	if err != nil {
		return User{}, err
	}
	if !Verify(u.PasswordHash, password) {
		return User{}, ErrBadCredentials
	}
	return u, nil
}

