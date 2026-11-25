package store

import "testing"

func TestRebindPostgres(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`SELECT 1`, `SELECT 1`},
		{`SELECT * FROM users WHERE id = ?`, `SELECT * FROM users WHERE id = $1`},
		{`SELECT * FROM users WHERE a = ? AND b = ?`, `SELECT * FROM users WHERE a = $1 AND b = $2`},
		{`SELECT * FROM cards WHERE name LIKE ? AND lower(tags) LIKE ?`, `SELECT * FROM cards WHERE name LIKE $1 AND lower(tags) LIKE $2`},
		{`SELECT '?' AS q, id FROM users WHERE id = ?`, `SELECT '?' AS q, id FROM users WHERE id = $1`},
		{`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
			`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = excluded.value`},
	}
	for _, tc := range cases {
		if got := rebind(tc.in, true); got != tc.want {
			t.Fatalf("rebind(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got := rebind(tc.in, false); got != tc.in {
			t.Fatalf("sqlite rebind changed query: %q", got)
		}
	}
}

func TestPostgresArgs(t *testing.T) {
	in := []any{true, false, 7, int64(9), "x", []byte("b"), nil}
	out := postgresArgs(in)
	if out[0] != int64(1) || out[1] != int64(0) || out[2] != int64(7) || out[3] != int64(9) {
		t.Fatalf("unexpected numeric conversion: %#v", out)
	}
	if out[4] != "x" || string(out[5].([]byte)) != "b" || out[6] != nil {
		t.Fatalf("unexpected passthrough: %#v", out)
	}
}

func TestIsPostgresDSN(t *testing.T) {
	for _, dsn := range []string{
		"postgres://user:pass@localhost:5432/db",
		"postgresql://user@localhost/db?sslmode=disable",
		"  POSTGRES://localhost/db",
	} {
		if !isPostgresDSN(dsn) {
			t.Fatalf("%q should be detected as PostgreSQL", dsn)
		}
	}
	for _, dsn := range []string{"", "./data/team-nav.db", "file:team.db?cache=shared", "sqlite://x"} {
		if isPostgresDSN(dsn) {
			t.Fatalf("%q should not be detected as PostgreSQL", dsn)
		}
	}
}
