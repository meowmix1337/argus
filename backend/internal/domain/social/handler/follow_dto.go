package handler

import "github.com/meowmix1337/argus/backend/internal/model"

// FollowRequest is the JSON body for POST /api/follow.
type FollowRequest struct {
	FollowingID string `json:"followingId" validate:"required,len=36"`
}

// FollowStatusResponse is the JSON response for GET /api/follow/status.
type FollowStatusResponse struct {
	Following bool `json:"following"`
}

// UserSummaryResponse is the JSON response for a lightweight user profile.
type UserSummaryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// FollowListResponse is the paginated JSON response for follower/following lists.
type FollowListResponse struct {
	Users  []UserSummaryResponse `json:"users"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

func userSummaryToResponse(u model.UserSummary) UserSummaryResponse {
	return UserSummaryResponse{
		ID:        u.ID,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}
}

func userSummariesToResponse(users []model.UserSummary) []UserSummaryResponse {
	resp := make([]UserSummaryResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, userSummaryToResponse(u))
	}
	return resp
}
