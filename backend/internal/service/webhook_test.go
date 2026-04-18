package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// computeTestHMAC produces a GitHub-style HMAC-SHA256 header value for tests.
func computeTestHMAC(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ---- validateHMACSignature ----

func TestValidateHMACSignature_ValidSignature(t *testing.T) {
	secret := []byte("webhooksecret")
	payload := []byte(`{"action":"opened","repository":{"full_name":"owner/repo"}}`)
	sig := computeTestHMAC(secret, payload)

	if !validateHMACSignature(secret, payload, sig) {
		t.Error("expected valid HMAC to return true")
	}
}

func TestValidateHMACSignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	sig := computeTestHMAC([]byte("right-secret"), payload)

	if validateHMACSignature([]byte("wrong-secret"), payload, sig) {
		t.Error("expected wrong secret to return false")
	}
}

func TestValidateHMACSignature_MissingPrefix(t *testing.T) {
	// GitHub always prefixes with "sha256="; a bare hex string must be rejected.
	if validateHMACSignature([]byte("secret"), []byte("payload"), "deadbeef") {
		t.Error("expected missing 'sha256=' prefix to return false")
	}
}

func TestValidateHMACSignature_InvalidHex(t *testing.T) {
	if validateHMACSignature([]byte("secret"), []byte("payload"), "sha256=not-valid-hex!!") {
		t.Error("expected invalid hex to return false")
	}
}

func TestValidateHMACSignature_TamperedPayload(t *testing.T) {
	secret := []byte("secret")
	sig := computeTestHMAC(secret, []byte("original payload"))

	if validateHMACSignature(secret, []byte("tampered payload"), sig) {
		t.Error("expected tampered payload to fail HMAC verification")
	}
}

// ---- extractOwnerRepo ----

func TestExtractOwnerRepo_Valid(t *testing.T) {
	payload := []byte(`{"repository":{"full_name":"acme-corp/backend"}}`)
	owner, repo, err := extractOwnerRepo(payload)
	if err != nil {
		t.Fatalf("extractOwnerRepo: %v", err)
	}
	if owner != "acme-corp" || repo != "backend" {
		t.Errorf("got owner=%q repo=%q, want acme-corp/backend", owner, repo)
	}
}

