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
)
