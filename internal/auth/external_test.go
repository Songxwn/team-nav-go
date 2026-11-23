package auth

import (
	"strings"
	"testing"
)

func TestSanitizeUsername(t *testing.T) {
	cases := []struct {
		raw, subject, want string
	}{
		{"alice@corp.com", "sub-1", "alice"},
		{"alice.smith", "sub-1", "alice.smith"},
		{"  bob  ", "sub-2", "bob"},
		{"ab", "subject-abcdef", "user-93f0ec73"},
		{"张三", "subject-zhang", "user-1bc109dd"},
		{"a b/c", "subject-4", "a-b-c"},
		{"A_very-long-user-name-that-exceeds-the-limit", "sub-5", "A_very-long-user-name-that-excee"},
	}
	for _, tc := range cases {
		if got := SanitizeUsername(tc.raw, tc.subject); got != tc.want {
			t.Errorf("SanitizeUsername(%q, %q) = %q, want %q", tc.raw, tc.subject, got, tc.want)
		}
	}
}

func TestOIDCConfigDefaults(t *testing.T) {
	cfg := OIDCConfigFromSettings(map[string]string{
		"oidcEnabled":  "1",
		"oidcIssuer":   "https://id.example.com/",
		"oidcClientID": "nav",
	})
	if cfg.Issuer != "https://id.example.com" {
		t.Fatalf("issuer not trimmed: %q", cfg.Issuer)
	}
	if !cfg.Enabled || !cfg.AutoCreate {
		t.Fatalf("expected enabled and auto create")
	}
	if cfg.UsernameClaim != "preferred_username" || cfg.DefaultRole != "member" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if strings.Join(cfg.Scopes, " ") != "openid profile email" {
		t.Fatalf("scopes = %v", cfg.Scopes)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	noOpenID := OIDCConfigFromSettings(map[string]string{
		"oidcEnabled":  "1",
		"oidcIssuer":   "https://id.example.com",
		"oidcClientID": "nav",
		"oidcScopes":   "profile email",
	})
	if noOpenID.Scopes[0] != "openid" {
		t.Fatalf("openid scope not forced: %v", noOpenID.Scopes)
	}

	invalid := OIDCConfigFromSettings(map[string]string{"oidcEnabled": "1"})
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected validation error for missing issuer")
	}
}

func TestLDAPConfigDefaultsAndValidation(t *testing.T) {
	cfg := LDAPConfigFromSettings(map[string]string{
		"ldapEnabled":      "1",
		"ldapAddr":         "ldaps://dc.corp.com",
		"ldapBindDN":       "CN=svc,DC=corp,DC=com",
		"ldapBindPassword": "secret",
		"ldapBaseDN":       "DC=corp,DC=com",
	})
	if !cfg.Enabled || !cfg.AutoCreate {
		t.Fatal("expected enabled and auto create")
	}
	if cfg.UserFilter != "(uid={username})" || cfg.UsernameAttr != "uid" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	noAddr := LDAPConfigFromSettings(map[string]string{"ldapEnabled": "1"})
	if err := noAddr.Validate(); err == nil {
		t.Fatal("expected missing address error")
	}

	noSearchBase := LDAPConfigFromSettings(map[string]string{
		"ldapEnabled":      "1",
		"ldapAddr":         "dc.corp.com:389",
		"ldapBindDN":       "CN=svc,DC=corp,DC=com",
		"ldapBindPassword": "secret",
	})
	if err := noSearchBase.Validate(); err == nil {
		t.Fatal("expected missing base DN error")
	}

	templateOnly := LDAPConfigFromSettings(map[string]string{
		"ldapEnabled": "1",
		"ldapAddr":    "dc.corp.com",
		"ldapUserDN":  "{username}@corp.com",
	})
	if err := templateOnly.Validate(); err != nil {
		t.Fatalf("template only should validate: %v", err)
	}

	badTemplate := LDAPConfigFromSettings(map[string]string{
		"ldapEnabled": "1",
		"ldapAddr":    "dc.corp.com",
		"ldapUserDN":  "uid=fixed,dc=corp,dc=com",
	})
	if err := badTemplate.Validate(); err == nil {
		t.Fatal("expected template placeholder error")
	}
}

func TestLDAPDialAddress(t *testing.T) {
	cases := []struct {
		raw    string
		addr   string
		ldaps  bool
		host   string
		failed bool
	}{
		{"dc.corp.com", "dc.corp.com:389", false, "dc.corp.com", false},
		{"dc.corp.com:636", "dc.corp.com:636", true, "dc.corp.com", false},
		{"ldap://dc.corp.com:389", "dc.corp.com:389", false, "dc.corp.com", false},
		{"ldaps://dc.corp.com", "dc.corp.com:636", true, "dc.corp.com", false},
		{"", "", false, "", true},
	}
	for _, tc := range cases {
		addr, ldaps, host, err := ldapDialAddress(tc.raw)
		if tc.failed {
			if err == nil {
				t.Errorf("ldapDialAddress(%q) expected error", tc.raw)
			}
			continue
		}
		if err != nil || addr != tc.addr || ldaps != tc.ldaps || host != tc.host {
			t.Errorf("ldapDialAddress(%q) = %q, %v, %q, %v; want %q, %v, %q", tc.raw, addr, ldaps, host, err, tc.addr, tc.ldaps, tc.host)
		}
	}
}

func TestCodeChallenge(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := CodeChallenge(verifier); got != want {
		t.Fatalf("CodeChallenge() = %q, want %q", got, want)
	}
}

func TestClaimGroups(t *testing.T) {
	claims := map[string]any{
		"groups":  []any{"TeamNav-Admins", "Everyone"},
		"single":  "TeamNav-Admins",
		"ignored": 42,
	}
	if groups := claimGroups(claims, "groups"); len(groups) != 2 || !containsGroup(groups, "teamnav-admins") {
		t.Fatalf("groups = %v", groups)
	}
	if groups := claimGroups(claims, "single"); len(groups) != 1 || groups[0] != "TeamNav-Admins" {
		t.Fatalf("single = %v", groups)
	}
	if groups := claimGroups(claims, "missing"); groups != nil {
		t.Fatalf("missing = %v", groups)
	}
}
