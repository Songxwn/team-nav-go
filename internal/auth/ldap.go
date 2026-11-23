package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
)

// Errors returned by the LDAP authenticator. The server maps them to HTTP
// responses without leaking directory details.
var (
	ErrLDAPInvalidCredentials = errors.New("ldap: invalid credentials")
	ErrLDAPUnavailable        = errors.New("ldap: provider unavailable")
	ErrLDAPNotProvisioned     = errors.New("ldap: account not provisioned")
	ErrLDAPConflict           = errors.New("ldap: username conflict")
)

// LDAPConfig is built from the ldap* site settings.
type LDAPConfig struct {
	Enabled         bool
	Addr            string
	StartTLS        bool
	SkipVerify      bool
	BindDN          string
	BindPassword    string
	BaseDN          string
	UserFilter      string
	UserDNTemplate  string
	UsernameAttr    string
	DisplayNameAttr string
	AdminGroup      string
	DefaultRole     string
	AutoCreate      bool
}

// LDAPConfigFromSettings reads the directory configuration.
func LDAPConfigFromSettings(settings map[string]string) LDAPConfig {
	cfg := LDAPConfig{
		Enabled:         settings["ldapEnabled"] == "1",
		Addr:            strings.TrimSpace(settings["ldapAddr"]),
		StartTLS:        settings["ldapStartTLS"] == "1",
		SkipVerify:      settings["ldapSkipVerify"] == "1",
		BindDN:          strings.TrimSpace(settings["ldapBindDN"]),
		BindPassword:    settings["ldapBindPassword"],
		BaseDN:          strings.TrimSpace(settings["ldapBaseDN"]),
		UserFilter:      strings.TrimSpace(settings["ldapUserFilter"]),
		UserDNTemplate:  strings.TrimSpace(settings["ldapUserDN"]),
		UsernameAttr:    strings.TrimSpace(settings["ldapUsernameAttr"]),
		DisplayNameAttr: strings.TrimSpace(settings["ldapDisplayNameAttr"]),
		AdminGroup:      strings.TrimSpace(settings["ldapAdminGroup"]),
		DefaultRole:     settings["ldapDefaultRole"],
		AutoCreate:      settings["ldapAutoCreate"] != "0",
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(uid={username})"
	}
	if cfg.UsernameAttr == "" {
		cfg.UsernameAttr = "uid"
	}
	if cfg.DisplayNameAttr == "" {
		cfg.DisplayNameAttr = "displayName"
	}
	if cfg.DefaultRole != "admin" {
		cfg.DefaultRole = "member"
	}
	return cfg
}

// Validate reports configuration that would make every login fail.
func (c LDAPConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Addr == "" {
		return errors.New("LDAP 地址不能为空")
	}
	if c.BindDN == "" && c.UserDNTemplate == "" {
		return errors.New("请填写服务账号或用户 DN 模板")
	}
	if c.BindDN != "" && c.UserDNTemplate == "" && c.BindPassword == "" {
		return errors.New("请填写服务账号密码")
	}
	if c.UserDNTemplate != "" && !strings.Contains(c.UserDNTemplate, "{username}") {
		return errors.New("用户 DN 模板需包含 {username}")
	}
	if c.BindDN != "" && c.UserDNTemplate == "" && c.BaseDN == "" {
		return errors.New("使用目录搜索时必须填写 Base DN")
	}
	return nil
}

// LDAPAuthenticator checks directory credentials and provisions local users.
type LDAPAuthenticator struct {
	store *store.Store
}

// NewLDAPAuthenticator wires the directory authenticator.
func NewLDAPAuthenticator(s *store.Store) *LDAPAuthenticator {
	return &LDAPAuthenticator{store: s}
}

