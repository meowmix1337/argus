package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

const webhookBodyLimit = 1 << 20 // 1 MiB

// WebhooksHandler handles incoming GitHub webhook deliveries.
type WebhooksHandler struct {
	webhookSvc      *service.WebhookService
	notificationSvc *service.NotificationService
	appEnv          string
}

// NewWebhooksHandler creates a new WebhooksHandler.
func NewWebhooksHandler(
	webhookSvc *service.WebhookService,
	notificationSvc *service.NotificationService,
	appEnv string,
) *WebhooksHandler {
	return &WebhooksHandler{
		webhookSvc:      webhookSvc,
		notificationSvc: notificationSvc,
		appEnv:          appEnv,
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

// GitHubWebhook receives a real GitHub webhook delivery, validates the HMAC
// signature, parses the event, and creates a notification for the affected user.
func (h *WebhooksHandler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	owner, repo, err := service.ExtractOwnerRepo(payload)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid payload: missing repository")
		return
	}

	watchedRepos, err := h.webhookSvc.GetWatchedRepos(ctx, owner, repo)
	if err != nil {
		slog.Error("webhook: failed to look up watched repos", "owner", owner, "repo", repo, "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Find the watched repo whose decrypted secret validates the signature.
	// Each user installs their own webhook with a unique secret, so exactly one
	// entry should match per delivery.
	var matched *model.WatchedRepo
	for i := range watchedRepos {
		secret, err := h.webhookSvc.DecryptWebhookSecret(watchedRepos[i].WebhookSecret)
		if err != nil {
			slog.Warn("webhook: failed to decrypt webhook secret", "watched_repo_id", watchedRepos[i].ID, "error", err)
			continue
		}
		if service.ValidateHMACSignature([]byte(secret), payload, sigHeader) {
			matched = &watchedRepos[i]
			break
		}
	}

	if matched == nil {
		response.WriteError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	parsed, err := service.ParseGitHubEvent(eventType, payload)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "malformed event payload")
		return
	}
	if parsed == nil {
		// Unhandled event type or action — acknowledge without creating a notification.
		w.WriteHeader(http.StatusOK)
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("webhook: failed to generate notification ID", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	parsed.ID = id.String()
	parsed.UserID = matched.UserID
	if deliveryID != "" {
		parsed.GitHubDeliveryID = &deliveryID
	}

	if _, err := h.notificationSvc.Create(ctx, *parsed); err != nil {
		if errors.Is(err, apperrors.ErrDuplicateDelivery) {
			// Idempotent re-delivery — GitHub retried; silently acknowledge.
			w.WriteHeader(http.StatusOK)
			return
		}
		slog.Error("webhook: failed to create notification", "error", err, "user_id", matched.UserID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Simulate exercises the full event-parsing and notification-creation pipeline
// without HMAC validation. Only registered when AppEnv == "development".
func (h *WebhooksHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, webhookBodyLimit)
	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EventType == "" || len(req.Payload) == 0 {
		response.WriteError(w, http.StatusBadRequest, "event_type and payload are required")
		return
	}

	owner, repo, err := service.ExtractOwnerRepo(req.Payload)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid payload: missing repository")
		return
	}

	watchedRepos, err := h.webhookSvc.GetWatchedRepos(ctx, owner, repo)
	if err != nil {
		slog.Error("simulate: failed to look up watched repos", "owner", owner, "repo", repo, "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if len(watchedRepos) == 0 {
		response.WriteError(w, http.StatusBadRequest, "no watched repos found for this repository")
		return
	}

	parsed, err := service.ParseGitHubEvent(req.EventType, req.Payload)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "malformed event payload")
		return
	}
	if parsed == nil {
		response.WriteJSON(w, http.StatusOK, map[string]string{"status": "unhandled event type or action"})
		return
	}

	deliveryID := req.DeliveryID
	if deliveryID == "" {
		generated, err := uuid.NewV7()
		if err != nil {
			slog.Error("simulate: failed to generate delivery ID", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		deliveryID = generated.String()
	}

	created := 0
	for _, wr := range watchedRepos {
		id, err := uuid.NewV7()
		if err != nil {
			slog.Error("simulate: failed to generate notification ID", "error", err)
			continue
		}
		n := *parsed
		n.ID = id.String()
		n.UserID = wr.UserID
		n.GitHubDeliveryID = &deliveryID

		if _, err := h.notificationSvc.Create(ctx, n); err != nil {
			if errors.Is(err, apperrors.ErrDuplicateDelivery) {
				continue
			}
			slog.Error("simulate: failed to create notification", "error", err, "user_id", wr.UserID)
			continue
		}
		created++
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"created": created,
	})
}
