package server

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Songxwn/team-nav-go/internal/auth"
	"github.com/Songxwn/team-nav-go/internal/store"
)

const oidcStateTTL = 10 * time.Minute

// handleOIDCStart begins the authorization code flow. It creates a one-time
// state, stores the nonce and PKCE verifier server side, then redirects to the
// identity provider.
func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("oidc:"+s.clientIP(r), 0.2, 10) {
		writeError(w, http.StatusTooManyRequests, "尝试次数过多，请一分钟后再试")
		return
	}
	ctx := r.Context()
	settings, err := s.store.Settings(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cfg := auth.OIDCConfigFromSettings(settings)
	if !cfg.Enabled {
		s.redirectAuthError(w, r, "disabled")
		return
	}
	state, err := auth.RandomToken(32)
	if err != nil {
		s.internalError(w, err)
		return
	}
	nonce, err := auth.RandomToken(24)
	if err != nil {
		s.internalError(w, err)
		return
	}
	verifier, err := auth.RandomToken(32)
	if err != nil {
		s.internalError(w, err)
		return
	}
	redirectTo := safeLocalPath(r.URL.Query().Get("redirect"))
	if err := s.store.SaveAuthState(ctx, &store.AuthState{
		State:     state,
		Nonce:     nonce,
		Verifier:  verifier,
		Redirect:  redirectTo,
		ExpiresAt: time.Now().Add(oidcStateTTL).Unix(),
	}); err != nil {
		s.internalError(w, err)
		return
	}
	authURL, err := s.oidc.AuthCodeURL(ctx, cfg, s.oidcRedirectURL(r, cfg), state, nonce, verifier)
	if err != nil {
		s.log.Error("oidc authorize url failed", "err", err)
		s.redirectAuthError(w, r, "unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback verifies the state, exchanges the code and signs the user
// in. Failures are reported to the UI as short error codes, never raw provider
// messages.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, err := s.store.Settings(ctx)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cfg := auth.OIDCConfigFromSettings(settings)
	if !cfg.Enabled {
		s.redirectAuthError(w, r, "disabled")
		return
	}
	pending, err := s.store.TakeAuthState(ctx, r.URL.Query().Get("state"))
	if err != nil {
		s.redirectAuthError(w, r, "state")
		return
	}
	if r.URL.Query().Get("error") != "" {
		s.redirectAuthError(w, r, "denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		s.redirectAuthError(w, r, "failed")
		return
	}
	identity, err := s.oidc.Exchange(ctx, cfg, s.oidcRedirectURL(r, cfg), code, pending.Verifier, pending.Nonce)
	if err != nil {
		s.log.Error("oidc token exchange failed", "err", err)
		s.redirectAuthError(w, r, "failed")
		return
	}
	user, err := s.oidc.Provision(ctx, cfg, identity)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrOIDCConflict):
			s.redirectAuthError(w, r, "conflict")
		case errors.Is(err, auth.ErrOIDCNotProvisioned):
			s.redirectAuthError(w, r, "notprovisioned")
		default:
			s.log.Error("oidc provisioning failed", "err", err)
			s.redirectAuthError(w, r, "failed")
		}
		return
	}
	token, ttl, err := s.auth.SignIn(ctx, user)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.auth.SetCookie(w, token, ttl)

	target := pending.Redirect
	if target == "" {
		target = "/"
	}
	if strings.HasPrefix(target, "/admin") && !user.IsAdmin() {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// oidcRedirectURL uses the configured callback when present, otherwise derives
// it from the current request (honouring X-Forwarded-Proto behind a proxy).
func (s *Server) oidcRedirectURL(r *http.Request, cfg auth.OIDCConfig) string {
	if cfg.RedirectURL != "" {
		return cfg.RedirectURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if s.trustProxy {
		if proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	return scheme + "://" + r.Host + "/api/auth/oidc/callback"
}

func (s *Server) redirectAuthError(w http.ResponseWriter, r *http.Request, code string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/?auth="+url.QueryEscape(code), http.StatusFound)
}

// safeLocalPath only allows same-site paths, preventing an open redirect via
// the redirect parameter.
func safeLocalPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.Contains(raw, "\\") || strings.ContainsAny(raw, "\r\n") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	cleaned := path.Clean(u.Path)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if u.RawQuery != "" {
		cleaned += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		cleaned += "#" + u.Fragment
	}
	if cleaned == "" {
		return "/"
	}
	return cleaned
}
