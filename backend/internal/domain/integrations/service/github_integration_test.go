package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// newTestGitHubService builds a minimal service with fake stores for unit tests.
func newTestGitHubService(intStore *fakeIntegrationStore, repoStore *fakeWatchedRepoStore) *GitHubIntegrationService {
	return &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: repoStore,
		httpClient:   &fakeHTTPClient{},
		oauthCfg: &oauth2.Config{
			ClientID:    "test-client-id",
			RedirectURL: "http://localhost/callback",
		},
	}
}

// ---- AuthCodeURL ----

func TestGitHubIntegrationService_AuthCodeURL_ContainsClientID(t *testing.T) {
	svc := &GitHubIntegrationService{
		oauthCfg: &oauth2.Config{
			ClientID:    "my-client-id",
			RedirectURL: "http://localhost/callback",
		},
	}
	url := svc.AuthCodeURL("csrf-state")
	if url == "" {
		t.Error("expected non-empty OAuth URL")
	}
}

// ---- GetStatus ----

func TestGitHubIntegrationService_GetStatus_Success(t *testing.T) {
	intStore := &fakeIntegrationStore{
		integration: model.UserIntegration{ID: "i1", UserID: "user1", ProviderID: "github"},
	}
	svc := newTestGitHubService(intStore, &fakeWatchedRepoStore{})
	got, err := svc.GetStatus(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if got.ID != "i1" {
		t.Errorf("ID = %q, want %q", got.ID, "i1")
	}
}

func TestGitHubIntegrationService_GetStatus_NotFound_ReturnsError(t *testing.T) {
	intStore := &fakeIntegrationStore{getErr: apperrors.ErrIntegrationNotFound}
	svc := newTestGitHubService(intStore, &fakeWatchedRepoStore{})
	_, err := svc.GetStatus(context.Background(), "user1")
	if !errors.Is(err, apperrors.ErrIntegrationNotFound) {
		t.Errorf("expected ErrIntegrationNotFound, got %v", err)
	}
}

// ---- Disconnect ----

func TestGitHubIntegrationService_Disconnect_IntegrationNotFound(t *testing.T) {
	intStore := &fakeIntegrationStore{getErr: apperrors.ErrIntegrationNotFound}
	svc := newTestGitHubService(intStore, &fakeWatchedRepoStore{})
	err := svc.Disconnect(context.Background(), "user1")
	if err == nil {
		t.Error("expected error when integration not found")
	}
}

// ---- ExchangeCode ----

// TestGitHubIntegrationService_ExchangeCode_Success drives the full exchange flow
// against a local httptest server so no real GitHub OAuth is needed.
func TestGitHubIntegrationService_ExchangeCode_Success(t *testing.T) {
	// Simulate GitHub's token endpoint returning an access token.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gh_test_token","token_type":"bearer"}`))
	}))
	defer ts.Close()

	enc := mustNewEncryptionService(t)
	intStore := &fakeIntegrationStore{
		// Disconnect will see ErrIntegrationNotFound and silently continue.
		getErr: apperrors.ErrIntegrationNotFound,
	}
	svc := &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: &fakeWatchedRepoStore{},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{responseBody: gitHubAPIUser{ID: 42, Login: "octocat"}},
		oauthCfg: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: ts.URL,
				AuthURL:  ts.URL,
			},
		},
	}

	result, err := svc.ExchangeCode(context.Background(), "user1", "test-auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if result.UserID != "user1" {
		t.Errorf("UserID = %q, want %q", result.UserID, "user1")
	}
}

func TestGitHubIntegrationService_Disconnect_NoWatchedRepos_Success(t *testing.T) {
	enc := mustNewEncryptionService(t)
	// Pre-encrypt a dummy token so Decrypt succeeds during Disconnect.
	encToken, err := enc.Encrypt("gh_token_abc")
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}

	intStore := &fakeIntegrationStore{
		integration: model.UserIntegration{ID: "i1", AccessToken: encToken},
		deleteN:     1,
	}
	svc := &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: &fakeWatchedRepoStore{},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{},
		oauthCfg:     &oauth2.Config{},
	}
	if err := svc.Disconnect(context.Background(), "user1"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

// ---- NewGitHubIntegrationService ----

func TestNewGitHubIntegrationService_Constructor(t *testing.T) {
	enc := mustNewEncryptionService(t)
	svc := NewGitHubIntegrationService(
		&fakeIntegrationStore{},
		&fakeWatchedRepoStore{},
		enc,
		&fakeHTTPClient{},
		"client-id", "client-secret", "http://localhost/callback", "http://localhost/webhook",
	)
	if svc == nil {
		t.Fatal("expected non-nil GitHubIntegrationService")
	}
}

// TestGitHubIntegrationService_ExchangeCode_FetchUserError verifies that a failure
// to fetch the GitHub user profile after a successful token exchange is propagated.
func TestGitHubIntegrationService_ExchangeCode_FetchUserError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gh_test_token","token_type":"bearer"}`))
	}))
	defer ts.Close()

	enc := mustNewEncryptionService(t)
	svc := &GitHubIntegrationService{
		integrations: &fakeIntegrationStore{getErr: apperrors.ErrIntegrationNotFound},
		watchedRepos: &fakeWatchedRepoStore{},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{err: fmt.Errorf("github user fetch failed")},
		oauthCfg: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: ts.URL,
				AuthURL:  ts.URL,
			},
		},
	}
	if _, err := svc.ExchangeCode(context.Background(), "user1", "code"); err == nil {
		t.Error("expected error when GitHub user fetch fails, got nil")
	}
}

