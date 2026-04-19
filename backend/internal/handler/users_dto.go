package handler

import "github.com/meowmix1337/argus/backend/internal/model"

// UserSummaryResponse is the JSON response for a lightweight user profile.
type UserSummaryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// UserSearchResponse is the paginated JSON response for GET /api/users/search.
type UserSearchResponse struct {
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
