package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeIntegrationStore implements service.IntegrationStore.
type fakeIntegrationStore struct {
	integration model.UserIntegration
	err         error
	deleteRows  int64
	deleteErr   error
}

func (f *fakeIntegrationStore) Create(_ context.Context, _ model.IntegrationCreate) (model.UserIntegration, error) {
	return f.integration, f.err
}

func (f *fakeIntegrationStore) GetByUserAndProvider(_ context.Context, _, _ string) (model.UserIntegration, error) {
	return f.integration, f.err
}

func (f *fakeIntegrationStore) GetByID(_ context.Context, _, _ string) (model.UserIntegration, error) {
	return f.integration, f.err
}

func (f *fakeIntegrationStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return f.deleteRows, f.deleteErr
}

// fakeWatchedRepoStore implements service.WatchedRepoStore.
type fakeWatchedRepoStore struct {
	repos []model.WatchedRepo
	err   error
}

func (f *fakeWatchedRepoStore) Create(_ context.Context, _ model.WatchedRepoCreate) (model.WatchedRepo, error) {
	return model.WatchedRepo{}, f.err
}

func (f *fakeWatchedRepoStore) GetByID(_ context.Context, _, _ string) (model.WatchedRepo, error) {
	return model.WatchedRepo{}, f.err
}

func (f *fakeWatchedRepoStore) ListByIntegration(_ context.Context, _, _ string) ([]model.WatchedRepo, error) {
	return f.repos, f.err
}

func (f *fakeWatchedRepoStore) GetByOwnerRepo(_ context.Context, _, _ string) ([]model.WatchedRepo, error) {
	return f.repos, f.err
}

func (f *fakeWatchedRepoStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return 0, f.err
}

func newTestIntegrationsHandler(integStore service.IntegrationStore, watchedStore service.WatchedRepoStore) *IntegrationsHandler {
	svc := service.NewGitHubIntegrationService(integStore, watchedStore, nil, nil, "", "", "", "")
	return NewIntegrationsHandler(svc, validator.New())
}

func TestGetIntegrationStatus(t *testing.T) {
	tests := []struct {
		name          string
		noSession     bool
		integration   model.UserIntegration
		storeErr      error
		wantStatus    int
		wantConnected bool
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "integration not found returns 200 disconnected",
			storeErr:      apperrors.ErrIntegrationNotFound,
			wantStatus:    http.StatusOK,
			wantConnected: false,
		},
		{
			name:          "integration found returns 200 connected",
			integration:   model.UserIntegration{ProviderUsername: "octocat"},
			wantStatus:    http.StatusOK,
			wantConnected: true,
		},
		{
			name:       "store error returns 500",
			storeErr:   errors.New("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intStore := &fakeIntegrationStore{integration: tt.integration, err: tt.storeErr}
			h := newTestIntegrationsHandler(intStore, &fakeWatchedRepoStore{})

			req := httptest.NewRequest(http.MethodGet, "/api/integrations/github", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.GetStatus(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp IntegrationStatusResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Connected != tt.wantConnected {
				t.Errorf("connected = %v, want %v", resp.Connected, tt.wantConnected)
			}
		})
	}
}

func TestDisconnectIntegration(t *testing.T) {
	tests := []struct {
		name       string
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "integration not found returns 404",
			storeErr:   apperrors.ErrIntegrationNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "store error returns 500",
			storeErr:   errors.New("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intStore := &fakeIntegrationStore{err: tt.storeErr}
			h := newTestIntegrationsHandler(intStore, &fakeWatchedRepoStore{})

			req := httptest.NewRequest(http.MethodDelete, "/api/integrations/github", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Disconnect(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestListRepos(t *testing.T) {
	tests := []struct {
		name       string
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "integration not found returns 404",
			storeErr:   apperrors.ErrIntegrationNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "store error returns 500",
			storeErr:   errors.New("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intStore := &fakeIntegrationStore{err: tt.storeErr}
			h := newTestIntegrationsHandler(intStore, &fakeWatchedRepoStore{})

			req := httptest.NewRequest(http.MethodGet, "/api/integrations/github/repos", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.ListRepos(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestUpdateWatchedRepos(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			body:       `{"repos":["owner/repo"]}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid body returns 400",
			body:       "not-json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "too many repos returns 400",
			body:       buildReposJSON(maxWatchedRepos + 1),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid repo name format returns 400",
			body:       `{"repos":["invalid repo name!"]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "integration not found returns 404",
			body:       `{"repos":["owner/repo"]}`,
			storeErr:   apperrors.ErrIntegrationNotFound,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intStore := &fakeIntegrationStore{err: tt.storeErr}
			h := newTestIntegrationsHandler(intStore, &fakeWatchedRepoStore{})

			req := httptest.NewRequest(http.MethodPut, "/api/integrations/github/repos",
				bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.UpdateWatchedRepos(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// buildReposJSON builds a JSON body with n valid "owner/repoN" entries.
func buildReposJSON(n int) string {
	repos := make([]string, n)
	for i := range repos {
		repos[i] = fmt.Sprintf("owner/repo%d", i)
	}
	b, _ := json.Marshal(map[string][]string{"repos": repos})
	return string(b)
}
