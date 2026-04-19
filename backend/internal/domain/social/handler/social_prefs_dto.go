package handler

// UpdateSocialPrefsRequest is the request body for PUT /api/settings/social-notifications.
type UpdateSocialPrefsRequest struct {
	MutePosts   bool `json:"mutePosts"`
	MuteFollows bool `json:"muteFollows"`
}

// SocialPrefsResponse is the response body for GET /api/settings/social-notifications.
type SocialPrefsResponse struct {
	MutePosts   bool `json:"mutePosts"`
	MuteFollows bool `json:"muteFollows"`
}
