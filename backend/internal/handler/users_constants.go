package handler

import "time"

const (
	sessionDuration  = 7 * 24 * time.Hour
	sessionMaxAge    = int(sessionDuration / time.Second)
	oauthStateMaxAge = 5 * 60
)
