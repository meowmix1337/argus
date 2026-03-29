package errors

import "errors"

// Domain errors — used by services, mapped to HTTP status codes by handlers.

var ErrNotFound = errors.New("not found")
var ErrTaskNotFound = errors.New("task not found")
var ErrSettingsNotFound = errors.New("user settings not found")
var ErrCategoryNotFound = errors.New("news category not found")
var ErrSymbolNotFound = errors.New("symbol not in watchlist")
var ErrLabelNotFound = errors.New("label not found")
var ErrIntegrationNotFound = errors.New("integration not found")
var ErrWatchedRepoNotFound = errors.New("watched repo not found")
var ErrNotificationNotFound = errors.New("notification not found")

// Validation errors.

var ErrValidation = errors.New("validation failed")
var ErrTaskValidation = errors.New("task validation failed")
var ErrSettingsValidation = errors.New("settings validation failed")
var ErrLabelValidation = errors.New("label validation failed")
var ErrIntegrationValidation = errors.New("integration validation failed")

// Conflict errors.

var ErrLabelAlreadyAssigned = errors.New("label already assigned to task")
var ErrLabelAssignmentNotFound = errors.New("label assignment not found")
var ErrIntegrationAlreadyExists = errors.New("integration already connected")
var ErrRepoAlreadyWatched = errors.New("repo already watched")
var ErrDuplicateDelivery = errors.New("duplicate webhook delivery")

// Webhook errors.

var ErrInvalidWebhookPayload = errors.New("invalid webhook payload")
var ErrUnhandledEvent = errors.New("unhandled webhook event type or action")

// Auth errors.

var ErrBillNotFound = errors.New("bill not found")
var ErrBillValidation = errors.New("bill validation failed")
var ErrBillPaymentNotFound = errors.New("bill payment not found")
var ErrBillAlreadyPaid = errors.New("bill occurrence already marked as paid")

var ErrUnauthorized = errors.New("unauthorized")
