package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeNotificationStore is an in-memory NotificationStore for handler tests.
type fakeNotificationStore struct {
	notifications []model.Notification
	total         int
	err           error
	// captured args from the most recent List call
	lastProviderID  string
	lastEventTypeID string
}

func (f *fakeNotificationStore) List(_ context.Context, _, _, _, providerID, eventTypeID string, _, _ int) ([]model.Notification, int, error) {
	f.lastProviderID = providerID
	f.lastEventTypeID = eventTypeID
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.notifications, f.total, nil
}

func (f *fakeNotificationStore) Create(_ context.Context, _ model.NotificationCreate) (model.Notification, error) {
	return model.Notification{}, f.err
}

func (f *fakeNotificationStore) GetByID(_ context.Context, _, _ string) (model.Notification, error) {
	return model.Notification{}, f.err
}

func (f *fakeNotificationStore) MarkRead(_ context.Context, _, _ string) (int64, error) {
	return 0, f.err
}

func (f *fakeNotificationStore) MarkDismissed(_ context.Context, _, _ string) (int64, error) {
	return 0, f.err
}

func (f *fakeNotificationStore) MarkAllRead(_ context.Context, _ string) (int64, error) {
	return 0, f.err
}

func (f *fakeNotificationStore) CountUnread(_ context.Context, _ string) (int, error) {
	return 0, f.err
}

func newTestNotificationsHandler(store service.NotificationStore) *NotificationsHandler {
	svc := service.NewNotificationService(store)
	return NewNotificationsHandler(svc, validator.New())
}

func TestListNotifications(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		noSession       bool
		wantStatus      int
		wantProviderID  string
		wantEventTypeID string
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "no filter params returns 200",
			wantStatus: http.StatusOK,
		},
		{
			name:           "valid provider returns 200 and forwards providerID",
			query:          "?provider=github",
			wantStatus:     http.StatusOK,
			wantProviderID: "github",
		},
		{
			name:            "valid event_type returns 200 and forwards eventTypeID",
			query:           "?event_type=pr_merged",
			wantStatus:      http.StatusOK,
			wantEventTypeID: "pr_merged",
		},
		{
			name:            "combined provider and event_type returns 200",
			query:           "?provider=github&event_type=pr_comment",
			wantStatus:      http.StatusOK,
			wantProviderID:  "github",
			wantEventTypeID: "pr_comment",
		},
		{
			name:           "combined provider and search returns 200",
			query:          "?provider=github&q=fix",
			wantStatus:     http.StatusOK,
			wantProviderID: "github",
		},
		{
			name:       "invalid provider returns 400",
			query:      "?provider=slack",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid event_type returns 400",
			query:      "?event_type=unknown_event",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeNotificationStore{}
			h := newTestNotificationsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/notifications"+tt.query, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp NotificationListResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if store.lastProviderID != tt.wantProviderID {
				t.Errorf("providerID forwarded = %q, want %q", store.lastProviderID, tt.wantProviderID)
			}
			if store.lastEventTypeID != tt.wantEventTypeID {
				t.Errorf("eventTypeID forwarded = %q, want %q", store.lastEventTypeID, tt.wantEventTypeID)
			}
		})
	}
}
