package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"github.com/Songxwn/team-nav-go/internal/model"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrUsernameTaken is returned when an external account maps to a username
// that already belongs to another identity source.
var ErrUsernameTaken = errors.New("username already taken")

// ErrBackupUnsupported is returned when a one-click snapshot is requested on a
// backend that does not support it.
var ErrBackupUnsupported = errors.New("backup is only available for the SQLite backend")

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'member',
	source        TEXT NOT NULL DEFAULT 'local',
	external_id   TEXT NOT NULL DEFAULT '',
	created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS categories (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL,
	icon        TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	parent_id   INTEGER NOT NULL DEFAULT 0,
	sort        INTEGER NOT NULL DEFAULT 0,
	visible     INTEGER NOT NULL DEFAULT 1,
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id, sort);

CREATE TABLE IF NOT EXISTS cards (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	name        TEXT NOT NULL,
	url         TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	icon_type   TEXT NOT NULL DEFAULT 'text',
	icon_value  TEXT NOT NULL DEFAULT '',
	tags        TEXT NOT NULL DEFAULT '[]',
	sort        INTEGER NOT NULL DEFAULT 0,
	status      TEXT NOT NULL DEFAULT 'approved',
	clicks      INTEGER NOT NULL DEFAULT 0,
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cards_category ON cards(category_id, sort);
CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(status);

CREATE TABLE IF NOT EXISTS notices (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	title      TEXT NOT NULL DEFAULT '',
	content    TEXT NOT NULL DEFAULT '',
	level      TEXT NOT NULL DEFAULT 'info',
	link       TEXT NOT NULL DEFAULT '',
	active     INTEGER NOT NULL DEFAULT 1,
	sort       INTEGER NOT NULL DEFAULT 0,
	expires_at INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS images (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL DEFAULT '',
	mime       TEXT NOT NULL DEFAULT 'application/octet-stream',
	size       INTEGER NOT NULL DEFAULT 0,
	sha256     TEXT NOT NULL,
	data       BLOB NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_sha ON images(sha256);

CREATE TABLE IF NOT EXISTS daily_stats (
	day   TEXT PRIMARY KEY,
	views INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS auth_states (
	state      TEXT PRIMARY KEY,
	nonce      TEXT NOT NULL,
	verifier   TEXT NOT NULL,
	redirect   TEXT NOT NULL DEFAULT '/',
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_states_expiry ON auth_states(expires_at);
`

// postgresSchema mirrors schema using PostgreSQL types. Integer flags stay as
// INTEGER so the Go models keep scanning them directly; identifiers, timestamps
// and byte sizes use BIGINT, and image payloads live in BYTEA.
const postgresSchema = `
CREATE TABLE IF NOT EXISTS users (
	id            BIGSERIAL PRIMARY KEY,
	username      TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'member',
	source        TEXT NOT NULL DEFAULT 'local',
	external_id   TEXT NOT NULL DEFAULT '',
	created_at    BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (lower(username));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external ON users(source, external_id) WHERE external_id <> '';

CREATE TABLE IF NOT EXISTS categories (
	id          BIGSERIAL PRIMARY KEY,
	name        TEXT NOT NULL,
	icon        TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	parent_id   BIGINT NOT NULL DEFAULT 0,
	sort        INTEGER NOT NULL DEFAULT 0,
	visible     INTEGER NOT NULL DEFAULT 1,
	created_at  BIGINT NOT NULL,
	updated_at  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id, sort);

CREATE TABLE IF NOT EXISTS cards (
	id          BIGSERIAL PRIMARY KEY,
	category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	name        TEXT NOT NULL,
	url         TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	icon_type   TEXT NOT NULL DEFAULT 'text',
	icon_value  TEXT NOT NULL DEFAULT '',
	tags        TEXT NOT NULL DEFAULT '[]',
	sort        INTEGER NOT NULL DEFAULT 0,
	status      TEXT NOT NULL DEFAULT 'approved',
	clicks      BIGINT NOT NULL DEFAULT 0,
	created_at  BIGINT NOT NULL,
	updated_at  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cards_category ON cards(category_id, sort);
CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(status);

CREATE TABLE IF NOT EXISTS notices (
	id         BIGSERIAL PRIMARY KEY,
	title      TEXT NOT NULL DEFAULT '',
	content    TEXT NOT NULL DEFAULT '',
	level      TEXT NOT NULL DEFAULT 'info',
	link       TEXT NOT NULL DEFAULT '',
	active     INTEGER NOT NULL DEFAULT 1,
	sort       INTEGER NOT NULL DEFAULT 0,
	expires_at BIGINT NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL,
	updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at BIGINT NOT NULL,
	created_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS images (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL DEFAULT '',
	mime       TEXT NOT NULL DEFAULT 'application/octet-stream',
	size       BIGINT NOT NULL DEFAULT 0,
	sha256     TEXT NOT NULL,
	data       BYTEA NOT NULL,
	created_at BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_sha ON images(sha256);

CREATE TABLE IF NOT EXISTS daily_stats (
	day   TEXT PRIMARY KEY,
	views BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS auth_states (
	state      TEXT PRIMARY KEY,
	nonce      TEXT NOT NULL,
	verifier   TEXT NOT NULL,
	redirect   TEXT NOT NULL DEFAULT '/',
	expires_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_states_expiry ON auth_states(expires_at);
`

// conn rebinds SQLite style placeholders for PostgreSQL and hides the dialect
// from the rest of the store.
type conn struct {
	*sql.DB
	postgres bool
}

type tx struct {
	*sql.Tx
	postgres bool
}

// rebind rewrites ? placeholders into $1, $2, ... while leaving quoted text
// alone. SQLite queries are returned untouched.
func rebind(query string, postgres bool) string {
	if !postgres {
		return query
	}
	var (
		b        strings.Builder
		n        int
		inSingle bool
		inDouble bool
	)
	b.Grow(len(query) + 8)
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			b.WriteByte(c)
		case c == '"' && !inSingle:
			inDouble = !inDouble
			b.WriteByte(c)
		case c == '?' && !inSingle && !inDouble:
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// postgresArgs normalizes Go values for PostgreSQL. Flag columns are INTEGER in
// both schemas, so booleans must travel as integers, and integer widths are
// widened to BIGINT to avoid per-query casts.
func postgresArgs(args []any) []any {
	out := make([]any, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case bool:
			if v {
				out[i] = int64(1)
			} else {
				out[i] = int64(0)
			}
		case int:
			out[i] = int64(v)
		case int8:
			out[i] = int64(v)
		case int16:
			out[i] = int64(v)
		case int32:
			out[i] = int64(v)
		case uint:
			out[i] = int64(v)
		case uint8:
			out[i] = int64(v)
		case uint16:
			out[i] = int64(v)
		case uint32:
			out[i] = int64(v)
		case uint64:
			out[i] = int64(v)
		default:
			out[i] = arg
		}
	}
	return out
}

func (c *conn) args(args []any) []any {
	if !c.postgres {
		return args
	}
	return postgresArgs(args)
}

func (c *conn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.DB.ExecContext(ctx, rebind(query, c.postgres), c.args(args)...)
}

func (c *conn) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.DB.QueryContext(ctx, rebind(query, c.postgres), c.args(args)...)
}

func (c *conn) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.DB.QueryRowContext(ctx, rebind(query, c.postgres), c.args(args)...)
}

func (c *conn) BeginTx(ctx context.Context, opts *sql.TxOptions) (*tx, error) {
	t, err := c.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &tx{Tx: t, postgres: c.postgres}, nil
}

func (t *tx) args(args []any) []any {
	if !t.postgres {
		return args
	}
	return postgresArgs(args)
}

func (t *tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, rebind(query, t.postgres), t.args(args)...)
}

func (t *tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, rebind(query, t.postgres), t.args(args)...)
}

func (t *tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, rebind(query, t.postgres), t.args(args)...)
}

// runner is implemented by both conn and tx.
type runner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store wraps the database handle.
type Store struct {
	db       *conn
	path     string
	postgres bool
}

// Open opens (and creates when needed) a SQLite database and applies the schema
// plus seed data.
func Open(path, adminUser, adminPass string) (*Store, error) {
	return OpenDSN(path, adminUser, adminPass)
}

// OpenDSN opens either SQLite (a file path) or PostgreSQL (a postgres:// or
// postgresql:// URL) and prepares the schema and seed data.
func OpenDSN(dsn, adminUser, adminPass string) (*Store, error) {
	dsn = strings.TrimSpace(dsn)
	if isPostgresDSN(dsn) {
		return openPostgres(dsn, adminUser, adminPass)
	}
	return openSQLite(dsn, adminUser, adminPass)
}

func isPostgresDSN(dsn string) bool {
	lower := strings.ToLower(strings.TrimSpace(dsn))
	return strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://")
}

func openSQLite(path, adminUser, adminPass string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s := &Store{db: &conn{DB: db}, path: path}
	if err := s.execScript(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(adminUser, adminPass); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func openPostgres(dsn, adminUser, adminPass string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	s := &Store{db: &conn{DB: db, postgres: true}, path: redactDSN(dsn), postgres: true}
	if err := s.execScript(ctx, postgresSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(adminUser, adminPass); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// execScript runs a multi statement schema. PostgreSQL rejects multiple
// commands in a prepared statement, so statements are always split.
func (s *Store) execScript(ctx context.Context, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// insertID inserts a row and returns its generated identifier. PostgreSQL does
// not support LastInsertId, so it appends RETURNING id there.
func (s *Store) insertID(ctx context.Context, r runner, query string, args ...any) (int64, error) {
	if s.postgres {
		var id int64
		if err := r.QueryRowContext(ctx, query+" RETURNING id", args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	res, err := r.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "postgres"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the SQLite database file location, or the redacted PostgreSQL
// connection string.
func (s *Store) Path() string { return s.path }

// Backend reports the active database engine ("sqlite" or "postgres").
func (s *Store) Backend() string {
	if s.postgres {
		return "postgres"
	}
	return "sqlite"
}

// migrate upgrades databases created by older releases. Columns added after
// the initial schema are appended when missing.
func (s *Store) migrate(ctx context.Context) error {
	if s.postgres {
		for _, stmt := range []string{
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local'`,
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id TEXT NOT NULL DEFAULT ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (lower(username))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external ON users(source, external_id) WHERE external_id <> ''`,
		} {
			if _, err := s.db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrate users table: %w", err)
			}
		}
		_, _ = s.db.ExecContext(ctx, `DELETE FROM auth_states WHERE expires_at < ?`, time.Now().Unix())
		return nil
	}

	cols := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(users)`)
	if err != nil {
		return fmt.Errorf("inspect users table: %w", err)
	}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		cols[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !cols["source"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN source TEXT NOT NULL DEFAULT 'local'`); err != nil {
			return fmt.Errorf("add users.source: %w", err)
		}
	}
	if !cols["external_id"] {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN external_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add users.external_id: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external ON users(source, external_id) WHERE external_id <> ''`); err != nil {
		return fmt.Errorf("index users external id: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (lower(username))`); err != nil {
		return fmt.Errorf("index users username: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM auth_states WHERE expires_at < ?`, time.Now().Unix())
	return nil
}

func (s *Store) seed(adminUser, adminPass string) error {
	ctx := context.Background()
	now := time.Now().Unix()

	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "admin123"
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO users (username, password_hash, display_name, role, created_at) VALUES (?, ?, ?, 'admin', ?)`,
			adminUser, string(hash), "管理员", now)
		if err != nil {
			return fmt.Errorf("seed admin: %w", err)
		}
	}

	defaults := map[string]string{
		"siteName":       "星云导航",
		"siteSubtitle":   "团队服务，一站式入口",
		"siteLogo":       "",
		"footerText":     "Built with Go · SQLite · Cocoon UI",
		"welcomeText":    "今天也要高效出发",
		"allowApply":     "1",
		"showClicks":     "1",
		"noticeInterval": "8",
		"sessionTTL":     "66",
		"accent":         "indigo",
		"language":       "zh",

		"ldapEnabled":         "0",
		"ldapAddr":            "",
		"ldapStartTLS":        "0",
		"ldapSkipVerify":      "0",
		"ldapBindDN":          "",
		"ldapBindPassword":    "",
		"ldapBaseDN":          "",
		"ldapUserFilter":      "(uid={username})",
		"ldapUserDN":          "",
		"ldapUsernameAttr":    "uid",
		"ldapDisplayNameAttr": "displayName",
		"ldapAdminGroup":      "",
		"ldapDefaultRole":     "member",
		"ldapAutoCreate":      "1",

		"oidcEnabled":          "0",
		"oidcName":             "SSO",
		"oidcIssuer":           "",
		"oidcClientID":         "",
		"oidcClientSecret":     "",
		"oidcRedirectURL":      "",
		"oidcScopes":           "openid profile email",
		"oidcUsernameClaim":    "preferred_username",
		"oidcDisplayNameClaim": "name",
		"oidcGroupsClaim":      "groups",
		"oidcAdminGroup":       "",
		"oidcDefaultRole":      "member",
		"oidcAutoCreate":       "1",
	}
	for k, v := range defaults {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT DO NOTHING`, k, v); err != nil {
			return err
		}
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if err := s.seedContent(ctx, now); err != nil {
			return err
		}
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notices`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO notices (title, content, level, link, active, sort, created_at, updated_at) VALUES (?, ?, 'info', '', 1, 0, ?, ?)`,
			"欢迎使用星云导航 ✨", "在左侧选择分类，或按下 Ctrl + K 搜索全部服务。登录后可以收藏常用网址、提交新的卡片。", now, now)
		if err != nil {
			return err
		}
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now)
	return nil
}

type seedCategory struct {
	name, icon, desc string
	children         []seedCategory
	cards            []seedCard
}
type seedCard struct {
	name, url, desc, icon, color string
	tags                         []string
}

func (s *Store) seedContent(ctx context.Context, now int64) error {
	seed := []seedCategory{
		{
			name: "研发中心", icon: "💻", desc: "代码、构建与研发效率",
			children: []seedCategory{
				{
					name: "代码仓库", icon: "🐙", desc: "源码托管与代码评审",
					cards: []seedCard{
						{"GitHub", "https://github.com", "全球开发者社区与代码托管平台", "🐙", "#1f2937", []string{"代码", "开源"}},
						{"GitLab", "https://gitlab.com", "DevOps 一体化平台", "🦊", "#f97316", []string{"代码", "CI"}},
						{"Gitea", "https://gitea.com", "轻量级自托管 Git 服务", "🍵", "#16a34a", []string{"代码", "自托管"}},
					},
				},
				{
					name: "开发工具", icon: "🛠️", desc: "日常研发工具链",
					cards: []seedCard{
						{"Nexus", "https://www.sonatype.com/products/sonatype-nexus-repository", "私有制品仓库", "📦", "#2563eb", []string{"制品"}},
						{"Harbor", "https://goharbor.io", "云原生镜像仓库", "⚓", "#0ea5e9", []string{"镜像", "K8s"}},
						{"SonarQube", "https://www.sonarsource.com/products/sonarqube/", "代码质量与安全扫描", "🔍", "#4f46e5", []string{"质量"}},
					},
				},
				{
					name: "API 文档", icon: "📘", desc: "接口设计与调试",
					cards: []seedCard{
						{"Apifox", "https://apifox.com", "API 设计、调试、Mock 一体化", "🦉", "#06b6d4", []string{"API"}},
						{"Swagger", "https://swagger.io", "OpenAPI 规范与在线文档", "🧩", "#85ea2d", []string{"API", "文档"}},
					},
				},
			},
		},
		{
			name: "运维中心", icon: "🛰️", desc: "可观测性与基础设施",
			children: []seedCategory{
				{
					name: "监控告警", icon: "📈", desc: "指标、日志与链路",
					cards: []seedCard{
						{"Grafana", "https://grafana.com", "可视化监控面板", "📊", "#f59e0b", []string{"监控", "可视化"}},
						{"Prometheus", "https://prometheus.io", "云原生指标监控系统", "🔥", "#e11d48", []string{"监控", "指标"}},
						{"Kibana", "https://www.elastic.co/kibana", "日志检索与可视化", "🔎", "#ec4899", []string{"日志"}},
					},
				},
				{
					name: "基础设施", icon: "🖥️", desc: "网络、主机与资源",
					cards: []seedCard{
						{"NetBox", "https://netbox.dev", "网络与数据中心资源管理", "🗄️", "#0f766e", []string{"CMDB"}},
						{"Kubernetes", "https://kubernetes.io", "容器编排平台", "☸️", "#326ce5", []string{"容器"}},
						{"Proxmox", "https://www.proxmox.com", "虚拟化管理平台", "🧱", "#e57000", []string{"虚拟化"}},
					},
				},
				{
					name: "发布平台", icon: "🚀", desc: "持续集成与交付",
					cards: []seedCard{
						{"Argo CD", "https://argo-cd.readthedocs.io", "GitOps 持续交付", "🧭", "#ef4444", []string{"GitOps"}},
						{"Jenkins", "https://www.jenkins.io", "自动化构建流水线", "🎩", "#d33833", []string{"CI"}},
					},
				},
			},
		},
		{
			name: "产品与设计", icon: "🎨", desc: "体验、视觉与洞察",
			children: []seedCategory{
				{
					name: "设计资源", icon: "🖌️", desc: "原型、视觉与素材",
					cards: []seedCard{
						{"Figma", "https://www.figma.com", "协同界面设计工具", "🎯", "#a855f7", []string{"设计", "协同"}},
						{"蓝湖", "https://lanhuapp.com", "产品设计协作平台", "🌊", "#3b82f6", []string{"设计"}},
						{"Iconify", "https://icon-sets.iconify.design", "开源图标集合", "✨", "#22c55e", []string{"图标"}},
					},
				},
				{
					name: "数据分析", icon: "📊", desc: "指标与业务洞察",
					cards: []seedCard{
						{"Metabase", "https://www.metabase.com", "人人都能用的 BI 工具", "🧪", "#509ee3", []string{"BI"}},
						{"Superset", "https://superset.apache.org", "现代化数据探索平台", "🚁", "#20a7c9", []string{"BI", "开源"}},
					},
				},
			},
		},
		{
			name: "团队协作", icon: "🤝", desc: "知识、沟通与流程",
			children: []seedCategory{
				{
					name: "知识库", icon: "📚", desc: "文档沉淀与分享",
					cards: []seedCard{
						{"语雀", "https://www.yuque.com", "优雅的团队知识库", "🪶", "#16a34a", []string{"文档"}},
						{"Notion", "https://www.notion.so", "文档、项目与 Wiki", "📝", "#111827", []string{"文档", "协作"}},
						{"Outline", "https://www.getoutline.com", "团队知识库", "🧠", "#6366f1", []string{"Wiki"}},
					},
				},
				{
					name: "沟通协作", icon: "💬", desc: "即时沟通与项目管理",
					cards: []seedCard{
						{"飞书", "https://www.feishu.cn", "一站式协作与办公平台", "🐦", "#3370ff", []string{"IM", "办公"}},
						{"Jira", "https://www.atlassian.com/software/jira", "敏捷项目管理", "🧭", "#0052cc", []string{"项目"}},
						{"Slack", "https://slack.com", "团队沟通中心", "💬", "#611f69", []string{"IM"}},
					},
				},
			},
		},
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertCategory := func(name, icon, desc string, parentID int64, sort int) (int64, error) {
		return s.insertID(ctx, tx,
			`INSERT INTO categories (name, icon, description, parent_id, sort, visible, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, name, icon, desc, parentID, sort, now, now)
	}
	insertCard := func(categoryID int64, c seedCard, sort int) error {
		tags, _ := json.Marshal(c.tags)
		_, err := tx.ExecContext(ctx,
			`INSERT INTO cards (category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'emoji', ?, ?, ?, 'approved', 0, ?, ?)`,
			categoryID, c.name, c.url, c.desc, c.icon, string(tags), sort, now, now)
		return err
	}

	for i, top := range seed {
		topID, err := insertCategory(top.name, top.icon, top.desc, 0, i)
		if err != nil {
			return err
		}
		for j, child := range top.children {
			childID, err := insertCategory(child.name, child.icon, child.desc, topID, j)
			if err != nil {
				return err
			}
			for k, card := range child.cards {
				if err := insertCard(childID, card, k); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Settings

// Settings returns every setting as a key/value map.
func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// UpdateSettings upserts the given settings in one transaction.
func (s *Store) UpdateSettings(ctx context.Context, values map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range values {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Categories

// Categories lists categories. Hidden ones are included when all is true.
func (s *Store) Categories(ctx context.Context, all bool) ([]model.Category, error) {
	q := `SELECT c.id, c.name, c.icon, c.description, c.parent_id, c.sort, c.visible, c.created_at, c.updated_at,
	             (SELECT COUNT(*) FROM cards k WHERE k.category_id = c.id AND k.status = 'approved')
	      FROM categories c`
	if !all {
		q += ` WHERE c.visible = 1`
	}
	q += ` ORDER BY c.parent_id, c.sort, c.id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Icon, &c.Description, &c.ParentID, &c.Sort, &c.Visible, &c.CreatedAt, &c.UpdatedAt, &c.CardCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Category loads one category.
func (s *Store) Category(ctx context.Context, id int64) (*model.Category, error) {
	var c model.Category
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, icon, description, parent_id, sort, visible, created_at, updated_at FROM categories WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Icon, &c.Description, &c.ParentID, &c.Sort, &c.Visible, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCategory inserts a category and returns it.
func (s *Store) CreateCategory(ctx context.Context, c *model.Category) error {
	now := time.Now().Unix()
	id, err := s.insertID(ctx, s.db,
		`INSERT INTO categories (name, icon, description, parent_id, sort, visible, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Icon, c.Description, c.ParentID, c.Sort, c.Visible, now, now)
	if err != nil {
		return err
	}
	c.ID = id
	c.CreatedAt, c.UpdatedAt = now, now
	return nil
}

// UpdateCategory saves an existing category.
func (s *Store) UpdateCategory(ctx context.Context, c *model.Category) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE categories SET name = ?, icon = ?, description = ?, parent_id = ?, sort = ?, visible = ?, updated_at = ?
		 WHERE id = ?`,
		c.Name, c.Icon, c.Description, c.ParentID, c.Sort, c.Visible, time.Now().Unix(), c.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// DeleteCategory removes a category, its descendants and their cards.
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := []int64{id}
	for i := 0; i < len(ids); i++ {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM categories WHERE parent_id = ?`, ids[i])
		if err != nil {
			return err
		}
		for rows.Next() {
			var child int64
			if err := rows.Scan(&child); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, child)
		}
		rows.Close()
	}
	for _, catID := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE category_id = ?`, catID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, catID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Cards

// CardFilter narrows a card query.
type CardFilter struct {
	CategoryID int64
	Status     string
	Query      string
	Limit      int
	Public     bool
}

// Cards lists cards matching the filter.
func (s *Store) Cards(ctx context.Context, f CardFilter) ([]model.Card, error) {
	var (
		where []string
		args  []any
	)
	if f.CategoryID > 0 {
		where = append(where, `category_id = ?`)
		args = append(args, f.CategoryID)
	}
	if f.Status != "" {
		where = append(where, `status = ?`)
		args = append(args, f.Status)
	}
	if f.Query != "" {
		like := "%" + strings.ToLower(f.Query) + "%"
		where = append(where, `(lower(name) LIKE ? OR lower(description) LIKE ? OR lower(url) LIKE ? OR lower(tags) LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	q := `SELECT id, category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at FROM cards`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}
	q += ` ORDER BY sort, id`
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCard(sc scanner) (*model.Card, error) {
	var (
		c    model.Card
		tags string
	)
	if err := sc.Scan(&c.ID, &c.CategoryID, &c.Name, &c.URL, &c.Description, &c.IconType, &c.IconValue,
		&tags, &c.Sort, &c.Status, &c.Clicks, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Tags = []string{}
	_ = json.Unmarshal([]byte(tags), &c.Tags)
	return &c, nil
}

// Card loads one card.
func (s *Store) Card(ctx context.Context, id int64) (*model.Card, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at
		 FROM cards WHERE id = ?`, id)
	c, err := scanCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// CreateCard inserts a card and returns it.
func (s *Store) CreateCard(ctx context.Context, c *model.Card) error {
	c.Normalize()
	now := time.Now().Unix()
	tags, _ := json.Marshal(c.Tags)
	id, err := s.insertID(ctx, s.db,
		`INSERT INTO cards (category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		c.CategoryID, c.Name, c.URL, c.Description, c.IconType, c.IconValue, string(tags), c.Sort, c.Status, now, now)
	if err != nil {
		return err
	}
	c.ID = id
	c.CreatedAt, c.UpdatedAt = now, now
	return nil
}

// UpdateCard saves an existing card.
func (s *Store) UpdateCard(ctx context.Context, c *model.Card) error {
	c.Normalize()
	tags, _ := json.Marshal(c.Tags)
	res, err := s.db.ExecContext(ctx,
		`UPDATE cards SET category_id = ?, name = ?, url = ?, description = ?, icon_type = ?, icon_value = ?,
		 tags = ?, sort = ?, status = ?, updated_at = ? WHERE id = ?`,
		c.CategoryID, c.Name, c.URL, c.Description, c.IconType, c.IconValue, string(tags), c.Sort, c.Status, time.Now().Unix(), c.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// DeleteCard removes a card.
func (s *Store) DeleteCard(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cards WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// IncrementClick records a card visit.
func (s *Store) IncrementClick(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE cards SET clicks = clicks + 1 WHERE id = ? AND status = 'approved'`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Notices

// Notices lists notices; only active, unexpired ones when activeOnly.
func (s *Store) Notices(ctx context.Context, activeOnly bool) ([]model.Notice, error) {
	q := `SELECT id, title, content, level, link, active, sort, expires_at, created_at, updated_at FROM notices`
	var args []any
	if activeOnly {
		q += ` WHERE active = 1 AND (expires_at = 0 OR expires_at > ?)`
		args = append(args, time.Now().Unix())
	}
	q += ` ORDER BY sort, id DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Notice
	for rows.Next() {
		var n model.Notice
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.Level, &n.Link, &n.Active, &n.Sort, &n.ExpiresAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CreateNotice inserts a notice.
func (s *Store) CreateNotice(ctx context.Context, n *model.Notice) error {
	now := time.Now().Unix()
	id, err := s.insertID(ctx, s.db,
		`INSERT INTO notices (title, content, level, link, active, sort, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Title, n.Content, n.Level, n.Link, n.Active, n.Sort, n.ExpiresAt, now, now)
	if err != nil {
		return err
	}
	n.ID = id
	n.CreatedAt, n.UpdatedAt = now, now
	return nil
}

// UpdateNotice saves a notice.
func (s *Store) UpdateNotice(ctx context.Context, n *model.Notice) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE notices SET title = ?, content = ?, level = ?, link = ?, active = ?, sort = ?, expires_at = ?, updated_at = ? WHERE id = ?`,
		n.Title, n.Content, n.Level, n.Link, n.Active, n.Sort, n.ExpiresAt, time.Now().Unix(), n.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// DeleteNotice removes a notice.
func (s *Store) DeleteNotice(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// ---------------------------------------------------------------------------
// Users and sessions

// UserByUsername loads a user together with the password hash.
func (s *Store) UserByUsername(ctx context.Context, username string) (*model.User, string, error) {
	var (
		u    model.User
		hash string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, display_name, role, source, external_id, created_at FROM users WHERE lower(username) = lower(?)`, username).
		Scan(&u.ID, &u.Username, &hash, &u.DisplayName, &u.Role, &u.Source, &u.ExternalID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &u, hash, err
}

// UserByExternal loads a user previously provisioned by an external provider.
func (s *Store) UserByExternal(ctx context.Context, source, externalID string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, role, source, external_id, created_at FROM users
		 WHERE source = ? AND external_id = ?`, source, externalID).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Source, &u.ExternalID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// UserByID loads a user.
func (s *Store) UserByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, role, source, external_id, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Source, &u.ExternalID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// Users lists all accounts.
func (s *Store) Users(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, display_name, role, source, external_id, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Source, &u.ExternalID, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AdminCount returns how many administrator accounts exist.
func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count, err
}

// CreateUser inserts a local account with a bcrypt password hash.
func (s *Store) CreateUser(ctx context.Context, username, password, displayName, role string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	id, err := s.insertID(ctx, s.db,
		`INSERT INTO users (username, password_hash, display_name, role, source, external_id, created_at)
		 VALUES (?, ?, ?, ?, 'local', '', ?)`,
		username, string(hash), displayName, role, now)
	if err != nil {
		return nil, err
	}
	return &model.User{ID: id, Username: username, DisplayName: displayName, Role: role, Source: model.SourceLocal, CreatedAt: now}, nil
}

// CreateExternalUser provisions an account for an LDAP or OIDC identity. The
// password hash is random and unusable: these accounts always authenticate
// through their provider.
func (s *Store) CreateExternalUser(ctx context.Context, username, displayName, role, source, externalID string) (*model.User, error) {
	secret, err := randomSecret()
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	id, err := s.insertID(ctx, s.db,
		`INSERT INTO users (username, password_hash, display_name, role, source, external_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		username, string(hash), displayName, role, source, externalID, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return &model.User{ID: id, Username: username, DisplayName: displayName, Role: role, Source: source, ExternalID: externalID, CreatedAt: now}, nil
}

// UpdateUser saves role and display name.
func (s *Store) UpdateUser(ctx context.Context, u *model.User) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, role = ? WHERE id = ?`, u.DisplayName, u.Role, u.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// UpdateExternalUser refreshes the profile and role of a provider managed
// account. The username is intentionally left alone so links and references
// stay stable.
func (s *Store) UpdateExternalUser(ctx context.Context, id int64, displayName, role, source, externalID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET display_name = ?, role = ?, source = ?, external_id = ? WHERE id = ?`,
		displayName, role, source, externalID, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique violation")
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// SetPassword replaces the bcrypt hash for a user.
func (s *Store) SetPassword(ctx context.Context, id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// DeleteUser removes an account.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// CreateSession stores a session token hash for a user.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID, expiresAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, expiresAt, time.Now().Unix())
	return err
}

// SessionUser resolves a session token hash to a user.
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.display_name, u.role, u.source, u.external_id, u.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token = ? AND s.expires_at > ?`, tokenHash, time.Now().Unix()).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Source, &u.ExternalID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// DeleteSession removes one session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, tokenHash)
	return err
}

// PurgeSessions removes expired sessions.
func (s *Store) PurgeSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}

// ---------------------------------------------------------------------------
// OIDC authorization state

// AuthState is the short lived state kept between the OIDC redirect and the
// callback. It is deliberately stored server side: the browser only sees the
// opaque state value.
type AuthState struct {
	State     string
	Nonce     string
	Verifier  string
	Redirect  string
	ExpiresAt int64
}

// SaveAuthState stores a pending OIDC authorization.
func (s *Store) SaveAuthState(ctx context.Context, st *AuthState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO auth_states (state, nonce, verifier, redirect, expires_at) VALUES (?, ?, ?, ?, ?)`,
		st.State, st.Nonce, st.Verifier, st.Redirect, st.ExpiresAt)
	return err
}

// TakeAuthState consumes an authorization state exactly once.
func (s *Store) TakeAuthState(ctx context.Context, state string) (*AuthState, error) {
	st := &AuthState{State: state}
	err := s.db.QueryRowContext(ctx,
		`SELECT nonce, verifier, redirect, expires_at FROM auth_states WHERE state = ?`, state).
		Scan(&st.Nonce, &st.Verifier, &st.Redirect, &st.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM auth_states WHERE state = ?`, state); err != nil {
		return nil, err
	}
	if st.ExpiresAt < time.Now().Unix() {
		return nil, ErrNotFound
	}
	return st, nil
}

// PurgeAuthStates removes abandoned OIDC authorizations.
func (s *Store) PurgeAuthStates(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_states WHERE expires_at < ?`, time.Now().Unix())
	return err
}

// ---------------------------------------------------------------------------
// Images

// SaveImage stores bytes, de-duplicating by content hash.
func (s *Store) SaveImage(ctx context.Context, name, mime, id string, data []byte) (*model.Image, bool, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var existing model.Image
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, mime, size, sha256, created_at FROM images WHERE sha256 = ?`, hash).
		Scan(&existing.ID, &existing.Name, &existing.Mime, &existing.Size, &existing.SHA256, &existing.CreatedAt)
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO images (id, name, mime, size, sha256, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, mime, len(data), hash, data, now)
	if err != nil {
		return nil, false, err
	}
	return &model.Image{ID: id, Name: name, Mime: mime, Size: int64(len(data)), SHA256: hash, CreatedAt: now}, true, nil
}

// ImageMeta loads image metadata without the blob.
func (s *Store) ImageMeta(ctx context.Context, id string) (*model.Image, error) {
	var img model.Image
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, mime, size, sha256, created_at FROM images WHERE id = ?`, id).
		Scan(&img.ID, &img.Name, &img.Mime, &img.Size, &img.SHA256, &img.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &img, err
}

// ImageBytes loads the blob for an image.
func (s *Store) ImageBytes(ctx context.Context, id string) (*model.Image, []byte, error) {
	var img model.Image
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, mime, size, sha256, created_at, data FROM images WHERE id = ?`, id).
		Scan(&img.ID, &img.Name, &img.Mime, &img.Size, &img.SHA256, &img.CreatedAt, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &img, data, nil
}

// Images lists uploaded assets.
func (s *Store) Images(ctx context.Context) ([]model.Image, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, mime, size, sha256, created_at FROM images ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Image{}
	for rows.Next() {
		var img model.Image
		if err := rows.Scan(&img.ID, &img.Name, &img.Mime, &img.Size, &img.SHA256, &img.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

// DeleteImage removes an image.
func (s *Store) DeleteImage(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// ---------------------------------------------------------------------------
// Statistics and maintenance

// RecordView increments today's page view counter.
func (s *Store) RecordView(ctx context.Context, day string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO daily_stats (day, views) VALUES (?, 1)
		 ON CONFLICT(day) DO UPDATE SET views = views + 1`, day)
	return err
}

// Stats builds the admin dashboard payload.
func (s *Store) Stats(ctx context.Context) (*model.Stats, error) {
	st := &model.Stats{}
	queries := []struct {
		q    string
		dest *int
	}{
		{`SELECT COUNT(*) FROM categories`, &st.Categories},
		{`SELECT COUNT(*) FROM cards`, &st.Cards},
		{`SELECT COUNT(*) FROM cards WHERE status = 'pending'`, &st.Pending},
		{`SELECT COUNT(*) FROM users`, &st.Users},
		{`SELECT COUNT(*) FROM images`, &st.Images},
	}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.q).Scan(item.dest); err != nil {
			return nil, err
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT CAST(COALESCE(SUM(clicks), 0) AS BIGINT) FROM cards`).Scan(&st.TotalClicks); err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	_ = s.db.QueryRowContext(ctx, `SELECT views FROM daily_stats WHERE day = ?`, today).Scan(&st.ViewsToday)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at
		 FROM cards WHERE status = 'approved' ORDER BY clicks DESC, id LIMIT 6`)
	if err != nil {
		return nil, err
	}
	st.TopCards = []model.Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		st.TopCards = append(st.TopCards, *c)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx,
		`SELECT id, category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at
		 FROM cards ORDER BY created_at DESC, id DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	st.RecentCards = []model.Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		st.RecentCards = append(st.RecentCards, *c)
	}
	rows.Close()

	st.DailyViews = make([]model.Daily, 0, 7)
	for i := 6; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var views int64
		_ = s.db.QueryRowContext(ctx, `SELECT views FROM daily_stats WHERE day = ?`, day).Scan(&views)
		st.DailyViews = append(st.DailyViews, model.Daily{Day: day, Views: views})
	}
	if s.postgres {
		var size int64
		_ = s.db.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&size)
		st.DatabaseSize = size
	} else if info, err := os.Stat(s.path); err == nil {
		st.DatabaseSize = info.Size()
	}
	return st, nil
}

// Backup writes a consistent snapshot of the database next to it and returns
// the snapshot path. PostgreSQL does not expose a portable SQL snapshot through
// the driver, so operators should use pg_dump there.
func (s *Store) Backup(ctx context.Context) (string, error) {
	if s.postgres {
		return "", ErrBackupUnsupported
	}
	dir := filepath.Join(filepath.Dir(s.path), "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("team-nav-%s.db", time.Now().Format("20060102-150405"))
	dest := filepath.Join(dir, name)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// ReorderCards assigns sequential sort values to cards in one category.
func (s *Store) ReorderCards(ctx context.Context, categoryID int64, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE cards SET sort = ?, updated_at = ? WHERE id = ? AND category_id = ?`,
			i, time.Now().Unix(), id, categoryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReorderCategories assigns sequential sort values to sibling categories.
func (s *Store) ReorderCategories(ctx context.Context, parentID int64, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE categories SET sort = ?, updated_at = ? WHERE id = ? AND parent_id = ?`,
			i, time.Now().Unix(), id, parentID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func expectOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
