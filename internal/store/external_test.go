package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Songxwn/team-nav-go/internal/model"
)

func TestExternalUsers(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	const dn = "uid=alice,ou=People,dc=example,dc=com"
	user, err := st.CreateExternalUser(ctx, "ldapalice", "Alice", "member", model.SourceLDAP, dn)
	if err != nil {
		t.Fatalf("create external user: %v", err)
	}
	if user.Source != model.SourceLDAP || user.ExternalID != dn || !user.IsExternal() {
		t.Fatalf("unexpected user: %+v", user)
	}

	found, err := st.UserByExternal(ctx, model.SourceLDAP, dn)
	if err != nil {
		t.Fatalf("user by external: %v", err)
	}
	if found.ID != user.ID || found.Username != "ldapalice" {
		t.Fatalf("unexpected external lookup: %+v", found)
	}

	byName, _, err := st.UserByUsername(ctx, "ldapalice")
	if err != nil {
		t.Fatalf("user by name: %v", err)
	}
	if byName.Source != model.SourceLDAP || !byName.IsExternal() {
		t.Fatalf("expected external source, got %+v", byName)
	}

	if count, err := st.AdminCount(ctx); err != nil || count != 1 {
		t.Fatalf("admin count = %d, %v; want 1", count, err)
	}

	// The same username from a different identity must not take over the row.
	if _, err := st.CreateExternalUser(ctx, "ldapalice", "Other", "member", model.SourceOIDC, "issuer|other"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken for username collision, got %v", err)
	}
	// The same provider identity must not be inserted twice either.
	if _, err := st.CreateExternalUser(ctx, "ldapalice2", "Alice", "member", model.SourceLDAP, dn); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken for external id collision, got %v", err)
	}
}

func TestAuthStateSingleUse(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	state := &AuthState{
		State:     "state-one",
		Nonce:     "nonce",
		Verifier:  "verifier",
		Redirect:  "/admin",
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}
	if err := st.SaveAuthState(ctx, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	got, err := st.TakeAuthState(ctx, "state-one")
	if err != nil {
		t.Fatalf("take state: %v", err)
	}
	if got.Nonce != "nonce" || got.Verifier != "verifier" || got.Redirect != "/admin" {
		t.Fatalf("unexpected state: %+v", got)
	}
	if _, err := st.TakeAuthState(ctx, "state-one"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("state should only be usable once, got %v", err)
	}

	expired := &AuthState{State: "state-old", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	if err := st.SaveAuthState(ctx, expired); err != nil {
		t.Fatalf("save expired state: %v", err)
	}
	if _, err := st.TakeAuthState(ctx, "state-old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired state should be rejected, got %v", err)
	}
}

func TestOpenMigratesLegacyUsersTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL,
			display_name  TEXT NOT NULL DEFAULT '',
			role          TEXT NOT NULL DEFAULT 'member',
			created_at    INTEGER NOT NULL
		)`)
	if err != nil {
		db.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, display_name, role, created_at)
		VALUES ('legacy', 'x', 'Legacy', 'admin', 1)`)
	if err != nil {
		db.Close()
		t.Fatalf("insert legacy user: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	st, err := Open(path, "admin", "test-password")
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	user, _, err := st.UserByUsername(context.Background(), "legacy")
	if err != nil {
		t.Fatalf("legacy user: %v", err)
	}
	if user.Source != model.SourceLocal || user.ExternalID != "" {
		t.Fatalf("expected migrated local account, got %+v", user)
	}
	if _, err := st.UserByExternal(context.Background(), model.SourceLDAP, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected no external match, got %v", err)
	}
}
