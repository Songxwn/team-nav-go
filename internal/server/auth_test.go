package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSafeLocalPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "/"},
		{"/", "/"},
		{"/admin", "/admin"},
		{"/admin?x=1", "/admin?x=1"},
		{"//evil.example", "/"},
		{"https://evil.example/", "/"},
		{"\\\\evil", "/"},
		{"/ok/../admin", "/admin"},
	}
	for _, tc := range cases {
		if got := safeLocalPath(tc.in); got != tc.want {
			t.Errorf("safeLocalPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOIDCStateCookieMatches(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=abc", nil)
	if oidcStateCookieMatches(req, "abc") {
		t.Fatal("missing state cookie must not match")
	}
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	if !oidcStateCookieMatches(req, "abc") {
		t.Fatal("matching state cookie was rejected")
	}
	if oidcStateCookieMatches(req, "other") {
		t.Fatal("mismatched state must not match")
	}
	if oidcStateCookieMatches(req, "") {
		t.Fatal("empty state must not match")
	}
}

func TestPublicSiteSettingsHidesSecrets(t *testing.T) {
	all := map[string]string{
		"siteName":          "Nav",
		"language":          "en",
		"ldapBindPassword":  "secret",
		"oidcClientSecret":  "secret",
		"ldapAddr":          "ldaps://dc.corp.com",
		"oidcIssuer":        "https://id.example.com",
		"oidcClientID":      "nav",
		"unrelatedInternal": "x",
	}
	public := publicSiteSettings(all)
	if public["siteName"] != "Nav" || public["language"] != "en" {
		t.Fatalf("public settings missing values: %v", public)
	}
	for key := range all {
		if key == "siteName" || key == "language" {
			continue
		}
		if _, leaked := public[key]; leaked {
			t.Errorf("public settings leaked %q", key)
		}
	}
}
