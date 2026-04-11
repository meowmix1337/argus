package handler

// FeedResponse is the JSON response for GET /api/feed.
type FeedResponse struct {
	Posts      []PostResponse `json:"posts"`
	NextCursor *string        `json:"nextCursor"` // opaque cursor for next page, null if no more
}
