package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	githubendpoint "golang.org/x/oauth2/github"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/httpclient"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// IntegrationStore defines the data-access contract for user integrations.
type IntegrationStore interface {
	Create(ctx context.Context, i model.IntegrationCreate) (model.UserIntegration, error)
	GetByUserAndProvider(ctx context.Context, userID, providerID string) (model.UserIntegration, error)
	GetByID(ctx context.Context, id, userID string) (model.UserIntegration, error)
	Delete(ctx context.Context, id, userID string) (int64, error)
}

// WatchedRepoStore defines the data-access contract for watched repositories.
type WatchedRepoStore interface {
	Create(ctx context.Context, w model.WatchedRepoCreate) (model.WatchedRepo, error)
	GetByID(ctx context.Context, id, userID string) (model.WatchedRepo, error)
	ListByIntegration(ctx context.Context, integrationID, userID string) ([]model.WatchedRepo, error)
	GetByOwnerRepo(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error)
	Delete(ctx context.Context, id, userID string) (int64, error)
}

// GitHubIntegrationService manages GitHub OAuth, repo watching, and webhook lifecycle.
type GitHubIntegrationService struct {
	integrations IntegrationStore
	watchedRepos WatchedRepoStore
	encSvc       *EncryptionService
	httpClient   httpclient.HTTPClient
	oauthCfg     *oauth2.Config
	webhookURL   string // public URL GitHub posts webhook deliveries to
}

// NewGitHubIntegrationService creates a new GitHubIntegrationService.
func NewGitHubIntegrationService(
	integrations IntegrationStore,
	watchedRepos WatchedRepoStore,
	encSvc *EncryptionService,
	httpClient httpclient.HTTPClient,
	clientID, clientSecret, callbackURL, webhookURL string,
) *GitHubIntegrationService {
	return &GitHubIntegrationService{
		integrations: integrations,
		watchedRepos: watchedRepos,
		encSvc:       encSvc,
		httpClient:   httpClient,
		oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  callbackURL,
			Scopes:       []string{"read:user", "repo", "admin:repo_hook"},
			Endpoint:     githubendpoint.Endpoint,
		},
		webhookURL: webhookURL,
	}
}

// AuthCodeURL returns the GitHub consent-screen URL with the given CSRF state.
func (s *GitHubIntegrationService) AuthCodeURL(state string) string {
	return s.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// ExchangeCode exchanges the OAuth code for a token, fetches the GitHub user's
// profile, encrypts the token, and creates the integration record. Any existing
// GitHub integration is disconnected first (best-effort webhook cleanup).
func (s *GitHubIntegrationService) ExchangeCode(ctx context.Context, userID, code string) (model.UserIntegration, error) {
	// Best-effort cleanup before creating a fresh integration.
	if err := s.Disconnect(ctx, userID); err != nil && !errors.Is(err, apperrors.ErrIntegrationNotFound) {
		slog.Warn("github: failed to disconnect existing integration during re-link", "error", err, "user_id", userID)
	}

	token, err := s.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return model.UserIntegration{}, fmt.Errorf("github oauth exchange: %w", err)
	}

	var ghUser gitHubAPIUser
	if err := s.httpClient.Get(ctx,
		"https://api.github.com/user",
		&ghUser,
		httpclient.WithHeader("Authorization", "Bearer "+token.AccessToken),
		httpclient.WithHeader("Accept", "application/vnd.github+json"),
	); err != nil {
		return model.UserIntegration{}, fmt.Errorf("fetch github user: %w", err)
	}

	encToken, err := s.encSvc.Encrypt(token.AccessToken)
	if err != nil {
		return model.UserIntegration{}, fmt.Errorf("encrypt access token: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return model.UserIntegration{}, fmt.Errorf("generate integration id: %w", err)
	}

	return s.integrations.Create(ctx, model.IntegrationCreate{
		ID:               id.String(),
		UserID:           userID,
		ProviderID:       "github",
		AccessToken:      encToken,
		ProviderUserID:   strconv.FormatInt(ghUser.ID, 10),
		ProviderUsername: ghUser.Login,
	})
}

// ListUserRepos returns the user's GitHub repos (up to 100, sorted by last push)
// annotated with whether each is already watched by Argus.
func (s *GitHubIntegrationService) ListUserRepos(ctx context.Context, userID string) ([]model.GitHubRepo, error) {
	integration, err := s.integrations.GetByUserAndProvider(ctx, userID, "github")
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}

	token, err := s.encSvc.Decrypt(integration.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}

	var apiRepos []gitHubAPIRepo
	if err := s.httpClient.Get(ctx,
		"https://api.github.com/user/repos",
		&apiRepos,
		httpclient.WithHeader("Authorization", "Bearer "+token),
		httpclient.WithHeader("Accept", "application/vnd.github+json"),
		httpclient.WithQueryParams(map[string]string{"per_page": "100", "sort": "pushed"}),
	); err != nil {
		return nil, fmt.Errorf("list github repos: %w", err)
	}

	watched, err := s.watchedRepos.ListByIntegration(ctx, integration.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("list watched repos: %w", err)
	}

	watchedSet := make(map[string]struct{}, len(watched))
	for _, w := range watched {
		watchedSet[w.Owner+"/"+w.Repo] = struct{}{}
	}

	repos := make([]model.GitHubRepo, 0, len(apiRepos))
	for _, r := range apiRepos {
		_, isWatched := watchedSet[r.FullName]
		repos = append(repos, model.GitHubRepo{
			FullName: r.FullName,
			Private:  r.Private,
			Watched:  isWatched,
		})
	}
	return repos, nil
}

