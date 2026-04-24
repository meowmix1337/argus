package handler

import "github.com/meowmix1337/argus/backend/internal/model"

// NotificationResponse is the JSON representation of a single notification.
type NotificationResponse struct {
	ID          string  `json:"id"`
	ProviderID  string  `json:"providerId"`
	EventTypeID string  `json:"eventTypeId"`
	Title       string  `json:"title"`
	Body        *string `json:"body"`
	URL         *string `json:"url"`
	ReadAt      *string `json:"readAt"`
	DismissedAt *string `json:"dismissedAt"`
	CreatedAt   string  `json:"createdAt"`
}

// NotificationListResponse is the paginated JSON response for GET /api/notifications.
type NotificationListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	Total         int                    `json:"total"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
}

// PatchNotificationRequest is the JSON body for PATCH /api/notifications/{id}.
type PatchNotificationRequest struct {
	Action string `json:"action" validate:"required,oneof=read dismissed"`
}

func notificationToResponse(n model.Notification) NotificationResponse {
	return NotificationResponse{
		ID:          n.ID,
		ProviderID:  n.ProviderID,
		EventTypeID: n.EventTypeID,
		Title:       n.Title,
		Body:        n.Body,
		URL:         n.URL,
		ReadAt:      n.ReadAt,
		DismissedAt: n.DismissedAt,
		CreatedAt:   n.CreatedAt,
	}
}

func notificationsToResponse(notifications []model.Notification) []NotificationResponse {
	resp := make([]NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		resp = append(resp, notificationToResponse(n))
	}
	return resp
}
