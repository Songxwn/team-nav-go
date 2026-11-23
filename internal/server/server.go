// Package server exposes the HTTP API and the embedded web application.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Songxwn/team-nav-go/internal/auth"
	"github.com/Songxwn/team-nav-go/internal/imgcache"
	"github.com/Songxwn/team-nav-go/internal/model"
	"github.com/Songxwn/team-nav-go/internal/store"
	webpkg "github.com/Songxwn/team-nav-go/internal/web"
)

// Options configures the HTTP server.
type Options struct {
	Store        *store.Store
	Auth         *auth.Manager
	ImageCacheMB int
	Version      string
	DevWebDir    string
	TrustProxy   bool
	Logger       *slog.Logger
}

// Server holds shared dependencies for all handlers.
type Server struct {
	store      *store.Store
	auth       *auth.Manager
	cache      *imgcache.Cache
	version    string
	trustProxy bool
	log        *slog.Logger
	limiter    *limiter
	web        fs.FS
	webDisk    string
	index      []byte
	admin      []byte
	handler    http.Handler
}

// New builds the HTTP handler tree.
func New(opts Options) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		store:      opts.Store,
		auth:       opts.Auth,
		cache:      imgcache.New(int64(opts.ImageCacheMB) << 20),
		version:    opts.Version,
		trustProxy: opts.TrustProxy,
		log:        logger,
		limiter:    newLimiter(),
	}
	if opts.DevWebDir != "" {
		s.webDisk = opts.DevWebDir
		s.web = os.DirFS(opts.DevWebDir)
	} else {
		s.web = webpkg.FS()
	}
	index, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		return nil, err
	}
	admin, err := fs.ReadFile(s.web, "admin.html")
	if err != nil {
		return nil, err
	}
	s.index = index
	s.admin = admin
	s.handler = s.routes()
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", s.handleHealth)
	mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("GET /api/categories", s.handleCategories)
	mux.HandleFunc("GET /api/cards", s.handleCards)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/cards/{id}/click", s.handleClick)
	mux.HandleFunc("POST /api/apply", s.handleApply)

	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)

	mux.HandleFunc("GET /api/images/{id}", s.handleImage)

	mux.HandleFunc("GET /api/admin/overview", s.handleAdminOverview)
	mux.HandleFunc("GET /api/admin/categories", s.handleAdminCategories)
	mux.HandleFunc("POST /api/admin/categories", s.handleAdminCreateCategory)
	mux.HandleFunc("PUT /api/admin/categories/{id}", s.handleAdminUpdateCategory)
	mux.HandleFunc("DELETE /api/admin/categories/{id}", s.handleAdminDeleteCategory)

	mux.HandleFunc("GET /api/admin/cards", s.handleAdminCards)
	mux.HandleFunc("POST /api/admin/cards", s.handleAdminCreateCard)
	mux.HandleFunc("PUT /api/admin/cards/{id}", s.handleAdminUpdateCard)
	mux.HandleFunc("DELETE /api/admin/cards/{id}", s.handleAdminDeleteCard)
	mux.HandleFunc("POST /api/admin/cards/{id}/approve", s.handleAdminApproveCard)
	mux.HandleFunc("POST /api/admin/sort", s.handleAdminSort)

	mux.HandleFunc("GET /api/admin/notices", s.handleAdminNotices)
	mux.HandleFunc("POST /api/admin/notices", s.handleAdminCreateNotice)
	mux.HandleFunc("PUT /api/admin/notices/{id}", s.handleAdminUpdateNotice)
	mux.HandleFunc("DELETE /api/admin/notices/{id}", s.handleAdminDeleteNotice)

	mux.HandleFunc("GET /api/admin/users", s.handleAdminUsers)
	mux.HandleFunc("POST /api/admin/users", s.handleAdminCreateUser)
	mux.HandleFunc("PUT /api/admin/users/{id}", s.handleAdminUpdateUser)
	mux.HandleFunc("DELETE /api/admin/users/{id}", s.handleAdminDeleteUser)
	mux.HandleFunc("POST /api/admin/users/{id}/password", s.handleAdminResetPassword)
	mux.HandleFunc("POST /api/admin/password", s.handleAdminOwnPassword)

	mux.HandleFunc("GET /api/admin/settings", s.handleAdminSettings)
	mux.HandleFunc("PUT /api/admin/settings", s.handleAdminUpdateSettings)

	mux.HandleFunc("GET /api/admin/images", s.handleAdminImages)
	mux.HandleFunc("POST /api/admin/images", s.handleAdminUploadImage)
	mux.HandleFunc("DELETE /api/admin/images/{id}", s.handleAdminDeleteImage)

	mux.HandleFunc("GET /api/admin/backup", s.handleAdminBackup)

	mux.HandleFunc("/", s.handleStatic)

	return s.middleware(mux)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

type ctxKey int

const userKey ctxKey = 1

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Resolve the session once per request and expose it to handlers.
		if user := s.auth.User(r.Context(), r); user != nil {
			r = r.WithContext(context.WithValue(r.Context(), userKey, user))
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		if !strings.HasPrefix(r.URL.Path, "/api/images/") {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; img-src 'self' data: blob: https: http:; "+
					"style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; "+
					"connect-src 'self'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'")
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" && !s.sameOrigin(origin, r.Host) {
				writeError(w, http.StatusForbidden, "跨站请求被拒绝")
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") && !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			}
		}

		if strings.HasPrefix(r.URL.Path, "/api/admin/") && !s.currentUser(r).IsAdmin() {
			writeError(w, http.StatusUnauthorized, "请先以管理员身份登录")
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if !strings.HasPrefix(r.URL.Path, "/api/images/") {
			s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
				"ms", time.Since(start).Milliseconds(), "ip", s.clientIP(r))
		}
	})
}

func (s *Server) sameOrigin(origin, host string) bool {
	origin = strings.TrimSuffix(strings.TrimPrefix(origin, "https://"), "/")
	origin = strings.TrimPrefix(origin, "http://")
	return strings.EqualFold(origin, host)
}

func (s *Server) currentUser(r *http.Request) *model.User {
	u, _ := r.Context().Value(userKey).(*model.User)
	return u
}

func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if rip := r.Header.Get("X-Real-IP"); rip != "" {
			return strings.TrimSpace(rip)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleStatic serves assets from the embedded (or dev) filesystem and falls
// back to index.html for the SPA shell and /admin.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clean := path.Clean("/" + r.URL.Path)
	if clean == "/admin" || clean == "/admin/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(s.admin)
		return
	}
	if clean == "/" || clean == "/index.html" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.store.RecordView(ctx, time.Now().Format("2006-01-02"))
		}()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(s.index)
		return
	}
	name := strings.TrimPrefix(clean, "/")
	if st, err := fs.Stat(s.web, name); err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if strings.HasSuffix(name, ".svg") {
		w.Header().Set("Content-Type", "image/svg+xml")
	}
	http.ServeFileFS(w, r, s.web, name)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "服务器开小差了，请稍后再试")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "请求数据格式不正确")
		return false
	}
	return true
}

func newID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(buf)
}

func parseID(r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	var id int64
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, false
		}
		id = id*10 + int64(raw[i]-'0')
	}
	return id, raw != "" && id > 0
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	entries, bytes, hits, misses := s.cache.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": s.version,
		"time":    time.Now().UTC().Format(time.RFC3339),
		"imageCache": map[string]any{
			"entries": entries,
			"bytes":   bytes,
			"hits":    hits,
			"misses":  misses,
		},
	})
}

// ErrUnsupportedUpload is returned for non image uploads.
var ErrUnsupportedUpload = errors.New("unsupported image type")
