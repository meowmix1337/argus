package handler

import "github.com/meowmix1337/argus/backend/internal/model"

// CreatePostRequest is the JSON body for POST /api/posts.
type CreatePostRequest struct {
	Content      string  `json:"content"      validate:"required,min=1,max=128"`
	ParentPostID *string `json:"parentPostId" validate:"omitempty,len=36"`
}

// PostResponse is the JSON response for a single post.
type PostResponse struct {
	ID           string   `json:"id"`
	UserID       string   `json:"userId"`
	UserName     string   `json:"userName"`
	UserAvatar   string   `json:"userAvatar"`
	Content      string   `json:"content"`
	ParentPostID *string  `json:"parentPostId"`
	LikeCount    int      `json:"likeCount"`
	MediaURLs    []string `json:"mediaUrls"`
	LikedByMe    bool     `json:"likedByMe"`
	CreatedAt    string   `json:"createdAt"`
}

// PostListResponse is the paginated JSON response for GET /api/posts/user/{id}.
type PostListResponse struct {
	Posts  []PostResponse `json:"posts"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// PostSearchResponse is the paginated JSON response for GET /api/posts/search.
type PostSearchResponse struct {
	Posts  []PostResponse `json:"posts"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func postToResponse(p model.Post) PostResponse {
	var mediaURLs []string
	if p.MediaURLs != nil {
		// Handler could parse JSON array here; for now pass empty slice.
		mediaURLs = []string{}
	} else {
		mediaURLs = []string{}
	}

	return PostResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		UserName:     p.UserName,
		UserAvatar:   p.UserAvatar,
		Content:      p.Content,
		ParentPostID: p.ParentPostID,
		LikeCount:    p.LikeCount,
		MediaURLs:    mediaURLs,
		LikedByMe:    p.LikedByMe,
		CreatedAt:    p.CreatedAt,
	}
}

func postsToResponse(posts []model.Post) []PostResponse {
	resp := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, postToResponse(p))
	}
	return resp
}
