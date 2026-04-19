package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
)

func TestStocksService_Fetch_CacheHit(t *testing.T) {
	cache := platformcache.NewCacheService()
	cached := []model.StockQuote{{Symbol: "AAPL", Price: 100.0}}
	cache.Set("stocks:user1", cached, time.Minute)

	svc := NewStocksService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP must not be called on cache hit")},
		"key", cache, newFakeWatchlistStore(),
	)

	quotes, err := svc.Fetch(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 1 || quotes[0].Symbol != "AAPL" {
		t.Errorf("expected cached quote AAPL, got: %+v", quotes)
	}
}

func TestStocksService_Fetch_NoAPIKey(t *testing.T) {
	svc := NewStocksService(&fakeHTTPClient{}, "", platformcache.NewCacheService(), newFakeWatchlistStore("AAPL"))
	_, err := svc.Fetch(context.Background(), "user1")
	if err == nil {
		t.Error("expected error when API key is empty")
	}
}

func TestStocksService_Fetch_SingleEquity_Success(t *testing.T) {
	// Watchlist has one equity; the fakeHTTPClient returns a Finnhub quote.
	store := newFakeWatchlistStore("AAPL")
	quote := finnhubQuote{C: 185.5, D: 2.3, DP: 1.25}
	svc := NewStocksService(&fakeHTTPClient{responseBody: quote}, "test-key", platformcache.NewCacheService(), store)

	quotes, err := svc.Fetch(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 1 {
		t.Fatalf("expected 1 quote, got %d", len(quotes))
	}
	if quotes[0].Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", quotes[0].Symbol)
	}
	if quotes[0].Price != 185.5 {
		t.Errorf("Price = %v, want 185.5", quotes[0].Price)
	}
}

func TestStocksService_Fetch_BTC_Success(t *testing.T) {
	// Watchlist has BTC; the fakeHTTPClient returns a CoinGecko response.
	store := newFakeWatchlistStore("BTC")
	btcResp := map[string]map[string]float64{
		"bitcoin": {"usd": 62000.0, "usd_24h_change": 3.5},
	}
	svc := NewStocksService(&fakeHTTPClient{responseBody: btcResp}, "test-key", platformcache.NewCacheService(), store)

	quotes, err := svc.Fetch(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 1 || quotes[0].Symbol != "BTC" {
		t.Errorf("expected BTC quote, got: %+v", quotes)
	}
	if quotes[0].Price != 62000.0 {
		t.Errorf("Price = %v, want 62000", quotes[0].Price)
	}
}

func TestStocksService_Fetch_EmptyWatchlist_ReturnsEmptySlice(t *testing.T) {
	svc := NewStocksService(&fakeHTTPClient{}, "test-key", platformcache.NewCacheService(), newFakeWatchlistStore())
	quotes, err := svc.Fetch(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 0 {
		t.Errorf("expected empty quotes for empty watchlist, got %d", len(quotes))
	}
}

func TestStocksService_Fetch_PopulatesCache(t *testing.T) {
	store := newFakeWatchlistStore("AAPL")
	quote := finnhubQuote{C: 150.0}
	cache := platformcache.NewCacheService()
	svc := NewStocksService(&fakeHTTPClient{responseBody: quote}, "test-key", cache, store)

	if _, err := svc.Fetch(context.Background(), "user1"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, ok := cache.Get("stocks:user1"); !ok {
		t.Error("expected result to be cached after successful fetch")
	}
}

func TestStocksService_GetSymbols_ReturnsAll(t *testing.T) {
	store := newFakeWatchlistStore("AAPL", "MSFT", "GOOG")
	svc := NewStocksService(&fakeHTTPClient{}, "key", platformcache.NewCacheService(), store)

	syms, err := svc.GetSymbols(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}
	if len(syms) != 3 {
		t.Errorf("expected 3 symbols, got %d", len(syms))
	}
}

func TestStocksService_GetSymbolsPaginated_RespectsLimit(t *testing.T) {
	store := newFakeWatchlistStore("A", "B", "C", "D", "E")
	svc := NewStocksService(&fakeHTTPClient{}, "key", platformcache.NewCacheService(), store)

	syms, total, err := svc.GetSymbolsPaginated(context.Background(), "user1", 2, 0)
	if err != nil {
		t.Fatalf("GetSymbolsPaginated: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(syms) != 2 {
		t.Errorf("len(syms) = %d, want 2 (limit)", len(syms))
	}
}
