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
	"strconv"
	"time"

	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

// CookieName is the session cookie used by the web UI.
const CookieName = "tn_session"

// Session lifetime defaults and bounds (in days). The effective value is read
// from the sessionTTL site setting so operators can tune it in the admin UI.
const (
	DefaultSessionTTLDays = 66
	MinSessionTTLDays     = 1
	MaxSessionTTLDays     = 365
)

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

// TTL resolves the configured session lifetime, falling back to the default
// when the setting is missing or out of range.
func (m *Manager) TTL(ctx context.Context) time.Duration {
	days := DefaultSessionTTLDays
	if settings, err := m.store.Settings(ctx); err == nil {
		if n, err := strconv.Atoi(settings["sessionTTL"]); err == nil &&
			n >= MinSessionTTLDays && n <= MaxSessionTTLDays {
			days = n
		}
	}
	return time.Duration(days) * 24 * time.Hour
}

// HashToken derives the storage key from a raw token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Login verifies a local password and returns the raw session token together
// with the lifetime that was applied. Accounts provisioned by LDAP or OIDC
// must authenticate through their provider instead.
func (m *Manager) Login(ctx context.Context, username, password string) (*model.User, string, time.Duration, error) {
	user, hash, err := m.store.UserByUsername(ctx, username)
	if err != nil {
		return nil, "", 0, err
	}
	if user.IsExternal() {
		return nil, "", 0, ErrExternalAccount
	}
	if !CheckPassword(hash, password) {
		return nil, "", 0, errors.New("invalid credentials")
	}
	token, ttl, err := m.SignIn(ctx, user)
	if err != nil {
		return nil, "", 0, err
	}
	return user, token, ttl, nil
}

// ErrExternalAccount is returned when a password login targets an account
// managed by LDAP or OIDC.
var ErrExternalAccount = errors.New("external account")

// SignIn creates a session for an already authenticated account, including
// accounts provisioned by an external provider.
func (m *Manager) SignIn(ctx context.Context, user *model.User) (string, time.Duration, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	ttl := m.TTL(ctx)
	expires := time.Now().Add(ttl).Unix()
	if err := m.store.CreateSession(ctx, HashToken(token), user.ID, expires); err != nil {
		return "", 0, err
	}
	return token, ttl, nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
func CheckPassword(hash, password string) bool {
	return bcryptCompare(hash, password)
}

// CookieSecure reports whether cookies are marked Secure. The OIDC state
// cookie follows the same policy as the session cookie.
func (m *Manager) CookieSecure() bool { return m.secure }

// SetCookie writes the session cookie with the matching lifetime.
func (m *Manager) SetCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl / time.Second),
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