// Authenticate performs a bind as the user (after an optional service account
// search) and maps the directory entry to a local account.
func (a *LDAPAuthenticator) Authenticate(ctx context.Context, cfg LDAPConfig, username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if !cfg.Enabled {
		return nil, ErrLDAPUnavailable
	}
	// An empty password would turn the second bind into an anonymous bind,
	// which many directories accept. Reject it before touching the wire.
	if username == "" || password == "" {
		return nil, ErrLDAPInvalidCredentials
	}
	addr, ldaps, host, err := ldapDialAddress(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPUnavailable, err)
	}
	tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.SkipVerify}
	scheme := "ldap"
	if ldaps {
		scheme = "ldaps"
	}
	conn, err := ldap.DialURL(scheme+"://"+addr,
		ldap.DialWithDialer(&net.Dialer{Timeout: 6 * time.Second}),
		ldap.DialWithTLSConfig(tlsCfg))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPUnavailable, err)
	}
	defer conn.Close()
	conn.SetTimeout(8 * time.Second)
	if !ldaps && cfg.StartTLS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			return nil, fmt.Errorf("%w: starttls: %v", ErrLDAPUnavailable, err)
		}
	}

	// Step 1: locate the directory entry.
	var entry *ldap.Entry
	userDN := ""
	if cfg.UserDNTemplate != "" {
		userDN = strings.ReplaceAll(cfg.UserDNTemplate, "{username}", ldap.EscapeDN(username))
	}
	if cfg.BindDN != "" && userDN == "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("%w: service bind: %v", ErrLDAPUnavailable, err)
		}
		filter := strings.ReplaceAll(cfg.UserFilter, "{username}", ldap.EscapeFilter(username))
		entry, err = searchOne(conn, cfg.BaseDN, filter)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, ErrLDAPInvalidCredentials
		}
		userDN = entry.DN
	}
	if userDN == "" {
		return nil, fmt.Errorf("%w: no user DN configured", ErrLDAPUnavailable)
	}

	// Step 2: bind with the supplied password.
	if err := conn.Bind(userDN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return nil, ErrLDAPInvalidCredentials
		}
		return nil, fmt.Errorf("%w: user bind: %v", ErrLDAPUnavailable, err)
	}

	// Step 3: read attributes when the service search did not provide them.
	if entry == nil {
		entry, _ = readEntry(conn, userDN)
	}

	usernameAttr := firstValue(entry, cfg.UsernameAttr)
	if usernameAttr == "" {
		usernameAttr = username
	}
	handle := SanitizeUsername(usernameAttr, userDN)
	displayName := firstValue(entry, cfg.DisplayNameAttr)
	if displayName == "" {
		displayName = firstValue(entry, "cn")
	}
	if displayName == "" {
		displayName = handle
	}

	role := cfg.DefaultRole
	existing, lookupErr := a.store.UserByExternal(ctx, model.SourceLDAP, userDN)
	if lookupErr != nil && !errors.Is(lookupErr, store.ErrNotFound) {
		return nil, lookupErr
	}
	if existing == nil {
		byName, _, nameErr := a.store.UserByUsername(ctx, handle)
		switch {
		case nameErr == nil:
			if byName.Source != model.SourceLDAP {
				return nil, ErrLDAPConflict
			}
			if byName.ExternalID != "" && !strings.EqualFold(byName.ExternalID, userDN) {
				return nil, ErrLDAPConflict
			}
			existing = byName
			role = a.resolveRole(ctx, cfg, byName.ID, byName.Role, entry)
			if err := a.store.UpdateExternalUser(ctx, byName.ID, displayName, role, model.SourceLDAP, userDN); err != nil {
				return nil, err
			}
			byName.DisplayName, byName.Role, byName.Source, byName.ExternalID = displayName, role, model.SourceLDAP, userDN
			return byName, nil
		case !errors.Is(nameErr, store.ErrNotFound):
			return nil, nameErr
		}
	}
	if existing != nil {
		role = a.resolveRole(ctx, cfg, existing.ID, existing.Role, entry)
		if err := a.store.UpdateExternalUser(ctx, existing.ID, displayName, role, model.SourceLDAP, userDN); err != nil {
			return nil, err
		}
		existing.DisplayName, existing.Role, existing.Source, existing.ExternalID = displayName, role, model.SourceLDAP, userDN
		return existing, nil
	}
	if !cfg.AutoCreate {
		return nil, ErrLDAPNotProvisioned
	}
	role = a.resolveRole(ctx, cfg, 0, "", entry)
	user, err := a.store.CreateExternalUser(ctx, handle, displayName, role, model.SourceLDAP, userDN)
	if errors.Is(err, store.ErrUsernameTaken) {
		// A concurrent first login may have created the row; reload it.
		if again, findErr := a.store.UserByExternal(ctx, model.SourceLDAP, userDN); findErr == nil {
			return again, nil
		}
		return nil, ErrLDAPConflict
	}
	return user, err
}

