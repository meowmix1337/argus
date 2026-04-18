package service

import (
	"context"
	"fmt"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

// QuotesService fetches a daily quote from API Ninjas.
type QuotesService struct {
	httpClient httpclient.HTTPClient
	apiKey     string
	cache      *CacheService
}

// NewQuotesService creates a new QuotesService.
func NewQuotesService(httpClient httpclient.HTTPClient, apiKey string, cache *CacheService) *QuotesService {
	return &QuotesService{
		httpClient: httpClient,
		apiKey:     apiKey,
		cache:      cache,
	}
}

const (
	quotesCacheTTL = 24 * time.Hour
	quotesAPIURL   = "https://api.api-ninjas.com/v2/quoteoftheday?categories=wisdom,philosophy,life,truth,inspirational"
)

// Fetch retrieves a random daily quote.
func (s *QuotesService) Fetch(ctx context.Context) (model.Quote, error) {
	const cacheKey = "quote"
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.(model.Quote), nil
	}

	if s.apiKey == "" {
		return model.Quote{}, fmt.Errorf("API_NINJAS_API_KEY not configured")
	}

	quote, err := s.fetchFromAPI(ctx)
	if err != nil {
		return model.Quote{}, err
	}

	s.cache.Set(cacheKey, quote, quotesCacheTTL)
	return quote, nil
}

func (s *QuotesService) fetchFromAPI(ctx context.Context) (model.Quote, error) {
	var quotes []apiNinjasQuote
	if err := s.httpClient.Get(ctx,
		quotesAPIURL,
		&quotes,
		httpclient.WithHeader("X-Api-Key", s.apiKey),
	); err != nil {
		return model.Quote{}, err
	}

	if len(quotes) == 0 {
		return model.Quote{}, fmt.Errorf("API Ninjas quotes returned empty response")
	}

	return model.Quote{
		Text:   quotes[0].Quote,
		Author: quotes[0].Author,
	}, nil
}

type apiNinjasQuote struct {
	Quote      string   `json:"quote"`
	Author     string   `json:"author"`
	Work       string   `json:"work"`
	Categories []string `json:"categories"`
}
