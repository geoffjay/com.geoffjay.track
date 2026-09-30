package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Token errors.
var (
	// ErrTokenNotFound is returned when a token id does not exist or belongs
	// to another user (not distinguished, to avoid leaking existence).
	ErrTokenNotFound = errors.New("token not found")
	// ErrBadExpiry is returned when an expiry timestamp is in the past or
	// unparseable.
	ErrBadExpiry = errors.New("invalid expiry")
)

// Token is a row of the api_tokens table. Hash is never exposed; raw values
// are shown exactly once at creation time.
type Token struct {
	ID         int64
	UserID     int64
	Name       string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// TokenStore provides API-token persistence and verification.
type TokenStore struct{ db *sql.DB }

// NewTokenStore wraps db.
func NewTokenStore(db *sql.DB) *TokenStore { return &TokenStore{db: db} }

// hashToken returns the hex SHA-256 of a raw token value.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// generateToken produces a new raw token value: 32 bytes of crypto-random
// data, base64url-encoded (43 chars, no padding).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// parseTokenTime parses a stored token timestamp; empty yields nil.
func parseTokenTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil, fmt.Errorf("parse token timestamp %q: %w", s, err)
	}
	utc := t.UTC()
	return &utc, nil
}

// tokenCols fixes the column order scanToken reads.
const tokenCols = `id, user_id, name, expires_at, last_used_at, created_at`

// scanToken reads one api_tokens row (column order fixed by tokenCols).
func scanToken(row interface{ Scan(...any) error }) (Token, error) {
	var t Token
	var expires, lastUsed, createdAt sql.NullString
	if err := row.Scan(&t.ID, &t.UserID, &t.Name, &expires, &lastUsed, &createdAt); err != nil {
		return Token{}, err
	}
	if expires.Valid {
		p, err := parseTokenTime(expires.String)
		if err != nil {
			return Token{}, err
		}
		t.ExpiresAt = p
	}
	if lastUsed.Valid {
		p, err := parseTokenTime(lastUsed.String)
		if err != nil {
			return Token{}, err
		}
		t.LastUsedAt = p
	}
	if createdAt.Valid {
		p, err := parseTokenTime(createdAt.String)
		if err != nil {
			return Token{}, err
		}
		t.CreatedAt = *p
	}
	return t, nil
}

// Create generates a new token for userID. name labels it in the settings
// UI; expiresAt nil means the token never expires. The raw value is
// returned exactly once — it is not recoverable later.
func (s *TokenStore) Create(ctx context.Context, userID int64, name string, expiresAt *time.Time) (Token, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "api token"
	}
	if expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
		return Token{}, "", ErrBadExpiry
	}
	raw, err := generateToken()
	if err != nil {
		return Token{}, "", err
	}
	var exp any
	if expiresAt != nil {
		exp = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (user_id, name, token_hash, expires_at) VALUES (?, ?, ?, ?)`,
		userID, name, hashToken(raw), exp)
	if err != nil {
		return Token{}, "", fmt.Errorf("insert token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Token{}, "", err
	}
	return Token{
		ID:        id,
		UserID:    userID,
		Name:      name,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, raw, nil
}

// Verify looks up a raw token value and, when valid and unexpired, returns
// the owning user. LastUsedAt is refreshed opportunistically; a failure to
// update it does not fail authentication.
func (s *TokenStore) Verify(ctx context.Context, raw string) (User, error) {
	if strings.TrimSpace(raw) == "" {
		return User{}, ErrBadCredentials
	}
	var (
		userID   int64
		username string
		password string
		expires  sql.NullString
	)
	// The user columns ride along to avoid a second query; password is
	// selected but ignored (bearer auth never needs it).
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.password_hash, t.expires_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ?`, hashToken(raw)).
		Scan(&userID, &username, &password, &expires)
	if err == sql.ErrNoRows {
		return User{}, ErrBadCredentials
	}
	if err != nil {
		return User{}, err
	}
	var exp *time.Time
	if expires.Valid {
		p, err := parseTokenTime(expires.String)
		if err != nil {
			return User{}, err
		}
		exp = p
	}
	if exp != nil && !exp.After(time.Now().UTC()) {
		return User{}, ErrBadCredentials
	}
	u := User{ID: userID, Username: username, PasswordHash: password}
	// Fire-and-forget last_used_at refresh; failure does not fail auth.
	go func() {
		_, _ = s.db.ExecContext(context.Background(),
			`UPDATE api_tokens SET last_used_at = ? WHERE token_hash = ?`,
			time.Now().UTC().Format(time.RFC3339Nano), hashToken(raw))
	}()
	return u, nil
}

// ListByUser returns a user's tokens, newest first. Hashes are never exposed.
func (s *TokenStore) ListByUser(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+tokenCols+` FROM api_tokens WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes one of the user's tokens.
func (s *TokenStore) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTokenNotFound
	}
	return nil
}
