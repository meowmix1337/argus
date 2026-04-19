package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	userssvc "github.com/meowmix1337/argus/backend/internal/domain/users/service"
	"github.com/meowmix1337/argus/backend/internal/platform/session"
)

// AuthHandler serves the Google OAuth login / callback / logout endpoints.
type AuthHandler struct {
	authSvc       *userssvc.AuthService
	sessionKey    []byte
	successURL    string // where to redirect after a successful login
	secureCookies bool
}

func NewAuthHandler(authSvc *userssvc.AuthService, sessionKey []byte, successURL string, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		authSvc:       authSvc,
		sessionKey:    sessionKey,
		successURL:    successURL,
		secureCookies: secureCookies,
	}
}

// Login redirects the browser to Google's consent screen.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateMaxAge,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.authSvc.AuthCodeURL(state), http.StatusFound)
}

// Callback handles the redirect from Google after the user consents.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// --- CSRF state check (constant-time to prevent timing attacks) ---
	stateCookie, err := r.Cookie("oauth_state")
	stateParam := r.URL.Query().Get("state")
	if err != nil || stateCookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(stateParam)) != 1 {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// Immediately expire the state cookie.
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	googleUser, err := h.authSvc.ExchangeAndGetUser(r.Context(), code)
	if err != nil {
		slog.Error("oauth exchange failed", "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	userID, err := h.authSvc.UpsertUser(r.Context(), googleUser)
	if err != nil {
		slog.Error("user upsert failed", "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Only store the avatar URL if it's a valid https://lh3.googleusercontent.com URL.
	avatarURL := ""
	if googleUser.Picture != "" {
		if u, err := url.Parse(googleUser.Picture); err == nil &&
			u.Scheme == "https" &&
			strings.HasSuffix(u.Hostname(), "googleusercontent.com") {
			avatarURL = googleUser.Picture
		}
	}

	encoded, err := session.Encode(h.sessionKey, session.Data{
		UserID:    userID,
		Email:     googleUser.Email,
		Name:      googleUser.Name,
		AvatarURL: avatarURL,
		ExpiresAt: time.Now().Add(sessionDuration).Unix(),
	})
	if err != nil {
		slog.Error("session encode failed", "error", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.successURL, http.StatusFound)
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// AddRoutes registers the public auth routes (no session required).
func (h *AuthHandler) AddRoutes(r chi.Router) {
	r.Get("/api/auth/login", h.Login)
	r.Get("/api/auth/callback", h.Callback)
	r.Post("/api/auth/logout", h.Logout)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
