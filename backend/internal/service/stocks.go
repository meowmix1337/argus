package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/httpclient"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// ErrSymbolNotFound is returned when a watchlist symbol does not exist.
var ErrSymbolNotFound = apperrors.ErrSymbolNotFound

// WatchlistStore defines the data-access contract for the stocks watchlist.
type WatchlistStore interface {
	// ListSymbols returns a page of active symbols plus the total count.
	// limit=0 means no limit (returns all symbols).
	ListSymbols(ctx context.Context, userID string, limit, offset int) ([]string, int, error)
	Exists(ctx context.Context, userID string, symbol string) (bool, error)
	Add(ctx context.Context, userID string, symbol string) error
	Remove(ctx context.Context, userID string, symbol string) error
}

// StocksService fetches stock quotes from Finnhub and crypto from CoinGecko.
type StocksService struct {
	httpClient httpclient.HTTPClient
	apiKey     string
	cache      *CacheService
	store      WatchlistStore
}

// NewStocksService creates a new StocksService backed by a watchlist store.
func NewStocksService(httpClient httpclient.HTTPClient, apiKey string, cache *CacheService, store WatchlistStore) *StocksService {
	return &StocksService{
		httpClient: httpClient,
		apiKey:     apiKey,
		cache:      cache,
		store:      store,
	}
}

const stocksCacheTTL = 10 * time.Second

// Fetch retrieves stock quotes for the current watchlist.
func (s *StocksService) Fetch(ctx context.Context, userID string) ([]model.StockQuote, error) {
	cacheKey := "stocks:" + userID
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]model.StockQuote), nil
	}

	if s.apiKey == "" {
		return nil, fmt.Errorf("FINNHUB_API_KEY not configured")
	}

	quotes, err := s.fetchFromAPIs(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.cache.Set(cacheKey, quotes, stocksCacheTTL)
	return quotes, nil
}

// GetSymbols returns all watchlist symbols for a user (no pagination — used internally for quote fetching).
func (s *StocksService) GetSymbols(ctx context.Context, userID string) ([]string, error) {
	symbols, _, err := s.store.ListSymbols(ctx, userID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("get symbols: %w", err)
	}
	return symbols, nil
}

// GetSymbolsPaginated returns a page of watchlist symbols plus the total count.
func (s *StocksService) GetSymbolsPaginated(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	symbols, total, err := s.store.ListSymbols(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get symbols: %w", err)
	}
	return symbols, total, nil
}

// AddSymbol adds a symbol to the user's watchlist (UPSERT — re-activates soft-deleted rows).
func (s *StocksService) AddSymbol(ctx context.Context, userID string, sym string) error {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	if sym == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	if err := s.store.Add(ctx, userID, sym); err != nil {
		return err
	}
	s.cache.Delete("stocks:" + userID)
	return nil
}

// RemoveSymbol removes a symbol from the user's watchlist (soft-delete).
func (s *StocksService) RemoveSymbol(ctx context.Context, userID string, sym string) error {
	sym = strings.ToUpper(strings.TrimSpace(sym))

	// Verify the symbol exists in the watchlist.
	exists, err := s.store.Exists(ctx, userID, sym)
	if err != nil {
		return fmt.Errorf("check symbol: %w", err)
	}
	if !exists {
		return ErrSymbolNotFound
	}

	if err := s.store.Remove(ctx, userID, sym); err != nil {
		return fmt.Errorf("remove symbol: %w", err)
	}
	s.cache.Delete("stocks:" + userID)
	return nil
}

// SearchSymbols queries Finnhub for symbols matching the given query string.
func (s *StocksService) SearchSymbols(ctx context.Context, query string) ([]model.SymbolSearchResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("FINNHUB_API_KEY not configured")
	}

	u := fmt.Sprintf("https://finnhub.io/api/v1/search?q=%s",
		url.QueryEscape(query))

	var result finnhubSearchResponse
	if err := s.httpClient.Get(ctx, u, &result, httpclient.WithHeader("X-Finnhub-Token", s.apiKey)); err != nil {
		return nil, err
	}

	const maxSymbolSearchResults = 10
	max := len(result.Result)
	if max > maxSymbolSearchResults {
		max = maxSymbolSearchResults
	}
	out := make([]model.SymbolSearchResult, 0, max)
	for _, item := range result.Result[:max] {
		out = append(out, model.SymbolSearchResult{
			Symbol:      item.Symbol,
			Description: item.Description,
			Type:        item.Type,
		})
	}
	return out, nil
}

func (s *StocksService) fetchFromAPIs(ctx context.Context, userID string) ([]model.StockQuote, error) {
	symbols, err := s.GetSymbols(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := make([]model.StockQuote, len(symbols))
	g, gctx := errgroup.WithContext(ctx)

	for i, sym := range symbols {
		i, sym := i, sym
		g.Go(func() error {
			var q model.StockQuote
			var err error
			if sym == "BTC" {
				q, err = s.fetchBTC(gctx)
				if err != nil {
					return nil // BTC failure is non-fatal; slot stays zero value
				}
			} else {
				q, err = s.fetchFinnhub(gctx, sym)
				if err != nil {
					return err
				}
			}
			results[i] = q
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Filter out zero-value entries (e.g. BTC slot if fetch failed)
	filtered := make([]model.StockQuote, 0, len(results))
	for _, q := range results {
		if q.Symbol != "" {
			filtered = append(filtered, q)
		}
	}
	return filtered, nil
}

func (s *StocksService) fetchFinnhub(ctx context.Context, symbol string) (model.StockQuote, error) {
	u := fmt.Sprintf("https://finnhub.io/api/v1/quote?symbol=%s", url.QueryEscape(symbol))

	var q finnhubQuote
	if err := s.httpClient.Get(ctx, u, &q, httpclient.WithHeader("X-Finnhub-Token", s.apiKey)); err != nil {
		return model.StockQuote{}, err
	}

	return model.StockQuote{
		Symbol: symbol,
		Price:  q.C,
		Change: q.D,
		Pct:    q.DP,
	}, nil
}

func (s *StocksService) fetchBTC(ctx context.Context) (model.StockQuote, error) {
	u := "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd&include_24hr_change=true"

	var result map[string]map[string]float64
	if err := s.httpClient.Get(ctx, u, &result); err != nil {
		return model.StockQuote{}, err
	}

	btc, ok := result["bitcoin"]
	if !ok {
		return model.StockQuote{}, fmt.Errorf("bitcoin not in response")
	}

	price := btc["usd"]
	pct := btc["usd_24h_change"]

	return model.StockQuote{
		Symbol: "BTC",
		Price:  price,
		Change: price * pct / 100,
		Pct:    pct,
	}, nil
}

// Finnhub API types
type finnhubQuote struct {
	C  float64 `json:"c"`  // current price
	D  float64 `json:"d"`  // change
	DP float64 `json:"dp"` // percent change
}

type finnhubSearchResponse struct {
	Count  int                 `json:"count"`
	Result []finnhubSearchItem `json:"result"`
}

type finnhubSearchItem struct {
	Description string `json:"description"`
	Symbol      string `json:"symbol"`
	Type        string `json:"type"`
}
