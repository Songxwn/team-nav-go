package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Songxwn/team-nav-go/internal/store"
)

func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "team-nav.db"), "admin", "test-password")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewManager(st, false), st
}

func TestSessionTTLDefaultsTo66Days(t *testing.T) {
	mgr, _ := newTestManager(t)
	got := mgr.TTL(context.Background())
	want := 66 * 24 * time.Hour
	if got != want {
		t.Fatalf("default TTL = %v, want %v", got, want)
	}
}

func TestSessionTTLFollowsSetting(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()

	if err := st.UpdateSettings(ctx, map[string]string{"sessionTTL": "30"}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if got, want := mgr.TTL(ctx), 30*24*time.Hour; got != want {
		t.Fatalf("configured TTL = %v, want %v", got, want)
	}

	// Values outside 1-365 days must fall back to the default.
	for _, invalid := range []string{"0", "366", "abc", ""} {
		if err := st.UpdateSettings(ctx, map[string]string{"sessionTTL": invalid}); err != nil {
			t.Fatalf("update settings %q: %v", invalid, err)
		}
		if got := mgr.TTL(ctx); got != DefaultSessionTTLDays*24*time.Hour {
			t.Fatalf("invalid value %q produced TTL %v", invalid, got)
		}
	}
}
