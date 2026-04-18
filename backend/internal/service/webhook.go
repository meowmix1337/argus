package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcrypto "github.com/meowmix1337/argus/backend/internal/platform/crypto"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// GitHub notification event type IDs — must match rows in notification_event_types table.
const (
	eventTypePROpened        = "pr_opened"
	eventTypePRMerged        = "pr_merged"
	eventTypePRClosed        = "pr_closed"
	eventTypePRComment       = "pr_comment"
	eventTypePRReviewComment = "pr_review_comment"
)

// WebhookService orchestrates HMAC authentication, event parsing, and notification
// creation for incoming GitHub webhook deliveries.
type WebhookService struct {
	watchedRepos  WatchedRepoStore
	notifications NotificationStore // reuses the interface defined in notification.go
	encSvc        *platformcrypto.EncryptionService
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(
	watchedRepos WatchedRepoStore,
	notifications NotificationStore,
	encSvc *platformcrypto.EncryptionService,
) *WebhookService {
	return &WebhookService{
		watchedRepos:  watchedRepos,
		notifications: notifications,
		encSvc:        encSvc,
	}
}

// ProcessDelivery validates an incoming GitHub webhook delivery end-to-end:
// extracts the repository, authenticates the HMAC signature, parses the event,
// and persists a notification for the matched user.
//
// Possible sentinel errors (map to HTTP status in the handler):
//   - ErrInvalidWebhookPayload → 400
//   - ErrUnauthorized          → 401
//   - ErrDuplicateDelivery     → 200 (idempotent re-delivery)
//   - ErrUnhandledEvent        → 200 (no notification needed)
func (s *WebhookService) ProcessDelivery(
	ctx context.Context,
	eventType string,
	payload []byte,
	sigHeader string,
	deliveryID string,
) error {
	owner, repo, err := extractOwnerRepo(payload)
	if err != nil {
		return apperrors.ErrInvalidWebhookPayload
	}

	matched, err := s.authenticateDelivery(ctx, owner, repo, payload, sigHeader)
	if err != nil {
		return err // ErrUnauthorized or wrapped DB error
	}

	parsed, err := parseGitHubEvent(eventType, payload)
	if err != nil {
		return apperrors.ErrInvalidWebhookPayload
	}
	if parsed == nil {
		return apperrors.ErrUnhandledEvent
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate notification ID: %w", err)
	}
	parsed.ID = id.String()
	parsed.UserID = matched.UserID
	if deliveryID != "" {
		parsed.GitHubDeliveryID = &deliveryID
	}

	if _, err := s.notifications.Create(ctx, *parsed); err != nil {
		return err // includes ErrDuplicateDelivery
	}
	return nil
}

// SimulateDelivery processes a delivery without HMAC validation, creating
// notifications for all users watching the target repository. Intended for
// local development testing only (registered behind APP_ENV=development).
//
// Returns the count of notifications created, or ErrInvalidWebhookPayload /
// ErrWatchedRepoNotFound / ErrUnhandledEvent on failure.
func (s *WebhookService) SimulateDelivery(
	ctx context.Context,
	eventType string,
	payload []byte,
	deliveryID string,
) (int, error) {
	owner, repo, err := extractOwnerRepo(payload)
	if err != nil {
		return 0, apperrors.ErrInvalidWebhookPayload
	}

	watchedRepos, err := s.watchedRepos.GetByOwnerRepo(ctx, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("get watched repos: %w", err)
	}
	if len(watchedRepos) == 0 {
		return 0, apperrors.ErrWatchedRepoNotFound
	}

	parsed, err := parseGitHubEvent(eventType, payload)
	if err != nil {
		return 0, apperrors.ErrInvalidWebhookPayload
	}
	if parsed == nil {
		return 0, apperrors.ErrUnhandledEvent
	}

	if deliveryID == "" {
		generated, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("generate delivery ID: %w", err)
		}
		deliveryID = generated.String()
	}

	created := 0
	for _, wr := range watchedRepos {
		id, err := uuid.NewV7()
		if err != nil {
			slog.Error("simulate: failed to generate notification ID", "error", err, "user_id", wr.UserID)
			continue
		}
		n := *parsed
		n.ID = id.String()
		n.UserID = wr.UserID
		n.GitHubDeliveryID = &deliveryID

		if _, err := s.notifications.Create(ctx, n); err != nil {
			if errors.Is(err, apperrors.ErrDuplicateDelivery) {
				continue
			}
			slog.Error("simulate: failed to create notification", "error", err, "user_id", wr.UserID)
			continue
		}
		created++
	}
	return created, nil
}

// authenticateDelivery finds the WatchedRepo whose decrypted webhook secret
// validates the HMAC signature. Returns ErrUnauthorized if no match is found.
func (s *WebhookService) authenticateDelivery(
	ctx context.Context,
	owner, repo string,
	payload []byte,
	sigHeader string,
) (*model.WatchedRepo, error) {
	repos, err := s.watchedRepos.GetByOwnerRepo(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get watched repos: %w", err)
	}
	for i := range repos {
		secret, err := s.encSvc.Decrypt(repos[i].WebhookSecret)
		if err != nil {
			slog.Warn("webhook: failed to decrypt webhook secret", "watched_repo_id", repos[i].ID, "error", err)
			continue
		}
		if validateHMACSignature([]byte(secret), payload, sigHeader) {
			return &repos[i], nil
		}
	}
	return nil, apperrors.ErrUnauthorized
}

