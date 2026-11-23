// Command teamnav runs the TeamNav team navigation service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Songxwn/team-nav-go/internal/auth"
	"github.com/Songxwn/team-nav-go/internal/server"
	"github.com/Songxwn/team-nav-go/internal/store"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func main() {
	var (
		addr       = flag.String("addr", envOr("TEAMNAV_ADDR", ":8080"), "listen address")
		dataDir    = flag.String("data", envOr("TEAMNAV_DATA_DIR", "./data"), "directory for the SQLite database")
		cacheMB    = flag.Int("image-cache-mb", envInt("TEAMNAV_IMAGE_CACHE_MB", 64), "in-memory image cache budget in MiB (0 disables)")
		devWeb     = flag.String("dev-web", envOr("TEAMNAV_DEV_WEB", ""), "serve web assets from this directory instead of the embedded copy")
		trustProxy = flag.Bool("trust-proxy", envOr("TEAMNAV_TRUST_PROXY", "0") == "1", "trust X-Forwarded-For from a reverse proxy")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	adminUser := envOr("TEAMNAV_ADMIN_USER", "admin")
	adminPass := envOr("TEAMNAV_ADMIN_PASSWORD", "admin123")
	dbPath := envOr("TEAMNAV_DB", "")
	if dbPath == "" {
		dbPath = *dataDir + string(os.PathSeparator) + "team-nav.db"
	}

	st, err := store.Open(dbPath, adminUser, adminPass)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if adminPass == "admin123" {
		logger.Warn("using default admin password, set TEAMNAV_ADMIN_PASSWORD before first start")
	}

	secureCookie := envOr("TEAMNAV_COOKIE_SECURE", "0") == "1"
	authMgr := auth.NewManager(st, secureCookie)

	srv, err := server.New(server.Options{
		Store:        st,
		Auth:         authMgr,
		ImageCacheMB: *cacheMB,
		Version:      version,
		DevWebDir:    *devWeb,
		TrustProxy:   *trustProxy,
		Logger:       logger,
	})
	if err != nil {
		logger.Error("failed to initialise server", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = st.PurgeSessions(ctx)
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("team-nav started", "addr", *addr, "database", dbPath, "version", version,
			"imageCacheMB", *cacheMB)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "err", err)
			stop <- syscall.SIGTERM
		}
	}()

	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown:", err)
	}
	logger.Info("bye")
}
