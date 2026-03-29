package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// WatchedRepoStore defines the data-access contract needed by WebhookService.
type WatchedRepoStore interface {
	GetByOwnerRepo(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error)
}

// WebhookService handles HMAC signature validation and GitHub event parsing.
type WebhookService struct {
	watchedRepos WatchedRepoStore
	encSvc       *EncryptionService
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(store WatchedRepoStore, encSvc *EncryptionService) *WebhookService {
	return &WebhookService{watchedRepos: store, encSvc: encSvc}
}

// GetWatchedRepos returns all active watched repos for the given owner and repo name.
func (s *WebhookService) GetWatchedRepos(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error) {
	repos, err := s.watchedRepos.GetByOwnerRepo(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get watched repos: %w", err)
	}
	return repos, nil
}

// DecryptWebhookSecret decrypts an encrypted webhook secret stored in the DB.
func (s *WebhookService) DecryptWebhookSecret(encrypted string) (string, error) {
	return s.encSvc.Decrypt(encrypted)
}

// ValidateHMACSignature returns true if sigHeader is a valid HMAC-SHA256 signature
// of payload using secret. Uses constant-time comparison to prevent timing attacks.
// sigHeader must be in the form "sha256=<hex>".
func ValidateHMACSignature(secret, payload []byte, sigHeader string) bool {
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

// ExtractOwnerRepo parses the repository full_name from a GitHub webhook payload
// and returns the owner and repo name separately.
func ExtractOwnerRepo(payload []byte) (owner, repo string, err error) {
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

// ParseGitHubEvent converts a raw GitHub webhook payload into a partial NotificationCreate.
// The returned struct has ProviderID, EventTypeID, Title, Body, and URL populated;
// the caller must set ID, UserID, and GitHubDeliveryID before persisting.
// Returns nil (no error) for unhandled event types or unhandled action values —
// the caller should skip notification creation but still return 200 OK to GitHub.
func ParseGitHubEvent(eventType string, payload []byte) (*model.NotificationCreate, error) {
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

// --- private event parsers ---

type gitHubPRPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
	} `json:"pull_request"`
}

type gitHubIssueCommentPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int              `json:"number"`
		PullRequest *json.RawMessage `json:"pull_request"` // non-nil means comment is on a PR
	} `json:"issue"`
	Comment struct {
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
}

type gitHubPRReviewCommentPayload struct {
	Action  string `json:"action"`
	Comment struct {
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	} `json:"pull_request"`
}

func parsePullRequestEvent(payload []byte) (*model.NotificationCreate, error) {
	var p gitHubPRPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse pull_request payload: %w", err)
	}

	var eventTypeID, title string
	switch p.Action {
	case "opened":
		eventTypeID = "pr_opened"
		title = fmt.Sprintf("PR #%d opened: %s", p.PullRequest.Number, p.PullRequest.Title)
	case "closed":
		if p.PullRequest.Merged {
			eventTypeID = "pr_merged"
			title = fmt.Sprintf("PR #%d merged: %s", p.PullRequest.Number, p.PullRequest.Title)
		} else {
			eventTypeID = "pr_closed"
			title = fmt.Sprintf("PR #%d closed: %s", p.PullRequest.Number, p.PullRequest.Title)
		}
	default:
		return nil, nil // unhandled action
	}

	url := p.PullRequest.HTMLURL
	return &model.NotificationCreate{
		ProviderID:  "github",
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
		ProviderID:  "github",
		EventTypeID: "pr_comment",
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
		ProviderID:  "github",
		EventTypeID: "pr_review_comment",
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
