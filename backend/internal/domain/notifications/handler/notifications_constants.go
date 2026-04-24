package handler

const (
	defaultNotificationLimit   = 20
	maxNotificationLimit       = 100
	maxNotificationSearchQuery = 200
)

var allowedNotificationProviders = map[string]struct{}{
	"github": {},
	"social": {},
}

var allowedNotificationEventTypes = map[string]struct{}{
	"pr_opened":           {},
	"pr_merged":           {},
	"pr_closed":           {},
	"pr_comment":          {},
	"pr_review_comment":   {},
	"social.post.created": {},
	"social.new_follower": {},
}
