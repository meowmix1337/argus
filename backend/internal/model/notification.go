package model

// UserIntegration represents a connected OAuth provider for a user.
type UserIntegration struct {
	ID               string  `db:"id"`
	UserID           string  `db:"user_id"`
	ProviderID       string  `db:"provider_id"`       // FK to provider_types; "github", future: "slack", "email"
	AccessToken      string  `db:"access_token"`      // encrypted
	RefreshToken     *string `db:"refresh_token"`     // encrypted, nullable
	ProviderUserID   string  `db:"provider_user_id"`  // e.g. GitHub numeric user ID
	ProviderUsername string  `db:"provider_username"` // e.g. GitHub login handle
	CreatedAt        string  `db:"created_at"`
	UpdatedAt        string  `db:"updated_at"`
	DeletedAt        *string `db:"deleted_at"`
}

// WatchedRepo represents a GitHub repo the user has configured Argus to watch.
type WatchedRepo struct {
	ID            string  `db:"id"`
	UserID        string  `db:"user_id"`
	IntegrationID string  `db:"integration_id"`
	Owner         string  `db:"owner"`
	Repo          string  `db:"repo"`
	WebhookID     string  `db:"webhook_id"`     // GitHub webhook ID for cleanup
	WebhookSecret string  `db:"webhook_secret"` // encrypted HMAC secret
	CreatedAt     string  `db:"created_at"`
	UpdatedAt     string  `db:"updated_at"`
	DeletedAt     *string `db:"deleted_at"`
}

// Notification is a single inbox notification for a user.
type Notification struct {
	ID               string  `db:"id"`
	UserID           string  `db:"user_id"`
	ProviderID       string  `db:"provider_id"`   // FK to provider_types
	EventTypeID      string  `db:"event_type_id"` // FK to notification_event_types
	Title            string  `db:"title"`
	Body             *string `db:"body"`
	URL              *string `db:"url"`
	ReadAt           *string `db:"read_at"`
	DismissedAt      *string `db:"dismissed_at"`
	GitHubDeliveryID *string `db:"github_delivery_id"`
	CreatedAt        string  `db:"created_at"`
	UpdatedAt        string  `db:"updated_at"`
	DeletedAt        *string `db:"deleted_at"`
}

// NotificationCreate holds the fields for inserting a new notification.
type NotificationCreate struct {
	ID               string
	UserID           string
	ProviderID       string
	EventTypeID      string
	Title            string
	Body             *string
	URL              *string
	GitHubDeliveryID *string
}

// GitHubRepo is a repository returned from the GitHub API (not stored in DB).
type GitHubRepo struct {
	FullName string `json:"fullName"` // "owner/repo"
	Private  bool   `json:"private"`
	Watched  bool   `json:"watched"` // true if user already has a webhook on this repo
}
