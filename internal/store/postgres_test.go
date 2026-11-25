package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Songxwn/team-nav-go/internal/model"
)

// TestPostgresBackend exercises the PostgreSQL dialect against a real server.
// It is skipped unless TEAMNAV_TEST_POSTGRES points at a disposable database.
func TestPostgresBackend(t *testing.T) {
	dsn := os.Getenv("TEAMNAV_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("TEAMNAV_TEST_POSTGRES is not set")
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	if _, err := admin.Exec(`DROP SCHEMA IF EXISTS public CASCADE`); err != nil {
		admin.Close()
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := admin.Exec(`CREATE SCHEMA public`); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	if err := admin.Close(); err != nil {
		t.Fatalf("close admin connection: %v", err)
	}

	st, err := OpenDSN(dsn, "admin", "test-password")
	if err != nil {
		t.Fatalf("open postgres store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if st.Backend() != "postgres" {
		t.Fatalf("backend = %q", st.Backend())
	}
	if st.Path() == "" {
		t.Fatal("expected a database path")
	}
	if strings.Contains(st.Path(), "teamnav:teamnav@") {
		t.Fatalf("path leaks credentials: %q", st.Path())
	}

	ctx := context.Background()
	categories, err := st.Categories(ctx, true)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(categories) < 4 {
		t.Fatalf("expected seeded categories, got %d", len(categories))
	}
	settings, err := st.Settings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if settings["siteName"] == "" {
		t.Fatal("expected seeded settings")
	}

	// Identity tables: generated ids, case-insensitive lookup and collisions.
	user, err := st.CreateUser(ctx, "Alice", "password123", "Alice", "member")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected generated user id")
	}
	found, _, err := st.UserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("case-insensitive lookup: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("lookup returned %d, want %d", found.ID, user.ID)
	}
	if _, err := st.CreateExternalUser(ctx, "ALICE", "Alice", "member", model.SourceLDAP, "uid=alice"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected username collision, got %v", err)
	}

	// Cards, notices and reordering cover the biggest INSERT/UPDATE paths.
	card := &model.Card{
		CategoryID: categories[0].ID,
		Name:       "PG card",
		URL:        "https://example.com",
		IconType:   "text",
		Status:     "approved",
	}
	if err := st.CreateCard(ctx, card); err != nil {
		t.Fatalf("create card: %v", err)
	}
	if card.ID == 0 {
		t.Fatal("expected generated card id")
	}
	if err := st.IncrementClick(ctx, card.ID); err != nil {
		t.Fatalf("increment click: %v", err)
	}
	notice := &model.Notice{Title: "PG notice", Content: "body", Level: "info", Active: true}
	if err := st.CreateNotice(ctx, notice); err != nil {
		t.Fatalf("create notice: %v", err)
	}
	if notice.ID == 0 {
		t.Fatal("expected generated notice id")
	}

	// Images live in BYTEA and are de-duplicated by content hash.
	blob := []byte("\x89PNG\r\n\x1a\npg-image")
	first, created, err := st.SaveImage(ctx, "pg.png", "image/png", "pg-img-1", blob)
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if !created {
		t.Fatal("first image should be created")
	}
	second, created, err := st.SaveImage(ctx, "copy.png", "image/png", "pg-img-2", blob)
	if err != nil {
		t.Fatalf("save duplicate image: %v", err)
	}
	if created || second.ID != first.ID {
		t.Fatalf("expected dedupe, got %+v created=%v", second, created)
	}
	meta, data, err := st.ImageBytes(ctx, first.ID)
	if err != nil {
		t.Fatalf("image bytes: %v", err)
	}
	if string(data) != string(blob) || meta.Size != int64(len(blob)) {
		t.Fatalf("image payload mismatch: %+v", meta)
	}

	if err := st.RecordView(ctx, time.Now().Format("2006-01-02")); err != nil {
		t.Fatalf("record view: %v", err)
	}
	stats, err := st.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Images != 1 || stats.Users != 2 || stats.Cards == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.DatabaseSize <= 0 {
		t.Fatalf("expected database size, got %d", stats.DatabaseSize)
	}

	// OIDC state stays single use on PostgreSQL too.
	if err := st.SaveAuthState(ctx, &AuthState{
		State: "pg-state", Nonce: "n", Verifier: "v", Redirect: "/", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("save auth state: %v", err)
	}
	if _, err := st.TakeAuthState(ctx, "pg-state"); err != nil {
		t.Fatalf("take auth state: %v", err)
	}
	if _, err := st.TakeAuthState(ctx, "pg-state"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("state should be single use, got %v", err)
	}

	if _, err := st.Backup(ctx); !errors.Is(err, ErrBackupUnsupported) {
		t.Fatalf("expected unsupported backup error, got %v", err)
	}
}
