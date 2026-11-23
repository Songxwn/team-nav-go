package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

// OIDC flow errors.
var (
	ErrOIDCUnavailable    = errors.New("oidc: provider unavailable")
	ErrOIDCNotProvisioned = errors.New("oidc: account not provisioned")
	ErrOIDCConflict       = errors.New("oidc: username conflict")
)

// OIDCConfig is built from the oidc* site settings.
type OIDCConfig struct {
	Enabled          bool
	Name             string
	Issuer           string
	ClientID         string
	ClientSecret     string
	RedirectURL      string
	Scopes           []string
	UsernameClaim    string
	DisplayNameClaim string
	GroupsClaim      string
	AdminGroup       string
	DefaultRole      string
	AutoCreate       bool
}

// OIDCConfigFromSettings reads the SSO configuration.
func OIDCConfigFromSettings(settings map[string]string) OIDCConfig {
	cfg := OIDCConfig{
		Enabled:          settings["oidcEnabled"] == "1",
		Name:             strings.TrimSpace(settings["oidcName"]),
		Issuer:           strings.TrimRight(strings.TrimSpace(settings["oidcIssuer"]), "/"),
		ClientID:         strings.TrimSpace(settings["oidcClientID"]),
		ClientSecret:     settings["oidcClientSecret"],
		RedirectURL:      strings.TrimSpace(settings["oidcRedirectURL"]),
		UsernameClaim:    strings.TrimSpace(settings["oidcUsernameClaim"]),
		DisplayNameClaim: strings.TrimSpace(settings["oidcDisplayNameClaim"]),
		GroupsClaim:      strings.TrimSpace(settings["oidcGroupsClaim"]),
		AdminGroup:       strings.TrimSpace(settings["oidcAdminGroup"]),
		DefaultRole:      settings["oidcDefaultRole"],
		AutoCreate:       settings["oidcAutoCreate"] != "0",
	}
	if cfg.Name == "" {
		cfg.Name = "SSO"
	}
	if cfg.UsernameClaim == "" {
		cfg.UsernameClaim = "preferred_username"
	}
	if cfg.DisplayNameClaim == "" {
		cfg.DisplayNameClaim = "name"
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	if cfg.DefaultRole != "admin" {
		cfg.DefaultRole = "member"
	}
	cfg.Scopes = strings.Fields(settings["oidcScopes"])
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	hasOpenID := false
	for _, scope := range cfg.Scopes {
		if scope == oidc.ScopeOpenID {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		cfg.Scopes = append([]string{oidc.ScopeOpenID}, cfg.Scopes...)
	}
	return cfg
}

// Validate reports configuration that cannot start an SSO flow.
func (c OIDCConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Issuer == "" {
		return errors.New("SSO Issuer 不能为空")
	}
	u, err := url.Parse(c.Issuer)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("SSO Issuer 必须是 http/https 地址")
	}
	if c.ClientID == "" {
		return errors.New("SSO Client ID 不能为空")
	}
	if c.RedirectURL != "" {
		r, err := url.Parse(c.RedirectURL)
		if err != nil || (r.Scheme != "http" && r.Scheme != "https") || r.Host == "" {
			return errors.New("SSO 回调地址必须是 http/https 地址")
		}
	}
	return nil
}

// OIDCIdentity is the provider neutral result of a successful token exchange.
type OIDCIdentity struct {
	Subject     string
	Username    string
	DisplayName string
	Groups      []string
}

// OIDCAuthenticator runs the authorization code flow and provisions users.
type OIDCAuthenticator struct {
	store *store.Store

	mu    sync.Mutex
	cache map[string]*oidc.Provider
}

// NewOIDCAuthenticator wires the SSO authenticator.
func NewOIDCAuthenticator(s *store.Store) *OIDCAuthenticator {
	return &OIDCAuthenticator{store: s, cache: make(map[string]*oidc.Provider)}
}

func (a *OIDCAuthenticator) provider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	a.mu.Lock()
	cached := a.cache[issuer]
	a.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("%w: discovery: %v", ErrOIDCUnavailable, err)
	}
	a.mu.Lock()
	a.cache[issuer] = provider
	a.mu.Unlock()
	return provider, nil
}

func (a *OIDCAuthenticator) oauthConfig(ctx context.Context, cfg OIDCConfig, redirectURL string) (*oauth2.Config, *oidc.Provider, error) {
	provider, err := a.provider(ctx, cfg.Issuer)
	if err != nil {
		return nil, nil, err
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}, provider, nil
}

