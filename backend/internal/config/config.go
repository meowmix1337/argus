package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port            string
	GNewsAPIKey     string
	FinnhubAPIKey   string
	APINinjasAPIKey string
	Latitude        float64
	Longitude       float64
	Timezone        *time.Location
	SQLitePath      string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string // e.g. http://localhost:8080/api/auth/callback — required
	SessionSecret      string // raw hex value of SESSION_SECRET env var
	SessionKey         []byte // decoded bytes of SessionSecret; populated by main after validation
	FrontendURL        string // post-login redirect target — required
	SecureCookies      bool   // set Secure flag on cookies; default true, disable only for local HTTP dev
	CORSOrigin         string // allowed CORS origin; defaults to http://localhost:5173
	EncryptionKey      string // hex-encoded AES-256 key for encrypting sensitive fields (optional)

	GitHubClientID     string // GitHub OAuth App client ID
	GitHubClientSecret string // GitHub OAuth App client secret
	GitHubCallbackURL  string // e.g. http://localhost:8080/api/auth/github/callback
	GitHubWebhookURL   string // public URL GitHub posts webhook deliveries to (e.g. https://argus.example.com/api/webhooks/github)
	AppEnv             string // "development" enables debug endpoints; omit or "production" for prod
}

func Load() *Config {
	loadDotEnv()
	lat := parseFloat(os.Getenv("LATITUDE"), 37.7749)
	lon := parseFloat(os.Getenv("LONGITUDE"), -122.4194)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	tz := time.Local
	if tzName := os.Getenv("TIMEZONE"); tzName != "" {
		if loc, err := time.LoadLocation(tzName); err == nil {
			tz = loc
		}
	}

	return &Config{
		Port:            port,
		GNewsAPIKey:     os.Getenv("GNEWS_API_KEY"),
		FinnhubAPIKey:   os.Getenv("FINNHUB_API_KEY"),
		APINinjasAPIKey: os.Getenv("API_NINJAS_API_KEY"),
		Latitude:        lat,
		Longitude:       lon,
		Timezone:        tz,
		SQLitePath:      sqlitePath(os.Getenv("SQLITE_PATH")),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleCallbackURL:  os.Getenv("GOOGLE_CALLBACK_URL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		FrontendURL:        os.Getenv("FRONTEND_URL"),
		SecureCookies:      os.Getenv("SECURE_COOKIES") != "false",
		CORSOrigin:         corsOrigin(os.Getenv("CORS_ORIGIN")),
		EncryptionKey:      os.Getenv("ENCRYPTION_KEY"),

		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubCallbackURL:  os.Getenv("GITHUB_CALLBACK_URL"),
		GitHubWebhookURL:   os.Getenv("GITHUB_WEBHOOK_URL"),
		AppEnv:             os.Getenv("APP_ENV"),
	}
}

// loadDotEnv reads a .env file and sets any keys not already present in the
// environment. It checks the current directory first, then the parent (so the
// server can be run from either the repo root or the backend/ subdirectory).
func loadDotEnv() {
	for _, path := range []string{".env", "../.env"} {
		if parseDotEnv(path) {
			return
		}
	}
}

func parseDotEnv(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return true
}

func sqlitePath(s string) string {
	if s == "" {
		return "dashboard.db"
	}
	return s
}

func corsOrigin(s string) string {
	if s == "" {
		return "http://localhost:5173"
	}
	return s
}

func parseFloat(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
