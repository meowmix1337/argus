package handler

import "encoding/json"

// SimulateRequest is the body for POST /api/webhooks/github/_simulate (dev only).
// It exercises the full event-parsing and notification-creation pipeline without
// HMAC validation.
type SimulateRequest struct {
	EventType  string          `json:"event_type"  validate:"required"`
	DeliveryID string          `json:"delivery_id"` // optional; a UUID is generated if empty
	Payload    json.RawMessage `json:"payload"     validate:"required"`
}
