// Package auth implements cookie based sessions backed by SQLite.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

// CookieName is the session cookie used by the web UI.
const CookieName = "tn_session"

const sessionTTL = 30 * 24 * time.Hour

// Manager creates and validates sessions.
type Manager struct {
	store  *store.Store
	secure bool
}

// NewManager wires a session manager. secure should be true behind HTTPS so
// the cookie is marked Secure.
func NewManager(s *store.Store, secure bool) *Manager {
	return &Manager{store: s, secure: secure}
}

// HashToken derives the storage key from a raw token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Login verifies the password and returns the raw session token.
func (m *Manager) Login(ctx context.Context, username, password string) (*model.User, string, error) {
	user, hash, err := m.store.UserByUsername(ctx, username)
	if err != nil {
		return nil, "", err
	}
	if !CheckPassword(hash, password) {
		return nil, "", errors.New("invalid credentials")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	expires := time.Now().Add(sessionTTL).Unix()
	if err := m.store.CreateSession(ctx, HashToken(token), user.ID, expires); err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
func CheckPassword(hash, password string) bool {
	return bcryptCompare(hash, password)
}

// SetCookie writes the session cookie.
func (m *Manager) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
}

// ClearCookie expires the session cookie.
func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Logout deletes the server side session and clears the cookie.
func (m *Manager) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = m.store.DeleteSession(ctx, HashToken(c.Value))
	}
	m.ClearCookie(w)
}

// User resolves the current session user, if any.
func (m *Manager) User(ctx context.Context, r *http.Request) *model.User {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	user, err := m.store.SessionUser(ctx, HashToken(c.Value))
	if err != nil {
		return nil
	}
	return user
}