// --- HMAC and payload helpers (unexported) ---

// validateHMACSignature returns true if sigHeader is a valid HMAC-SHA256 signature
// of payload using secret. Uses constant-time comparison to prevent timing attacks.
// sigHeader must be in the form "sha256=<hex>".
func validateHMACSignature(secret, payload []byte, sigHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, sig)
}

// extractOwnerRepo parses the repository full_name from a GitHub webhook payload
// and returns the owner and repo name separately.
func extractOwnerRepo(payload []byte) (owner, repo string, err error) {
	var base struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(payload, &base); err != nil {
		return "", "", fmt.Errorf("parse webhook payload: %w", err)
	}
	parts := strings.SplitN(base.Repository.FullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository full_name: %q", base.Repository.FullName)
	}
	return parts[0], parts[1], nil
}

// parseGitHubEvent converts a raw GitHub webhook payload into a partial
// NotificationCreate. The caller must set ID, UserID, and GitHubDeliveryID.
// Returns nil (no error) for unhandled event types or actions.
func parseGitHubEvent(eventType string, payload []byte) (*model.NotificationCreate, error) {
	switch eventType {
	case "pull_request":
		return parsePullRequestEvent(payload)
	case "issue_comment":
		return parseIssueCommentEvent(payload)
	case "pull_request_review_comment":
		return parsePRReviewCommentEvent(payload)
	default:
		return nil, nil
	}
}

// --- Named types for GitHub webhook JSON shapes ---

type gitHubUser struct {
	Login string `json:"login"`
}

type gitHubComment struct {
	HTMLURL string     `json:"html_url"`
	Body    string     `json:"body"`
	User    gitHubUser `json:"user"`
}

type gitHubPR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	Merged  bool   `json:"merged"`
}

type gitHubPRPayload struct {
	Action      string   `json:"action"`
	PullRequest gitHubPR `json:"pull_request"`
}

type gitHubIssueCommentPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int              `json:"number"`
		PullRequest *json.RawMessage `json:"pull_request"` // non-nil means comment is on a PR
	} `json:"issue"`
	Comment gitHubComment `json:"comment"`
}

type gitHubPRReviewCommentPayload struct {
	Action      string        `json:"action"`
	Comment     gitHubComment `json:"comment"`
	PullRequest gitHubPR      `json:"pull_request"`
}

// --- Event parsers ---

func parsePullRequestEvent(payload []byte) (*model.NotificationCreate, error) {
	var p gitHubPRPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse pull_request payload: %w", err)
	}

	var eventTypeID, title string
	switch p.Action {
	case "opened":
		eventTypeID = eventTypePROpened
		title = fmt.Sprintf("PR #%d opened: %s", p.PullRequest.Number, p.PullRequest.Title)
	case "closed":
		if p.PullRequest.Merged {
			eventTypeID = eventTypePRMerged
			title = fmt.Sprintf("PR #%d merged: %s", p.PullRequest.Number, p.PullRequest.Title)
		} else {
			eventTypeID = eventTypePRClosed
			title = fmt.Sprintf("PR #%d closed: %s", p.PullRequest.Number, p.PullRequest.Title)
		}
	default:
		return nil, nil // unhandled action
	}

	url := p.PullRequest.HTMLURL
	return &model.NotificationCreate{
		ProviderID:  providerGitHub,
		EventTypeID: eventTypeID,
		Title:       title,
		URL:         &url,
	}, nil
}

func parseIssueCommentEvent(payload []byte) (*model.NotificationCreate, error) {
	var p gitHubIssueCommentPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse issue_comment payload: %w", err)
	}

	// Only handle comments on PRs, not plain issues.
	if p.Action != "created" || p.Issue.PullRequest == nil {
		return nil, nil
	}

	title := fmt.Sprintf("Comment on PR #%d by %s", p.Issue.Number, p.Comment.User.Login)
	url := p.Comment.HTMLURL
	body := truncateText(p.Comment.Body, 200)
	return &model.NotificationCreate{
		ProviderID:  providerGitHub,
		EventTypeID: eventTypePRComment,
		Title:       title,
		Body:        &body,
		URL:         &url,
	}, nil
}

func parsePRReviewCommentEvent(payload []byte) (*model.NotificationCreate, error) {
	var p gitHubPRReviewCommentPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse pull_request_review_comment payload: %w", err)
	}

	if p.Action != "created" {
		return nil, nil
	}

	title := fmt.Sprintf("Review comment on PR #%d by %s", p.PullRequest.Number, p.Comment.User.Login)
	url := p.Comment.HTMLURL
	body := truncateText(p.Comment.Body, 200)
	return &model.NotificationCreate{
		ProviderID:  providerGitHub,
		EventTypeID: eventTypePRReviewComment,
		Title:       title,
		Body:        &body,
		URL:         &url,
	}, nil
}

// truncateText returns s truncated to maxLen runes with "…" appended if truncated.
func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
