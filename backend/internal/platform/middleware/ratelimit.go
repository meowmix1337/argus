package middleware

import "time"

const (
	// MutationRateLimit is the max requests per second per IP for mutation endpoints.
	MutationRateLimit = 10
	// SearchRateLimit is the max requests per second per IP for search endpoints.
	SearchRateLimit = 2
	// RateLimitWindow is the time window for rate limiting.
	RateLimitWindow = time.Second
)