// UpdateWatchedRepos diffs current watched repos against selectedRepos,
// installing webhooks for new entries and removing them for deselected ones.
// selectedRepos is a slice of "owner/repo" full names (may be empty to clear all).
func (s *GitHubIntegrationService) UpdateWatchedRepos(ctx context.Context, userID string, selectedRepos []string) error {
	integration, err := s.integrations.GetByUserAndProvider(ctx, userID, "github")
	if err != nil {
		return fmt.Errorf("update watched repos: %w", err)
	}

	token, err := s.encSvc.Decrypt(integration.AccessToken)
	if err != nil {
		return fmt.Errorf("decrypt token: %w", err)
	}

	current, err := s.watchedRepos.ListByIntegration(ctx, integration.ID, userID)
	if err != nil {
		return fmt.Errorf("list watched repos: %w", err)
	}

	currentMap := make(map[string]model.WatchedRepo, len(current))
	for _, w := range current {
		currentMap[w.Owner+"/"+w.Repo] = w
	}
	selectedSet := make(map[string]struct{}, len(selectedRepos))
	for _, r := range selectedRepos {
		selectedSet[r] = struct{}{}
	}

	// Remove webhooks for repos no longer selected.
	for fullName, w := range currentMap {
		if _, keep := selectedSet[fullName]; keep {
			continue
		}
		if err := s.removeGitHubWebhook(ctx, token, w.Owner, w.Repo, w.WebhookID); err != nil {
			slog.Warn("github: failed to remove webhook", "repo", fullName, "error", err)
		}
		if _, err := s.watchedRepos.Delete(ctx, w.ID, userID); err != nil {
			slog.Warn("github: failed to delete watched repo record", "repo", fullName, "error", err)
		}
	}

	// Install webhooks for newly selected repos.
	for fullName := range selectedSet {
		if _, exists := currentMap[fullName]; exists {
			continue
		}
		parts := strings.SplitN(fullName, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			slog.Warn("github: invalid repo full_name, skipping", "repo", fullName)
			continue
		}
		if err := s.installWatchedRepo(ctx, userID, integration.ID, token, parts[0], parts[1]); err != nil {
			slog.Error("github: failed to install webhook", "repo", fullName, "error", err)
			return fmt.Errorf("install webhook for %s: %w", fullName, err)
		}
	}
	return nil
}