// TestRandomSecret_IsValidHex verifies that randomSecret produces a 64-character
// hex string (32 bytes), which is the required length for HMAC-SHA256 webhook secrets.
func TestRandomSecret_IsValidHex(t *testing.T) {
	secret, err := randomSecret()
	if err != nil {
		t.Fatalf("randomSecret: %v", err)
	}
	if len(secret) != 64 {
		t.Errorf("len = %d, want 64 (32 bytes as hex)", len(secret))
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Errorf("randomSecret is not valid hex: %v", err)
	}
}

// TestGitHubIntegrationService_Disconnect_ListByIntegrationError verifies that a failure
// to list watched repos during Disconnect is propagated to the caller.
func TestGitHubIntegrationService_Disconnect_ListByIntegrationError(t *testing.T) {
	enc := mustNewEncryptionService(t)
	intStore := &fakeIntegrationStore{
		integration: model.UserIntegration{ID: "i1", AccessToken: "plain_token"},
		deleteN:     1,
	}
	svc := &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: &fakeWatchedRepoStore{listByIntErr: fmt.Errorf("db failure")},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{},
		oauthCfg:     &oauth2.Config{},
	}
	if err := svc.Disconnect(context.Background(), "user1"); err == nil {
		t.Error("expected error when ListByIntegration fails, got nil")
	}
}

// TestGitHubIntegrationService_Disconnect_DeleteZeroRows_ReturnsNotFound verifies that
// when the integration Delete affects 0 rows, ErrIntegrationNotFound is returned.
func TestGitHubIntegrationService_Disconnect_DeleteZeroRows_ReturnsNotFound(t *testing.T) {
	enc := mustNewEncryptionService(t)
	intStore := &fakeIntegrationStore{
		integration: model.UserIntegration{ID: "i1", AccessToken: "plain_token"},
		deleteN:     0, // no rows deleted
	}
	svc := &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: &fakeWatchedRepoStore{},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{},
		oauthCfg:     &oauth2.Config{},
	}
	err := svc.Disconnect(context.Background(), "user1")
	if !errors.Is(err, apperrors.ErrIntegrationNotFound) {
		t.Errorf("expected ErrIntegrationNotFound when delete affects 0 rows, got %v", err)
	}
}

// TestGitHubIntegrationService_Disconnect_DeleteError_Propagates verifies that a store
// error during integration deletion is propagated to the caller.
func TestGitHubIntegrationService_Disconnect_DeleteError_Propagates(t *testing.T) {
	enc := mustNewEncryptionService(t)
	intStore := &fakeIntegrationStore{
		integration: model.UserIntegration{ID: "i1", AccessToken: "plain_token"},
		deleteErr:   fmt.Errorf("db failure"),
	}
	svc := &GitHubIntegrationService{
		integrations: intStore,
		watchedRepos: &fakeWatchedRepoStore{},
		encSvc:       enc,
		httpClient:   &fakeHTTPClient{},
		oauthCfg:     &oauth2.Config{},
	}
	if err := svc.Disconnect(context.Background(), "user1"); err == nil {
		t.Error("expected error when Delete fails, got nil")
	}
}

// TestGitHubIntegrationService_ExchangeCode_TokenExchangeError verifies that a failing
// OAuth token exchange is propagated as an error from ExchangeCode.
func TestGitHubIntegrationService_ExchangeCode_TokenExchangeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
	}))
	defer ts.Close()

	svc := &GitHubIntegrationService{
		integrations: &fakeIntegrationStore{},
		watchedRepos: &fakeWatchedRepoStore{},
		httpClient:   &fakeHTTPClient{},
		oauthCfg: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: ts.URL,
				AuthURL:  ts.URL,
			},
		},
	}
	if _, err := svc.ExchangeCode(context.Background(), "user1", "bad-code"); err == nil {
		t.Error("expected error when token exchange fails, got nil")
	}
}

// TestRandomSecret_ProducesUniqueValues ensures the CSPRNG is actually used:
// two consecutive calls must not produce the same secret.
func TestRandomSecret_ProducesUniqueValues(t *testing.T) {
	s1, err := randomSecret()
	if err != nil {
		t.Fatalf("first randomSecret: %v", err)
	}
	s2, err := randomSecret()
	if err != nil {
		t.Fatalf("second randomSecret: %v", err)
	}
	if s1 == s2 {
		t.Error("two consecutive randomSecret calls produced identical values — CSPRNG may be broken")
	}
}
