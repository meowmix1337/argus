package model

// Post is a social feed post (max 128 characters).
type Post struct {
	ID           string  `json:"id"`
	UserID       string  `json:"userId"`
	UserName     string  `json:"userName"`
	UserAvatar   string  `json:"userAvatar"`
	Content      string  `json:"content"`
	ParentPostID *string `json:"parentPostId"`
	LikeCount    int     `json:"likeCount"`
	MediaURLs    *string `json:"-"` // JSON array, decoded by handler
	LikedByMe    bool    `json:"likedByMe"`
	CreatedAt    string  `json:"createdAt"`
}

// PostCreate holds fields for inserting a new post.
type PostCreate struct {
	ID           string
	UserID       string
	Content      string
	ParentPostID *string
	MediaURLs    *string // JSON array
}

// Follower represents a follow relationship.
type Follower struct {
	ID          string `json:"id"`
	FollowerID  string `json:"followerId"`
	FollowingID string `json:"followingId"`
	CreatedAt   string `json:"createdAt"`
}

// UserSummary is a lightweight user profile for discover/follower lists.
type UserSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// FeedCursor holds pagination state for cursor-based feed queries.
type FeedCursor struct {
	CreatedAt string `json:"createdAt"` // ISO timestamp of last item
	ID        string `json:"id"`        // tie-breaker
}