// Disconnect removes all webhooks for the user's watched repos, deletes the
// watched repo records, and soft-deletes the GitHub integration.
func (s *GitHubIntegrationService) Disconnect(ctx context.Context, userID string) error {
	integration, err := s.integrations.GetByUserAndProvider(ctx, userID, "github")
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}

	token, decErr := s.encSvc.Decrypt(integration.AccessToken)
	if decErr != nil {
		slog.Warn("github: failed to decrypt token during disconnect, skipping webhook removal",
			"user_id", userID, "error", decErr)
	}

	watched, err := s.watchedRepos.ListByIntegration(ctx, integration.ID, userID)
	if err != nil {
		return fmt.Errorf("list watched repos for disconnect: %w", err)
	}

	for _, w := range watched {
		if decErr == nil {
			if err := s.removeGitHubWebhook(ctx, token, w.Owner, w.Repo, w.WebhookID); err != nil {
				slog.Warn("github: failed to remove webhook during disconnect", "repo", w.Owner+"/"+w.Repo, "error", err)
			}
		}
		if _, err := s.watchedRepos.Delete(ctx, w.ID, userID); err != nil {
			slog.Warn("github: failed to delete watched repo during disconnect", "repo", w.Owner+"/"+w.Repo, "error", err)
		}
	}

	rows, err := s.integrations.Delete(ctx, integration.ID, userID)
	if err != nil {
		return fmt.Errorf("delete integration: %w", err)
	}
	if rows == 0 {
		return apperrors.ErrIntegrationNotFound
	}
	return nil
}

// installWatchedRepo creates a GitHub webhook and stores the watched repo record.
func (s *GitHubIntegrationService) installWatchedRepo(
	ctx context.Context,
	userID, integrationID, token, owner, repo string,
) error {
	secret, err := randomSecret()
	if err != nil {
		return fmt.Errorf("generate webhook secret: %w", err)
	}

	hookResp, err := s.createGitHubWebhook(ctx, token, owner, repo, secret)
	if err != nil {
		return fmt.Errorf("create github webhook: %w", err)
	}

	encSecret, err := s.encSvc.Encrypt(secret)
	if err != nil {
		return fmt.Errorf("encrypt webhook secret: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate watched repo id: %w", err)
	}

	_, err = s.watchedRepos.Create(ctx, model.WatchedRepoCreate{
		ID:            id.String(),
		UserID:        userID,
		IntegrationID: integrationID,
		Owner:         owner,
		Repo:          repo,
		WebhookID:     strconv.FormatInt(hookResp.ID, 10),
		WebhookSecret: encSecret,
	})
	return err
}

// createGitHubWebhook calls POST /repos/{owner}/{repo}/hooks.
func (s *GitHubIntegrationService) createGitHubWebhook(
	ctx context.Context,
	token, owner, repo, secret string,
) (gitHubWebhookResponse, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks",
		url.PathEscape(owner), url.PathEscape(repo))
	body := gitHubWebhookCreate{
		Name: "web",
		Config: map[string]string{
			"url":          s.webhookURL,
			"content_type": "json",
			"secret":       secret,
			"insecure_ssl": "0",
		},
		Events: []string{"pull_request", "issue_comment", "pull_request_review_comment"},
		Active: true,
	}
	var resp gitHubWebhookResponse
	if err := s.httpClient.Post(ctx, apiURL, body, &resp,
		httpclient.WithHeader("Authorization", "Bearer "+token),
		httpclient.WithHeader("Accept", "application/vnd.github+json"),
	); err != nil {
		return gitHubWebhookResponse{}, err
	}
	return resp, nil
}

// removeGitHubWebhook calls DELETE /repos/{owner}/{repo}/hooks/{hook_id}.
// 404 responses are silently ignored (webhook already removed on GitHub's side).
func (s *GitHubIntegrationService) removeGitHubWebhook(
	ctx context.Context,
	token, owner, repo, webhookID string,
) error {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%s",
		url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(webhookID))
	err := s.httpClient.Delete(ctx, apiURL, nil,
		httpclient.WithHeader("Authorization", "Bearer "+token),
		httpclient.WithHeader("Accept", "application/vnd.github+json"),
	)
	if err != nil {
		if httpErr, ok := err.(*httpclient.HTTPError); ok && httpErr.StatusCode == 404 {
			return nil
		}
		return err
	}
	return nil
}

// randomSecret generates a 32-byte cryptographically random hex string
// for use as a webhook HMAC secret.
func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --- GitHub API types (unexported) ---

type gitHubAPIUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type gitHubAPIRepo struct {
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

type gitHubWebhookCreate struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
	Events []string          `json:"events"`
	Active bool              `json:"active"`
}

type gitHubWebhookResponse struct {
	ID int64 `json:"id"`
}