func (a *LDAPAuthenticator) resolveRole(ctx context.Context, cfg LDAPConfig, userID int64, existingRole string, entry *ldap.Entry) string {
	role := cfg.DefaultRole
	if cfg.AdminGroup == "" {
		if existingRole == "admin" || existingRole == "member" {
			role = existingRole
		}
	} else if entry != nil {
		for _, group := range entry.GetAttributeValues("memberOf") {
			if strings.EqualFold(strings.TrimSpace(group), cfg.AdminGroup) {
				role = "admin"
				break
			}
		}
	} else if existingRole == "admin" || existingRole == "member" {
		// The entry could not be read (for example with direct DN binding and
		// no self-read permission). Keep the role instead of demoting by guess.
		role = existingRole
	}
	if role != "admin" && existingRole == "admin" && userID > 0 {
		// Never remove the last administrator by a directory sync.
		if count, err := a.store.AdminCount(ctx); err == nil && count <= 1 {
			return "admin"
		}
	}
	return role
}

func searchOne(conn *ldap.Conn, base, filter string) (*ldap.Entry, error) {
	req := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		2, 8, false, filter, []string{"*", "+"}, nil)
	res, err := conn.Search(req)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: search: %v", ErrLDAPUnavailable, err)
	}
	if len(res.Entries) == 0 {
		return nil, nil
	}
	if len(res.Entries) > 1 {
		// Ambiguous filters are treated as a failed login rather than guessing.
		return nil, ErrLDAPInvalidCredentials
	}
	return res.Entries[0], nil
}

func readEntry(conn *ldap.Conn, dn string) (*ldap.Entry, error) {
	req := ldap.NewSearchRequest(dn, ldap.ScopeBaseObject, ldap.NeverDerefAliases,
		1, 8, false, "(objectClass=*)", []string{"*", "+"}, nil)
	res, err := conn.Search(req)
	if err != nil {
		return nil, err
	}
	if len(res.Entries) == 0 {
		return nil, nil
	}
	return res.Entries[0], nil
}

func firstValue(entry *ldap.Entry, attr string) string {
	if entry == nil || attr == "" {
		return ""
	}
	return strings.TrimSpace(entry.GetAttributeValue(attr))
}

// ldapDialAddress accepts host:port, ldap://host, and ldaps://host forms and
// returns the dial address, whether TLS is required, and the host name.
func ldapDialAddress(raw string) (addr string, ldaps bool, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, "", errors.New("empty address")
	}
	if strings.Contains(raw, "://") {
		u, perr := url.Parse(raw)
		if perr != nil || u.Host == "" {
			return "", false, "", errors.New("invalid address")
		}
		raw = u.Host
		ldaps = strings.EqualFold(u.Scheme, "ldaps")
	}
	if h, p, splitErr := net.SplitHostPort(raw); splitErr == nil {
		host = h
		if p == "636" {
			ldaps = true
		}
		return net.JoinHostPort(h, p), ldaps, h, nil
	}
	host = strings.Trim(raw, "[]")
	port := "389"
	if ldaps {
		port = "636"
	}
	return net.JoinHostPort(host, port), ldaps, host, nil
}

// SanitizeUsername turns a provider supplied identifier into a valid local
// handle: 3-32 characters from [A-Za-z0-9_.-], with a stable fallback derived
// from the subject when the value cannot be used.
func SanitizeUsername(raw, subject string) string {
	raw = strings.TrimSpace(raw)
	if at := strings.IndexByte(raw, '@'); at > 0 {
		raw = raw[:at]
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '\\' || r == '/' || r == ':':
			b.WriteByte('-')
		}
		if b.Len() >= 32 {
			break
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if len(out) >= 3 {
		return out
	}
	suffix := shortHash(subject)
	return "user-" + suffix
}

func shortHash(value string) string {
	sum := sha256Sum(value)
	return sum[:8]
}
