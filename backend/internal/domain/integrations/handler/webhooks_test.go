package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	integrationsrepo "github.com/meowmix1337/argus/backend/internal/domain/integrations/repository"
	integrationssvc "github.com/meowmix1337/argus/backend/internal/domain/integrations/service"
	"github.com/meowmix1337/argus/backend/internal/model"
)

func newTestWebhooksHandler(appEnv string) *WebhooksHandler {
	svc := integrationssvc.NewWebhookService(nil, nil, nil)
	return NewWebhooksHandler(svc, validator.New(), appEnv)
}

func newTestWebhooksHandlerWithStores(appEnv string, watchedStore integrationsrepo.WatchedRepoStore, notifStore integrationssvc.NotificationWriter) *WebhooksHandler {
	svc := integrationssvc.NewWebhookService(watchedStore, notifStore, nil)
	return NewWebhooksHandler(svc, validator.New(), appEnv)
}

// fakeNotificationStore is a minimal NotificationWriter for webhook handler tests.
type fakeNotificationStore struct{}

func (f *fakeNotificationStore) Create(_ context.Context, _ model.NotificationCreate) (model.Notification, error) {
	return model.Notification{}, nil
}

// validGitHubPayload is a minimal GitHub webhook payload with a repository full_name.
const validGitHubPayload = `{"repository":{"full_name":"owner/repo"},"action":"opened","pull_request":{"number":1,"title":"Test PR","user":{"login":"alice"},"html_url":"https://github.com/owner/repo/pull/1","body":""}}`

func TestGitHubWebhook(t *testing.T) {
	tests := []struct {
		name         string
		eventType    string // empty = don't set the header
		payload      string
		watchedRepos []model.WatchedRepo
		watchedErr   error
		wantStatus   int
	}{
		{
			name:       "missing X-GitHub-Event header returns 400",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid header with missing repository in payload returns 400",
			eventType:  "push",
			payload:    `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			// valid payload + no watched repos → loop skips → ErrUnauthorized → 401
			name:         "no matching repos returns 401",
			eventType:    "pull_request",
			payload:      validGitHubPayload,
			watchedRepos: []model.WatchedRepo{},
			wantStatus:   http.StatusUnauthorized,
		},
		{
			// store error from GetByOwnerRepo wraps as non-sentinel → 500
			name:       "store error returns 500",
			eventType:  "pull_request",
			payload:    validGitHubPayload,
			watchedErr: fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *WebhooksHandler
			if tt.watchedRepos != nil || tt.watchedErr != nil {
				watchedStore := &fakeWatchedRepoStore{repos: tt.watchedRepos, err: tt.watchedErr}
				h = newTestWebhooksHandlerWithStores("production", watchedStore, &fakeNotificationStore{})
			} else {
				h = newTestWebhooksHandler("production")
			}

			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github",
				bytes.NewReader([]byte(tt.payload)))
			if tt.eventType != "" {
				req.Header.Set("X-GitHub-Event", tt.eventType)
			}
			w := httptest.NewRecorder()
			h.GitHubWebhook(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestSimulateWebhook(t *testing.T) {
	// validSimulateBody is a simulate request with a valid repository payload
	// and a known PR event type.
	const validSimulateBody = `{"event_type":"pull_request","payload":` + validGitHubPayload + `}`
	// unknownEventBody uses a known-valid repo payload but an unhandled event type.
	const unknownEventBody = `{"event_type":"unknown_event","payload":` + validGitHubPayload + `}`

	tests := []struct {
		name         string
		body         string
		watchedRepos []model.WatchedRepo
		watchedErr   error
		useStores    bool
		wantStatus   int
	}{
		{
			name:       "empty body returns 400",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing event_type returns 400",
			body:       `{"payload":{}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing payload returns 400",
			body:       `{"event_type":"push"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid fields but invalid repository payload returns 400",
			body:       `{"event_type":"push","payload":{}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			// valid payload + no watched repos → ErrWatchedRepoNotFound → 400
			name:         "no watched repos returns 400",
			body:         validSimulateBody,
			useStores:    true,
			watchedRepos: []model.WatchedRepo{},
			wantStatus:   http.StatusBadRequest,
		},
		{
			// valid payload + watched repos + unknown event type → ErrUnhandledEvent → 200
			name:         "unhandled event type returns 200",
			body:         unknownEventBody,
			useStores:    true,
			watchedRepos: []model.WatchedRepo{{ID: "repo-1", UserID: "user1"}},
			wantStatus:   http.StatusOK,
		},
		{
			// store error from GetByOwnerRepo → wraps non-sentinel → 500
			name:       "store error returns 500",
			body:       validSimulateBody,
			useStores:  true,
			watchedErr: fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *WebhooksHandler
			if tt.useStores {
				watchedStore := &fakeWatchedRepoStore{repos: tt.watchedRepos, err: tt.watchedErr}
				h = newTestWebhooksHandlerWithStores("development", watchedStore, &fakeNotificationStore{})
			} else {
				h = newTestWebhooksHandler("development")
			}

			req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github/_simulate",
				bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.Simulate(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
