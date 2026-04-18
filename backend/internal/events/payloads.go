package events

import "time"

// Event topics — stable strings, never rename in production.
const (
	TopicPostCreated  = "post.created"
	TopicPostLiked    = "post.liked"
	TopicUserFollowed = "user.followed"
)

// EventEnvelope wraps every published message with version, type, and timestamp.
type EventEnvelope struct {
	Version   int    `json:"v"`
	Type      string `json:"type"`
	Timestamp string `json:"ts"`
	Payload   any    `json:"payload"`
}

// NewEnvelope creates an EventEnvelope with version 1 and the current UTC timestamp.
func NewEnvelope(eventType string, payload any) EventEnvelope {
	return EventEnvelope{
		Version:   1,
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
}

// PostCreatedPayload is the payload for TopicPostCreated events.
type PostCreatedPayload struct {
	PostID         string `json:"postId"`
	UserID         string `json:"userId"`
	AuthorName     string `json:"authorName"`
	ContentPreview string `json:"contentPreview"` // first 100 runes of content
}

// PostLikedPayload is the payload for TopicPostLiked events.
type PostLikedPayload struct {
	PostID  string `json:"postId"`
	LikerID string `json:"likerId"`
	OwnerID string `json:"ownerId"`
}

// UserFollowedPayload is the payload for TopicUserFollowed events.
type UserFollowedPayload struct {
	FollowerID   string `json:"followerId"`
	FollowingID  string `json:"followingId"`
	FollowerName string `json:"followerName"`
}
