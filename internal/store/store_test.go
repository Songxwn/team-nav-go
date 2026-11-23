package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Songxwn/team-nav-go/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "team-nav.db"), "admin", "test-password")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestSeedData(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	categories, err := st.Categories(ctx, true)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(categories) < 4 {
		t.Fatalf("expected seeded categories, got %d", len(categories))
	}

	cards, err := st.Cards(ctx, CardFilter{Status: "approved"})
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) < 10 {
		t.Fatalf("expected seeded cards, got %d", len(cards))
	}

	settings, err := st.Settings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if settings["siteName"] == "" {
		t.Fatal("expected siteName setting")
	}

	user, _, err := st.UserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("admin user: %v", err)
	}
	if !user.IsAdmin() {
		t.Fatal("seeded user should be admin")
	}
}

func TestCardLifecycle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	categories, err := st.Categories(ctx, true)
	if err != nil || len(categories) == 0 {
		t.Fatalf("categories: %v", err)
	}
	target := categories[0]

	card := &model.Card{
		CategoryID:  target.ID,
		Name:        "  Test Service  ",
		URL:         "https://example.com",
		Description: "a card",
		IconType:    "emoji",
		IconValue:   "🧪",
		Tags:        []string{" test ", "test", "go"},
		Status:      "pending",
	}
	if err := st.CreateCard(ctx, card); err != nil {
		t.Fatalf("create card: %v", err)
	}
	if card.Name != "Test Service" {
		t.Fatalf("name was not normalised: %q", card.Name)
	}
	if len(card.Tags) != 2 {
		t.Fatalf("tags were not deduplicated: %v", card.Tags)
	}
	if card.ID == 0 {
		t.Fatal("expected generated id")
	}

	if err := st.IncrementClick(ctx, card.ID); err == nil {
		t.Fatal("pending cards should not accept clicks")
	}
	card.Status = "approved"
	if err := st.UpdateCard(ctx, card); err != nil {
		t.Fatalf("update card: %v", err)
	}
	if err := st.IncrementClick(ctx, card.ID); err != nil {
		t.Fatalf("increment click: %v", err)
	}
	loaded, err := st.Card(ctx, card.ID)
	if err != nil {
		t.Fatalf("load card: %v", err)
	}
	if loaded.Clicks != 1 {
		t.Fatalf("expected 1 click, got %d", loaded.Clicks)
	}

	stats, err := st.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Cards == 0 || stats.TotalClicks == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	if err := st.DeleteCard(ctx, card.ID); err != nil {
		t.Fatalf("delete card: %v", err)
	}
	if _, err := st.Card(ctx, card.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestImageDedupAndCacheMetadata(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	data := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")

	first, created, err := st.SaveImage(ctx, "logo.png", "image/png", "img-1", data)
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if !created || first.ID != "img-1" {
		t.Fatalf("unexpected first save: %+v created=%v", first, created)
	}
	second, created, err := st.SaveImage(ctx, "duplicate.png", "image/png", "img-2", data)
	if err != nil {
		t.Fatalf("save duplicate: %v", err)
	}
	if created {
		t.Fatal("duplicate image should be de-duplicated")
	}
	if second.ID != first.ID {
		t.Fatalf("expected dedupe to return %q, got %q", first.ID, second.ID)
	}

	meta, blob, err := st.ImageBytes(ctx, first.ID)
	if err != nil {
		t.Fatalf("image bytes: %v", err)
	}
	if string(blob) != string(data) || meta.Size != int64(len(data)) {
		t.Fatal("image payload mismatch")
	}
	if meta.SHA256 == "" {
		t.Fatal("expected sha256 hash")
	}
}

func TestSessionsAndUsers(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	user, err := st.CreateUser(ctx, "alice", "password123", "Alice", "member")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	expires := time.Now().Add(time.Hour).Unix()
	if err := st.CreateSession(ctx, "token-hash", user.ID, expires); err != nil {
		t.Fatalf("create session: %v", err)
	}
	got, err := st.SessionUser(ctx, "token-hash")
	if err != nil {
		t.Fatalf("session user: %v", err)
	}
	if got.ID != user.ID || got.IsAdmin() {
		t.Fatalf("unexpected session user: %+v", got)
	}
	if err := st.DeleteSession(ctx, "token-hash"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := st.SessionUser(ctx, "token-hash"); err != ErrNotFound {
		t.Fatalf("expected expired session to be gone, got %v", err)
	}
}

func TestCategoryTreeAndReorder(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	root := &model.Category{Name: "Root", Icon: "🌱", Visible: true}
	if err := st.CreateCategory(ctx, root); err != nil {
		t.Fatalf("create root: %v", err)
	}
	child := &model.Category{Name: "Child", ParentID: root.ID, Visible: true}
	if err := st.CreateCategory(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	cardA := &model.Card{CategoryID: child.ID, Name: "A", IconType: "text", Status: "approved"}
	cardB := &model.Card{CategoryID: child.ID, Name: "B", IconType: "text", Status: "approved"}
	if err := st.CreateCard(ctx, cardA); err != nil {
		t.Fatalf("create card a: %v", err)
	}
	if err := st.CreateCard(ctx, cardB); err != nil {
		t.Fatalf("create card b: %v", err)
	}
	if err := st.ReorderCards(ctx, child.ID, []int64{cardB.ID, cardA.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	cards, err := st.Cards(ctx, CardFilter{CategoryID: child.ID})
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 2 || cards[0].ID != cardB.ID {
		t.Fatalf("unexpected order: %+v", cards)
	}

	if err := st.DeleteCategory(ctx, root.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	if _, err := st.Category(ctx, child.ID); err != ErrNotFound {
		t.Fatalf("child should be removed with parent, got %v", err)
	}
	if _, err := st.Card(ctx, cardA.ID); err != ErrNotFound {
		t.Fatalf("cards should be removed with category, got %v", err)
	}
}
