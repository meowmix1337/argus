package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/meowmix1337/argus/backend/internal/httpclient"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// NewsService fetches top headlines from GNews.
type NewsService struct {
	httpClient httpclient.HTTPClient
	apiKey     string
	cache      *CacheService
}

// NewNewsService creates a new NewsService.
func NewNewsService(httpClient httpclient.HTTPClient, apiKey string, cache *CacheService) *NewsService {
	return &NewsService{
		httpClient: httpClient,
		apiKey:     apiKey,
		cache:      cache,
	}
}

// 3h TTL: 9 categories × 8 cache misses/day = 72 requests, under GNews free tier (100/day).
const newsCacheTTL = 3 * time.Hour

var newsCategories = []string{
	"general", "world", "nation", "business", "technology",
	"entertainment", "sports", "science", "health",
}

// Fetch retrieves top news headlines for all categories.
func (s *NewsService) Fetch(ctx context.Context) ([]model.NewsCategory, error) {
	const cacheKey = "news"
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]model.NewsCategory), nil
	}
	if s.apiKey == "" {
		return nil, fmt.Errorf("GNEWS_API_KEY not configured")
	}
	categories, err := s.fetchAllCategories(ctx)
	if err != nil {
		return nil, err
	}
	s.cache.Set(cacheKey, categories, newsCacheTTL)
	return categories, nil
}

// fetchAllCategories fetches each category sequentially, respecting GNews's 1 req/sec rate limit.
func (s *NewsService) fetchAllCategories(ctx context.Context) ([]model.NewsCategory, error) {
	out := make([]model.NewsCategory, 0, len(newsCategories))
	for i, cat := range newsCategories {
		if i > 0 {
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		items, err := s.fetchCategory(ctx, cat)
		if err != nil {
			slog.Warn("news: category fetch failed", "category", cat, "error", err)
			items = []model.NewsItem{}
		}
		out = append(out, model.NewsCategory{Name: cat, Items: items})
	}
	return out, nil
}

func (s *NewsService) fetchCategory(ctx context.Context, category string) ([]model.NewsItem, error) {
	var gnewsResp gNewsResponse
	if err := s.httpClient.Get(ctx,
		"https://gnews.io/api/v4/top-headlines",
		&gnewsResp,
		httpclient.WithHeader("X-Api-Key", s.apiKey),
		httpclient.WithQueryParams(map[string]string{
			"category": category,
			"country":  "us",
			"lang":     "en",
			"max":      "8",
		}),
	); err != nil {
		return nil, err
	}

	items := make([]model.NewsItem, 0, len(gnewsResp.Articles))
	for _, a := range gnewsResp.Articles {
		pub, err := time.Parse(time.RFC3339, a.PublishedAt)
		if err != nil {
			pub, err = time.Parse(time.RFC3339Nano, a.PublishedAt)
			if err != nil {
				slog.Warn("news: failed to parse publishedAt", "raw", a.PublishedAt, "error", err)
			}
		}
		items = append(items, model.NewsItem{
			Title:  a.Title,
			Source: a.Source.Name,
			Time:   relativeTime(pub),
			URL:    a.URL,
		})
	}
	return items, nil
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "recently"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// GNews API response types
type gNewsResponse struct {
	Articles []struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		PublishedAt string `json:"publishedAt"`
		Source      struct {
			Name string `json:"name"`
		} `json:"source"`
	} `json:"articles"`
}
