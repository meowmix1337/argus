package handler

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// GitHubAuthHandler drives the GitHub OAuth link flow (not a login provider).
// Both routes sit behind requireAuth — they link a GitHub token to the existing
// session user.
type GitHubAuthHandler struct {
	githubSvc     *service.GitHubIntegrationService
	successURL    string
	secureCookies bool
}

// NewGitHubAuthHandler creates a new GitHubAuthHandler.
func NewGitHubAuthHandler(
	githubSvc *service.GitHubIntegrationService,
	successURL string,
	secureCookies bool,
) *GitHubAuthHandler {
	return &GitHubAuthHandler{
		githubSvc:     githubSvc,
		successURL:    successURL,
		secureCookies: secureCookies,
	}
}

// Link redirects the authenticated user to GitHub's consent screen.
func (h *GitHubAuthHandler) Link(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gh_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateMaxAge,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.githubSvc.AuthCodeURL(state), http.StatusFound)
}

// Callback handles the redirect from GitHub after the user consents.
func (h *GitHubAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("gh_oauth_state")
	stateParam := r.URL.Query().Get("state")
	if err != nil || stateCookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(stateParam)) != 1 {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gh_oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if _, err := h.githubSvc.ExchangeCode(r.Context(), sess.UserID, code); err != nil {
		slog.Error("github oauth exchange failed", "error", err)
		http.Error(w, "github authentication failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.successURL, http.StatusFound)
}

// AddRoutes registers the GitHub OAuth link routes (both require an active session).
func (h *GitHubAuthHandler) AddRoutes(r chi.Router) {
	r.Get("/api/auth/github", h.Link)
	r.Get("/api/auth/github/callback", h.Callback)
}
