package main

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/meowmix1337/argus/backend/internal/database"
	"github.com/meowmix1337/argus/backend/internal/platform/config"
	"github.com/meowmix1337/argus/backend/internal/server"
	"github.com/meowmix1337/argus/backend/internal/service"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Warn if Google OAuth credentials are absent — server starts but login will not work.
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		slog.Warn("GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET not set — OAuth login disabled")
	}

	// Fail fast on missing or weak required configuration.
	missing := false
	for _, check := range []struct {
		val  string
		name string
	}{
		{cfg.GoogleCallbackURL, "GOOGLE_CALLBACK_URL"},
		{cfg.FrontendURL, "FRONTEND_URL"},
	} {
		if check.val == "" {
			slog.Error("missing required env var", "var", check.name)
			missing = true
		}
	}

	// Decode session secret from hex; must be at least 32 bytes (64 hex chars).
	sessionKey, err := hex.DecodeString(cfg.SessionSecret)
	if err != nil || len(sessionKey) < 32 {
		slog.Error("SESSION_SECRET must be a valid hex string of at least 64 chars (openssl rand -hex 32)")
		missing = true
	} else {
		cfg.SessionKey = sessionKey
	}

	// Validate FRONTEND_URL is an absolute http(s) URL to prevent open redirects.
	if cfg.FrontendURL != "" {
		if u, parseErr := url.Parse(cfg.FrontendURL); parseErr != nil ||
			(u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			slog.Error("FRONTEND_URL must be an absolute http(s) URL", "value", cfg.FrontendURL)
			missing = true
		}
	}

	// Reject CORS_ORIGIN=* or "null" — incompatible with Access-Control-Allow-Credentials: true.
	corsOrigin := strings.TrimSpace(cfg.CORSOrigin)
	if corsOrigin == "*" || strings.EqualFold(corsOrigin, "null") {
		slog.Error("CORS_ORIGIN must not be \"*\" or \"null\" (credentials mode requires a specific origin)")
		missing = true
	} else if corsOrigin != "" {
		u, parseErr := url.Parse(corsOrigin)
		if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			slog.Error("CORS_ORIGIN must be an absolute http(s) URL", "value", corsOrigin)
			missing = true
		} else if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			slog.Error("CORS_ORIGIN must be scheme://host[:port] with no path or query", "value", corsOrigin)
			missing = true
		} else {
			// Normalize to scheme://host (no trailing slash) to match browser Origin header.
			cfg.CORSOrigin = u.Scheme + "://" + u.Host
		}
	}

	if missing {
		os.Exit(1)
	}

	// Build EncryptionService from hex key (required — validates + constructs in one step).
	encSvc, encErr := service.ProvideEncryptionService(cfg.EncryptionKey)
	if encErr != nil {
		slog.Error("ENCRYPTION_KEY configuration failed", "error", encErr)
		os.Exit(1)
	}
	cfg.EncryptionKey = "" // clear hex string from memory

	db, err := database.Open(cfg.SQLitePath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	if err := database.Migrate(migrateCtx, db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, db, encSvc)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	srv.Stop()
	slog.Info("server exited")
}
