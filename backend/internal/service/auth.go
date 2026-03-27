package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleUser holds the fields we care about from Google's userinfo endpoint.
type GoogleUser struct {
	Sub     string `json:"sub"` // Google's stable user ID
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// AuthService drives the Google OAuth 2.0 / OIDC flow and persists users.
type AuthService struct {
	db       *sqlx.DB
	oauthCfg *oauth2.Config
}

func NewAuthService(db *sqlx.DB, clientID, clientSecret, redirectURL string) *AuthService {
	return &AuthService{
		db: db,
		oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// AuthCodeURL returns the Google consent-screen URL with the given CSRF state.
func (s *AuthService) AuthCodeURL(state string) string {
	return s.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// ExchangeAndGetUser exchanges the authorisation code for a token, then fetches
// the user's profile from Google's userinfo endpoint.
func (s *AuthService) ExchangeAndGetUser(ctx context.Context, code string) (GoogleUser, error) {
	token, err := s.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("oauth exchange: %w", err)
	}

	client := s.oauthCfg.Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return GoogleUser{}, fmt.Errorf("userinfo fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return GoogleUser{}, fmt.Errorf("userinfo %d: %s", resp.StatusCode, body)
	}

	var gu GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return GoogleUser{}, fmt.Errorf("decode userinfo: %w", err)
	}
	if gu.Sub == "" || gu.Email == "" {
		return GoogleUser{}, errors.New("userinfo missing required fields")
	}
	return gu, nil
}

// UpsertUser finds or creates a user record for the given Google account.
// Uses INSERT ... ON CONFLICT to avoid a TOCTOU race when concurrent logins
// arrive for the same new Google account.
// Returns the application-level UUID for the user.
func (s *AuthService) UpsertUser(ctx context.Context, gu GoogleUser) (string, error) {
	email := strings.ToLower(gu.Email) // users table enforces lower(email)

	newID, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}
	id := newID.String()

	// Single atomic statement: insert on first login, update mutable fields on
	// subsequent logins.  The ON CONFLICT target matches uq_users_google_id_active.
	var resultID string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO users (id, google_id, email, name, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (google_id) WHERE deleted_at IS NULL AND google_id IS NOT NULL
		DO UPDATE SET name = excluded.name, avatar_url = excluded.avatar_url
		RETURNING id`,
		id, gu.Sub, email, gu.Name, gu.Picture,
	).Scan(&resultID)
	if err != nil {
		return "", fmt.Errorf("upsert user: %w", err)
	}
	return resultID, nil
}
