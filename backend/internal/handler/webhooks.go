package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

const webhookBodyLimit = 1 << 20 // 1 MiB

// WebhooksHandler handles incoming GitHub webhook deliveries.
type WebhooksHandler struct {
	webhookSvc *service.WebhookService
	validate   *validator.Validate
	appEnv     string
}

// NewWebhooksHandler creates a new WebhooksHandler.
func NewWebhooksHandler(
	webhookSvc *service.WebhookService,
	validate *validator.Validate,
	appEnv string,
) *WebhooksHandler {
	return &WebhooksHandler{
		webhookSvc: webhookSvc,
		validate:   validate,
		appEnv:     appEnv,
	}
}

// AddRoutes registers webhook routes on the public (unauthenticated) router.
// The _simulate endpoint is only registered when AppEnv == "development".
func (h *WebhooksHandler) AddRoutes(r chi.Router) {
	r.Post("/api/webhooks/github", h.GitHubWebhook)
	if h.appEnv == "development" {
		r.Post("/api/webhooks/github/_simulate", h.Simulate)
	}
}

// GitHubWebhook receives a real GitHub webhook delivery and delegates all
// authentication, parsing, and persistence to WebhookService.ProcessDelivery.
func (h *WebhooksHandler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		response.WriteError(w, http.StatusBadRequest, "missing X-GitHub-Event header")
		return
	}
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if err := h.webhookSvc.ProcessDelivery(r.Context(), eventType, payload, sigHeader, deliveryID); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidWebhookPayload):
			response.WriteError(w, http.StatusBadRequest, "invalid payload")
		case errors.Is(err, apperrors.ErrUnauthorized):
			response.WriteError(w, http.StatusUnauthorized, "invalid signature")
		case errors.Is(err, apperrors.ErrDuplicateDelivery), errors.Is(err, apperrors.ErrUnhandledEvent):
			// Idempotent re-delivery or unhandled event type — acknowledge silently.
			w.WriteHeader(http.StatusOK)
		default:
			slog.Error("webhook: process delivery failed", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Simulate exercises the full event-parsing and notification-creation pipeline
// without HMAC validation. Only registered when AppEnv == "development".
func (h *WebhooksHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "event_type and payload are required")
		return
	}

	created, err := h.webhookSvc.SimulateDelivery(r.Context(), req.EventType, req.Payload, req.DeliveryID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidWebhookPayload):
			response.WriteError(w, http.StatusBadRequest, "invalid payload")
		case errors.Is(err, apperrors.ErrWatchedRepoNotFound):
			response.WriteError(w, http.StatusBadRequest, "no watched repos found for this repository")
		case errors.Is(err, apperrors.ErrUnhandledEvent):
			response.WriteJSON(w, http.StatusOK, map[string]string{"status": "unhandled event type or action"})
		default:
			slog.Error("simulate: delivery failed", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"created": created,
	})
}
