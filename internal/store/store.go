package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"github.com/Songxwn/team-nav-go/internal/model"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'member',
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
`

// Store wraps the SQLite database.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (and creates when needed) the SQLite database and applies the
// schema plus seed data.
func Open(path, adminUser, adminPass string) (*Store, error) {
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
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.seed(adminUser, adminPass); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the database file location.
func (s *Store) Path() string { return s.path }

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
	}
	for k, v := range defaults {
		if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, k, v); err != nil {
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
		res, err := tx.ExecContext(ctx,
			`INSERT INTO categories (name, icon, description, parent_id, sort, visible, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, name, icon, desc, parentID, sort, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
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
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO categories (name, icon, description, parent_id, sort, visible, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Icon, c.Description, c.ParentID, c.Sort, c.Visible, now, now)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	c.CreatedAt, c.UpdatedAt = now, now
	return err
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
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO cards (category_id, name, url, description, icon_type, icon_value, tags, sort, status, clicks, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		c.CategoryID, c.Name, c.URL, c.Description, c.IconType, c.IconValue, string(tags), c.Sort, c.Status, now, now)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	c.CreatedAt, c.UpdatedAt = now, now
	return err
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
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO notices (title, content, level, link, active, sort, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Title, n.Content, n.Level, n.Link, n.Active, n.Sort, n.ExpiresAt, now, now)
	if err != nil {
		return err
	}
	n.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
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
		`SELECT id, username, password_hash, display_name, role, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &hash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &u, hash, err
}

// UserByID loads a user.
func (s *Store) UserByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// Users lists all accounts.
func (s *Store) Users(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, display_name, role, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateUser inserts an account with a bcrypt password hash.
func (s *Store) CreateUser(ctx context.Context, username, password, displayName, role string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, display_name, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		username, string(hash), displayName, role, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.User{ID: id, Username: username, DisplayName: displayName, Role: role, CreatedAt: now}, nil
}

// UpdateUser saves role and display name.
func (s *Store) UpdateUser(ctx context.Context, u *model.User) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, role = ? WHERE id = ?`, u.DisplayName, u.Role, u.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
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
		`SELECT u.id, u.username, u.display_name, u.role, u.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token = ? AND s.expires_at > ?`, tokenHash, time.Now().Unix()).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt)
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
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(clicks), 0) FROM cards`).Scan(&st.TotalClicks); err != nil {
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
	if info, err := os.Stat(s.path); err == nil {
		st.DatabaseSize = info.Size()
	}
	return st, nil
}

// Backup writes a consistent snapshot of the database next to it and returns
// the snapshot path.
func (s *Store) Backup(ctx context.Context) (string, error) {
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