func TestExtractOwnerRepo_InvalidJSON(t *testing.T) {
	_, _, err := extractOwnerRepo([]byte("not json {{"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExtractOwnerRepo_EmptyFullName(t *testing.T) {
	payload := []byte(`{"repository":{"full_name":""}}`)
	_, _, err := extractOwnerRepo(payload)
	if err == nil {
		t.Error("expected error for empty full_name")
	}
}

func TestExtractOwnerRepo_NoSlash(t *testing.T) {
	payload := []byte(`{"repository":{"full_name":"justowner"}}`)
	_, _, err := extractOwnerRepo(payload)
	if err == nil {
		t.Error("expected error for full_name without '/'")
	}
}

// ---- truncateText ----

func TestTruncateText_ShortText_Unchanged(t *testing.T) {
	if got := truncateText("hello", 10); got != "hello" {
		t.Errorf("truncateText = %q, want unchanged", got)
	}
}

func TestTruncateText_ExactLength_Unchanged(t *testing.T) {
	if got := truncateText("hello", 5); got != "hello" {
		t.Errorf("truncateText = %q, want unchanged at exact length", got)
	}
}

func TestTruncateText_LongText_TruncatedWithEllipsis(t *testing.T) {
	if got := truncateText("hello world", 5); got != "hello…" {
		t.Errorf("truncateText = %q, want %q", got, "hello…")
	}
}

func TestTruncateText_EmptyString(t *testing.T) {
	if got := truncateText("", 10); got != "" {
		t.Errorf("truncateText = %q, want empty", got)
	}
}

// TestTruncateText_Unicode ensures truncation counts runes, not bytes.
func TestTruncateText_Unicode_CountsRunes(t *testing.T) {
	// "日本語" = 3 runes but 9 bytes; truncate to 2 runes → "日本…"
	got := truncateText("日本語テスト", 2)
	if got != "日本…" {
		t.Errorf("truncateText = %q, want %q", got, "日本…")
	}
}

// ---- parseGitHubEvent ----

func TestParseGitHubEvent_UnknownEventType_ReturnsNil(t *testing.T) {
	result, err := parseGitHubEvent("ping", []byte(`{}`))
	if err != nil || result != nil {
		t.Errorf("expected (nil, nil) for unhandled event type, got result=%v err=%v", result, err)
	}
}

func TestParseGitHubEvent_PullRequest_Routed(t *testing.T) {
	payload, _ := json.Marshal(gitHubPRPayload{
		Action:      "opened",
		PullRequest: gitHubPR{Number: 1, Title: "Init"},
	})
	result, err := parseGitHubEvent("pull_request", payload)
	if err != nil || result == nil {
		t.Errorf("expected PR event to be parsed, got result=%v err=%v", result, err)
	}
}

func TestParseGitHubEvent_IssueComment_Routed(t *testing.T) {
	// A plain issue comment (no PullRequest field) returns nil — routing is confirmed.
	p := gitHubIssueCommentPayload{Action: "deleted"}
	payload, _ := json.Marshal(p)
	result, err := parseGitHubEvent("issue_comment", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result // nil is acceptable for "deleted" action
}

// ---- parsePullRequestEvent ----

func TestParsePullRequestEvent_Opened(t *testing.T) {
	payload, _ := json.Marshal(gitHubPRPayload{
		Action:      "opened",
		PullRequest: gitHubPR{Number: 42, Title: "Add feature X", HTMLURL: "https://github.com/owner/repo/pull/42"},
	})
	n, err := parsePullRequestEvent(payload)
	if err != nil {
		t.Fatalf("parsePullRequestEvent: %v", err)
	}
	if n == nil {
		t.Fatal("expected non-nil notification for 'opened'")
	}
	if n.EventTypeID != eventTypePROpened {
		t.Errorf("EventTypeID = %q, want %q", n.EventTypeID, eventTypePROpened)
	}
	if n.Title != "PR #42 opened: Add feature X" {
		t.Errorf("Title = %q", n.Title)
	}
}

func TestParsePullRequestEvent_ClosedAndMerged(t *testing.T) {
	payload, _ := json.Marshal(gitHubPRPayload{
		Action:      "closed",
		PullRequest: gitHubPR{Number: 10, Title: "Fix bug", Merged: true},
	})
	n, err := parsePullRequestEvent(payload)
	if err != nil || n == nil || n.EventTypeID != eventTypePRMerged {
		t.Errorf("expected pr_merged, got n=%v err=%v", n, err)
	}
	if n != nil && n.Title != "PR #10 merged: Fix bug" {
		t.Errorf("Title = %q", n.Title)
	}
}

func TestParsePullRequestEvent_ClosedNotMerged(t *testing.T) {
	payload, _ := json.Marshal(gitHubPRPayload{
		Action:      "closed",
		PullRequest: gitHubPR{Number: 10, Title: "Fix bug", Merged: false},
	})
	n, err := parsePullRequestEvent(payload)
	if err != nil || n == nil || n.EventTypeID != eventTypePRClosed {
		t.Errorf("expected pr_closed, got n=%v err=%v", n, err)
	}
}

func TestParsePullRequestEvent_UnhandledAction_ReturnsNil(t *testing.T) {
	payload, _ := json.Marshal(gitHubPRPayload{Action: "labeled"})
	n, err := parsePullRequestEvent(payload)
	if err != nil || n != nil {
		t.Errorf("expected (nil, nil) for unhandled action, got n=%v err=%v", n, err)
	}
}

func TestParsePullRequestEvent_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := parsePullRequestEvent([]byte("bad json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---- parseIssueCommentEvent ----

func TestParseIssueCommentEvent_CreatedOnPR(t *testing.T) {
	raw := json.RawMessage(`{"href":"https://api.github.com/repos/owner/repo/pulls/5"}`)
	p := gitHubIssueCommentPayload{
		Action: "created",
		Issue: struct {
			Number      int              `json:"number"`
			PullRequest *json.RawMessage `json:"pull_request"`
		}{Number: 5, PullRequest: &raw},
		Comment: gitHubComment{User: gitHubUser{Login: "alice"}, HTMLURL: "https://github.com/..."},
	}
	payload, _ := json.Marshal(p)
	n, err := parseIssueCommentEvent(payload)
	if err != nil || n == nil || n.EventTypeID != eventTypePRComment {
		t.Errorf("expected pr_comment, got n=%v err=%v", n, err)
	}
	if n != nil && n.Title != "Comment on PR #5 by alice" {
		t.Errorf("Title = %q", n.Title)
	}
}

func TestParseIssueCommentEvent_CreatedOnPlainIssue_ReturnsNil(t *testing.T) {
	// PullRequest field is nil → plain issue, not a PR.
	p := gitHubIssueCommentPayload{
		Action: "created",
		Issue: struct {
			Number      int              `json:"number"`
			PullRequest *json.RawMessage `json:"pull_request"`
		}{Number: 3, PullRequest: nil},
	}
	payload, _ := json.Marshal(p)
	n, err := parseIssueCommentEvent(payload)
	if err != nil || n != nil {
		t.Errorf("expected nil for plain issue comment, got n=%v err=%v", n, err)
	}
}

func TestParseIssueCommentEvent_NonCreatedAction_ReturnsNil(t *testing.T) {
	p := gitHubIssueCommentPayload{Action: "deleted"}
	payload, _ := json.Marshal(p)
	n, err := parseIssueCommentEvent(payload)
	if err != nil || n != nil {
		t.Errorf("expected nil for 'deleted' action, got n=%v err=%v", n, err)
	}
}

// ---- parsePRReviewCommentEvent ----

func TestParsePRReviewCommentEvent_Created(t *testing.T) {
	p := gitHubPRReviewCommentPayload{
		Action:      "created",
		PullRequest: gitHubPR{Number: 7},
		Comment:     gitHubComment{User: gitHubUser{Login: "bob"}, Body: "LGTM"},
	}
	payload, _ := json.Marshal(p)
	n, err := parsePRReviewCommentEvent(payload)
	if err != nil || n == nil || n.EventTypeID != eventTypePRReviewComment {
		t.Errorf("expected pr_review_comment, got n=%v err=%v", n, err)
	}
	if n != nil && n.Title != "Review comment on PR #7 by bob" {
		t.Errorf("Title = %q", n.Title)
	}
}

func TestParsePRReviewCommentEvent_EditedAction_ReturnsNil(t *testing.T) {
	p := gitHubPRReviewCommentPayload{Action: "edited"}
	payload, _ := json.Marshal(p)
	n, err := parsePRReviewCommentEvent(payload)
	if err != nil || n != nil {
		t.Errorf("expected nil for 'edited' action, got n=%v err=%v", n, err)
	}
}

// ---- WebhookService (service methods) ----

// newWebhookTestSvc is a convenience builder for webhook service tests.
func newWebhookTestSvc(t *testing.T, repos []model.WatchedRepo) (*WebhookService, *fakeNotificationStore) {
	t.Helper()
	enc := mustNewEncryptionService(t)
	notifStore := &fakeNotificationStore{}
	return NewWebhookService(&fakeWatchedRepoStore{repos: repos}, notifStore, enc), notifStore
}

// newWebhookTestSvcWithEnc returns the encryption service as well for HMAC setup.
func newWebhookTestSvcWithEnc(t *testing.T, repos []model.WatchedRepo) (*WebhookService, *fakeNotificationStore, *EncryptionService) {
	t.Helper()
	enc := mustNewEncryptionService(t)
	notifStore := &fakeNotificationStore{}
	return NewWebhookService(&fakeWatchedRepoStore{repos: repos}, notifStore, enc), notifStore, enc
}

// encryptedRepo creates a WatchedRepo whose WebhookSecret is the encrypted form of plainSecret.
func encryptedRepo(t *testing.T, enc *EncryptionService, userID, plainSecret string) model.WatchedRepo {
	t.Helper()
	encSecret, err := enc.Encrypt(plainSecret)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	return model.WatchedRepo{ID: "wr1", UserID: userID, WebhookSecret: encSecret}
}

// ---- ProcessDelivery ----

func TestWebhookService_ProcessDelivery_InvalidPayload(t *testing.T) {
	svc, _ := newWebhookTestSvc(t, nil)
	err := svc.ProcessDelivery(context.Background(), "pull_request", []byte("bad json"), "sig", "d1")
	if !errors.Is(err, apperrors.ErrInvalidWebhookPayload) {
		t.Errorf("expected ErrInvalidWebhookPayload, got %v", err)
	}
}

func TestWebhookService_ProcessDelivery_Unauthorized_WrongSignature(t *testing.T) {
	svc, _, enc := newWebhookTestSvcWithEnc(t, nil)
	repo := encryptedRepo(t, enc, "user1", "right-secret")
	svc.watchedRepos = &fakeWatchedRepoStore{repos: []model.WatchedRepo{repo}}

	payload := []byte(`{"action":"opened","pull_request":{"number":1},"repository":{"full_name":"owner/repo"}}`)
	wrongSig := computeTestHMAC([]byte("wrong-secret"), payload)
	err := svc.ProcessDelivery(context.Background(), "pull_request", payload, wrongSig, "d1")
	if !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestWebhookService_ProcessDelivery_UnhandledEvent(t *testing.T) {
	svc, _, enc := newWebhookTestSvcWithEnc(t, nil)
	secret := "webhook-secret"
	repo := encryptedRepo(t, enc, "user1", secret)
	svc.watchedRepos = &fakeWatchedRepoStore{repos: []model.WatchedRepo{repo}}

	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)
	sig := computeTestHMAC([]byte(secret), payload)
	err := svc.ProcessDelivery(context.Background(), "ping", payload, sig, "d1")
	if !errors.Is(err, apperrors.ErrUnhandledEvent) {
		t.Errorf("expected ErrUnhandledEvent, got %v", err)
	}
}

func TestWebhookService_ProcessDelivery_Success_PROpened(t *testing.T) {
	svc, notifStore, enc := newWebhookTestSvcWithEnc(t, nil)
	secret := "webhook-secret"
	repo := encryptedRepo(t, enc, "user1", secret)
	svc.watchedRepos = &fakeWatchedRepoStore{repos: []model.WatchedRepo{repo}}

	payload := []byte(`{"action":"opened","pull_request":{"number":42,"title":"Add feature"},"repository":{"full_name":"owner/repo"}}`)
	sig := computeTestHMAC([]byte(secret), payload)
	if err := svc.ProcessDelivery(context.Background(), "pull_request", payload, sig, "d1"); err != nil {
		t.Fatalf("ProcessDelivery: %v", err)
	}
	if len(notifStore.notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(notifStore.notifications))
	}
}

// ---- SimulateDelivery ----

func TestWebhookService_SimulateDelivery_InvalidPayload(t *testing.T) {
	svc, _ := newWebhookTestSvc(t, nil)
	if _, err := svc.SimulateDelivery(context.Background(), "pull_request", []byte("bad json"), ""); !errors.Is(err, apperrors.ErrInvalidWebhookPayload) {
		t.Errorf("expected ErrInvalidWebhookPayload, got %v", err)
	}
}

func TestWebhookService_SimulateDelivery_NoWatchedRepos(t *testing.T) {
	svc, _ := newWebhookTestSvc(t, nil)
	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)
	if _, err := svc.SimulateDelivery(context.Background(), "pull_request", payload, ""); !errors.Is(err, apperrors.ErrWatchedRepoNotFound) {
		t.Errorf("expected ErrWatchedRepoNotFound, got %v", err)
	}
}

func TestWebhookService_SimulateDelivery_UnhandledEvent(t *testing.T) {
	repos := []model.WatchedRepo{{ID: "wr1", UserID: "u1"}}
	svc, _ := newWebhookTestSvc(t, repos)
	payload := []byte(`{"repository":{"full_name":"owner/repo"}}`)
	if _, err := svc.SimulateDelivery(context.Background(), "ping", payload, ""); !errors.Is(err, apperrors.ErrUnhandledEvent) {
		t.Errorf("expected ErrUnhandledEvent, got %v", err)
	}
}

func TestWebhookService_SimulateDelivery_Success(t *testing.T) {
	repos := []model.WatchedRepo{{ID: "wr1", UserID: "u1"}}
	svc, notifStore := newWebhookTestSvc(t, repos)

	payload := []byte(`{"action":"opened","pull_request":{"number":1,"title":"Test"},"repository":{"full_name":"owner/repo"}}`)
	count, err := svc.SimulateDelivery(context.Background(), "pull_request", payload, "d1")
	if err != nil {
		t.Fatalf("SimulateDelivery: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if len(notifStore.notifications) != 1 {
		t.Errorf("expected 1 notification created, got %d", len(notifStore.notifications))
	}
}
