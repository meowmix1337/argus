package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeNotificationStore is an in-memory NotificationStore for handler tests.
type fakeNotificationStore struct {
	notifications []model.Notification
	total         int
	markRows      int64
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
	return f.markRows, f.err
}

func (f *fakeNotificationStore) MarkDismissed(_ context.Context, _, _ string) (int64, error) {
	return f.markRows, f.err
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

func TestPatchNotification(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		markRows   int64
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{name: "no session returns 401", noSession: true, wantStatus: http.StatusUnauthorized},
		{name: "empty body returns 400", body: "", wantStatus: http.StatusBadRequest},
		{name: "invalid action returns 400", body: `{"action":"invalid"}`, wantStatus: http.StatusBadRequest},
		{name: "notification not found returns 404", body: `{"action":"read"}`, markRows: 0, wantStatus: http.StatusNotFound},
		{name: "mark read returns 204", body: `{"action":"read"}`, markRows: 1, wantStatus: http.StatusNoContent},
		{name: "mark dismissed returns 204", body: `{"action":"dismissed"}`, markRows: 1, wantStatus: http.StatusNoContent},
		{name: "service error returns 500", body: `{"action":"read"}`, storeErr: fmt.Errorf("db error"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeNotificationStore{markRows: tt.markRows, err: tt.storeErr}
			h := newTestNotificationsHandler(store)

			req := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
				req = withChiParam(req, "id", "notif-1")
			}
			w := httptest.NewRecorder()
			h.Patch(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestListNotificationsWithData(t *testing.T) {
	notif := model.Notification{ID: "n1", Title: "test", ProviderID: "github", EventTypeID: "pr_opened"}
	store := &fakeNotificationStore{notifications: []model.Notification{notif}, total: 1}
	h := newTestNotificationsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	req = withSession(req, "user1")
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp NotificationListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Notifications) != 1 {
		t.Fatalf("len(notifications) = %d, want 1", len(resp.Notifications))
	}
	if resp.Notifications[0].ID != "n1" {
		t.Errorf("notification ID = %q, want %q", resp.Notifications[0].ID, "n1")
	}
	if resp.Notifications[0].ProviderID != "github" {
		t.Errorf("providerID = %q, want %q", resp.Notifications[0].ProviderID, "github")
	}
	if resp.Notifications[0].EventTypeID != "pr_opened" {
		t.Errorf("eventTypeID = %q, want %q", resp.Notifications[0].EventTypeID, "pr_opened")
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
}

func TestListNotificationsErrors(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "invalid state returns 400",
			query:      "?state=invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "search query too long returns 400",
			query:      "?q=" + strings.Repeat("A", maxNotificationSearchQuery+1),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service error returns 500",
			query:      "",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store *fakeNotificationStore
			if tt.wantStatus == http.StatusInternalServerError {
				store = &fakeNotificationStore{err: fmt.Errorf("db error")}
			} else {
				store = &fakeNotificationStore{}
			}
			h := newTestNotificationsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/notifications"+tt.query, nil)
			req = withSession(req, "user1")
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestMarkAllRead(t *testing.T) {
	tests := []struct {
		name       string
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{name: "no session returns 401", noSession: true, wantStatus: http.StatusUnauthorized},
		{name: "marks all read returns 204", wantStatus: http.StatusNoContent},
		{name: "service error returns 500", storeErr: fmt.Errorf("db error"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeNotificationStore{err: tt.storeErr}
			h := newTestNotificationsHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/notifications/mark-all-read", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.MarkAllRead(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