// AuthCodeURL builds the browser redirect for the authorization code flow with
// a nonce and PKCE challenge.
func (a *OIDCAuthenticator) AuthCodeURL(ctx context.Context, cfg OIDCConfig, redirectURL, state, nonce, verifier string) (string, error) {
	oauthCfg, _, err := a.oauthConfig(ctx, cfg, redirectURL)
	if err != nil {
		return "", err
	}
	return oauthCfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", CodeChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

// Exchange completes the flow and validates the ID token signature, issuer,
// audience and nonce.
func (a *OIDCAuthenticator) Exchange(ctx context.Context, cfg OIDCConfig, redirectURL, code, verifier, expectedNonce string) (*OIDCIdentity, error) {
	oauthCfg, provider, err := a.oauthConfig(ctx, cfg, redirectURL)
	if err != nil {
		return nil, err
	}
	token, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("token response has no id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}
	if expectedNonce != "" && idToken.Nonce != expectedNonce {
		return nil, errors.New("id_token nonce mismatch")
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	username := claimString(claims, cfg.UsernameClaim)
	if username == "" {
		username = claimString(claims, "email")
	}
	if username == "" {
		username = idToken.Subject
	}
	displayName := claimString(claims, cfg.DisplayNameClaim)
	if displayName == "" {
		displayName = claimString(claims, "preferred_username")
	}
	if displayName == "" {
		displayName = username
	}
	return &OIDCIdentity{
		Subject:     idToken.Subject,
		Username:    username,
		DisplayName: displayName,
		Groups:      claimGroups(claims, cfg.GroupsClaim),
	}, nil
}

// Provision maps an SSO identity to a local account, creating or refreshing it
// according to the settings.
func (a *OIDCAuthenticator) Provision(ctx context.Context, cfg OIDCConfig, ident *OIDCIdentity) (*model.User, error) {
	if ident == nil || ident.Subject == "" {
		return nil, errors.New("identity has no subject")
	}
	externalID := cfg.Issuer + "|" + ident.Subject
	handle := SanitizeUsername(ident.Username, ident.Subject)
	displayName := strings.TrimSpace(ident.DisplayName)
	if displayName == "" {
		displayName = handle
	}
	role := cfg.DefaultRole
	if cfg.AdminGroup != "" && containsGroup(ident.Groups, cfg.AdminGroup) {
		role = "admin"
	}

	existing, err := a.store.UserByExternal(ctx, model.SourceOIDC, externalID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		if cfg.AdminGroup == "" {
			role = existing.Role
		}
		role = a.keepLastAdmin(ctx, existing.ID, existing.Role, role)
		if err := a.store.UpdateExternalUser(ctx, existing.ID, displayName, role, model.SourceOIDC, externalID); err != nil {
			return nil, err
		}
		existing.DisplayName, existing.Role, existing.Source, existing.ExternalID = displayName, role, model.SourceOIDC, externalID
		return existing, nil
	}

	byName, _, nameErr := a.store.UserByUsername(ctx, handle)
	switch {
	case nameErr == nil:
		if byName.Source != model.SourceOIDC {
			return nil, ErrOIDCConflict
		}
		if byName.ExternalID != "" && byName.ExternalID != externalID {
			return nil, ErrOIDCConflict
		}
		if cfg.AdminGroup == "" {
			role = byName.Role
		}
		role = a.keepLastAdmin(ctx, byName.ID, byName.Role, role)
		if err := a.store.UpdateExternalUser(ctx, byName.ID, displayName, role, model.SourceOIDC, externalID); err != nil {
			return nil, err
		}
		byName.DisplayName, byName.Role, byName.Source, byName.ExternalID = displayName, role, model.SourceOIDC, externalID
		return byName, nil
	case !errors.Is(nameErr, store.ErrNotFound):
		return nil, nameErr
	}

	if !cfg.AutoCreate {
		return nil, ErrOIDCNotProvisioned
	}
	user, err := a.store.CreateExternalUser(ctx, handle, displayName, role, model.SourceOIDC, externalID)
	if errors.Is(err, store.ErrUsernameTaken) {
		if again, findErr := a.store.UserByExternal(ctx, model.SourceOIDC, externalID); findErr == nil {
			return again, nil
		}
		return nil, ErrOIDCConflict
	}
	return user, err
}

// keepLastAdmin prevents an SSO group change from removing the final
// administrator account.
func (a *OIDCAuthenticator) keepLastAdmin(ctx context.Context, userID int64, current, next string) string {
	if next == "admin" || current != "admin" || userID == 0 {
		return next
	}
	if count, err := a.store.AdminCount(ctx); err == nil && count <= 1 {
		return "admin"
	}
	return next
}

// RandomToken returns a URL safe random string with n bytes of entropy.
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// CodeChallenge derives the S256 PKCE challenge for a verifier.
func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
