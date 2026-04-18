package handler

import "time"

const (
	// Task pagination
	defaultTaskLimit = 5
	maxTaskLimit     = 100

	// Rate limiting for mutation endpoints (requests per second per IP).
	// Read endpoints are not rate-limited; search endpoints use searchRateLimit.
	mutationRateLimit = 10
	searchRateLimit   = 2
	rateLimitWindow   = time.Second

	// sessionDuration is the lifetime of a user session cookie and JWT.
	sessionDuration = 7 * 24 * time.Hour
	// sessionMaxAge is sessionDuration expressed in seconds for use in http.Cookie.MaxAge.
	sessionMaxAge = int(sessionDuration / time.Second)

	// oauthStateMaxAge is the lifetime of the short-lived OAuth state cookie.
	oauthStateMaxAge = 5 * 60 // 5 minutes in seconds

	// Stocks watchlist pagination
	defaultWatchlistLimit = 20
	maxWatchlistLimit     = 50

	// Bill pagination
	defaultBillLimit = 50
	maxBillLimit     = 200

	// Notification pagination
	defaultNotificationLimit   = 20
	maxNotificationLimit       = 100
	maxNotificationSearchQuery = 200

	// Post pagination
	defaultPostLimit = 20
	maxPostLimit     = 100

	// Follow pagination
	defaultFollowLimit = 20
	maxFollowLimit     = 100

	// Feed pagination
	defaultFeedLimit = 20
	maxFeedLimit     = 50

	// GitHub integration
	maxWatchedRepos = 20
)

// allowedNotificationProviders is the set of valid provider IDs for the notification filter.
// Must stay in sync with seeded rows in migrations/013_create_provider_types.sql and
// migrations/021_seed_social_provider_type.sql.
var allowedNotificationProviders = map[string]struct{}{
	"github": {},
	"social": {},
}

// allowedNotificationEventTypes is the set of valid event type IDs for the notification filter.
// Must stay in sync with seeded rows in migrations/014_create_notification_event_types.sql and
// migrations/022_seed_social_event_types.sql.
var allowedNotificationEventTypes = map[string]struct{}{
	"pr_opened":           {},
	"pr_merged":           {},
	"pr_closed":           {},
	"pr_comment":          {},
	"pr_review_comment":   {},
	"social.post.created": {},
	"social.new_follower": {},
}
