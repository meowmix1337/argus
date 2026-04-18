package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

// fakeWatchlistStore is an in-memory WatchlistStore for service tests.
type fakeWatchlistStore struct {
	symbols map[string]bool // symbol -> active
	addErr  error
	remErr  error
	listErr error
}

func newFakeWatchlistStore(symbols ...string) *fakeWatchlistStore {
	s := &fakeWatchlistStore{symbols: make(map[string]bool)}
	for _, sym := range symbols {
		s.symbols[sym] = true
	}
	return s
}

func (f *fakeWatchlistStore) ListSymbols(_ context.Context, _ string, limit, offset int) ([]string, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	out := make([]string, 0, len(f.symbols))
	for sym, active := range f.symbols {
		if active {
			out = append(out, sym)
		}
	}
	sort.Strings(out)
	total := len(out)
	if limit > 0 {
		if offset >= total {
			return []string{}, total, nil
		}
		out = out[offset:min(offset+limit, total)]
	}
	return out, total, nil
}

func (f *fakeWatchlistStore) Exists(_ context.Context, _ string, sym string) (bool, error) {
	return f.symbols[sym], nil
}

func (f *fakeWatchlistStore) Add(_ context.Context, _ string, sym string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.symbols[sym] = true
	return nil
}

func (f *fakeWatchlistStore) Remove(_ context.Context, _ string, sym string) error {
	if f.remErr != nil {
		return f.remErr
	}
	delete(f.symbols, sym)
	return nil
}

// fakeHTTPClient is a minimal HTTPClient that marshals responseBody into the result parameter.
// Set rawBytes to return raw bytes from GetBytes (e.g. ICS content for calendar tests).
type fakeHTTPClient struct {
	responseBody any
	rawBytes     []byte
	err          error
}

func (f *fakeHTTPClient) Get(_ context.Context, _ string, result any, _ ...httpclient.RequestOption) error {
	if f.err != nil {
		return f.err
	}
	b, err := json.Marshal(f.responseBody)
	if err != nil {
		return fmt.Errorf("fakeHTTPClient: marshal: %w", err)
	}
	return json.Unmarshal(b, result)
}

func (f *fakeHTTPClient) Post(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Put(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Delete(_ context.Context, _ string, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) GetBytes(_ context.Context, _ string, _ ...httpclient.RequestOption) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.rawBytes != nil {
		return f.rawBytes, nil
	}
	return json.Marshal(f.responseBody)
}

func newTestStocksService(store *fakeWatchlistStore) *StocksService {
	return NewStocksService(&fakeHTTPClient{}, "test-key", platformcache.NewCacheService(), store)
}

func TestAddSymbol_NormalizesToUppercase(t *testing.T) {
	store := newFakeWatchlistStore()
	svc := newTestStocksService(store)

	if err := svc.AddSymbol(context.Background(), "user1", "tsla"); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	if !store.symbols["TSLA"] {
		t.Error("expected TSLA in store (uppercase) after adding lowercase 'tsla'")
	}
}

func TestAddSymbol_TrimsWhitespace(t *testing.T) {
	store := newFakeWatchlistStore()
	svc := newTestStocksService(store)

	if err := svc.AddSymbol(context.Background(), "user1", "  AAPL  "); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	if !store.symbols["AAPL"] {
		t.Error("expected AAPL in store after adding whitespace-padded symbol")
	}
}

func TestAddSymbol_EmptySymbol_ReturnsError(t *testing.T) {
	store := newFakeWatchlistStore()
	svc := newTestStocksService(store)

	if err := svc.AddSymbol(context.Background(), "user1", "  "); err == nil {
		t.Error("expected error for empty symbol, got nil")
	}
}

func TestAddSymbol_InvalidatesCacheOnSuccess(t *testing.T) {
	store := newFakeWatchlistStore()
	cache := platformcache.NewCacheService()
	svc := NewStocksService(&fakeHTTPClient{}, "test-key", cache, store)

	// Pre-populate cache
	cache.Set("stocks:user1", []any{}, time.Minute)

	if err := svc.AddSymbol(context.Background(), "user1", "TSLA"); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	_, ok := cache.Get("stocks:user1")
	if ok {
		t.Error("expected cache to be invalidated after AddSymbol")
	}
}

func TestRemoveSymbol_NotFound_ReturnsErrSymbolNotFound(t *testing.T) {
	store := newFakeWatchlistStore()
	svc := newTestStocksService(store)

	err := svc.RemoveSymbol(context.Background(), "user1", "TSLA")
	if err != ErrSymbolNotFound {
		t.Errorf("expected ErrSymbolNotFound, got %v", err)
	}
}

func TestRemoveSymbol_Found_RemovesFromStore(t *testing.T) {
	store := newFakeWatchlistStore("TSLA")
	svc := newTestStocksService(store)

	if err := svc.RemoveSymbol(context.Background(), "user1", "TSLA"); err != nil {
		t.Fatalf("RemoveSymbol: %v", err)
	}
	if store.symbols["TSLA"] {
		t.Error("expected TSLA removed from store")
	}
}

func TestRemoveSymbol_InvalidatesCacheOnSuccess(t *testing.T) {
	store := newFakeWatchlistStore("TSLA")
	cache := platformcache.NewCacheService()
	svc := NewStocksService(&fakeHTTPClient{}, "test-key", cache, store)

	cache.Set("stocks:user1", []any{}, time.Minute)

	if err := svc.RemoveSymbol(context.Background(), "user1", "TSLA"); err != nil {
		t.Fatalf("RemoveSymbol: %v", err)
	}
	_, ok := cache.Get("stocks:user1")
	if ok {
		t.Error("expected cache to be invalidated after RemoveSymbol")
	}
}

func TestSearchSymbols_CapsResultsAtTen(t *testing.T) {
	type item struct {
		Description string `json:"description"`
		Symbol      string `json:"symbol"`
		Type        string `json:"type"`
	}
	items := make([]item, 15)
	for i := range items {
		items[i] = item{Symbol: fmt.Sprintf("SYM%d", i), Description: "Test Co", Type: "Common Stock"}
	}
	fakeResp := map[string]any{"count": 15, "result": items}

	svc := NewStocksService(
		&fakeHTTPClient{responseBody: fakeResp},
		"test-key",
		platformcache.NewCacheService(),
		newFakeWatchlistStore(),
	)

	results, err := svc.SearchSymbols(context.Background(), "SYM")
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 results (capped at max), got %d", len(results))
	}
}

func TestSearchSymbols_FewerThanTen(t *testing.T) {
	type item struct {
		Description string `json:"description"`
		Symbol      string `json:"symbol"`
		Type        string `json:"type"`
	}
	items := []item{{Symbol: "AAPL", Description: "Apple Inc", Type: "Common Stock"}}
	fakeResp := map[string]any{"count": 1, "result": items}

	svc := NewStocksService(
		&fakeHTTPClient{responseBody: fakeResp},
		"test-key",
		platformcache.NewCacheService(),
		newFakeWatchlistStore(),
	)

	results, err := svc.SearchSymbols(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %q", results[0].Symbol)
	}
}

func TestSearchSymbols_NoAPIKey_ReturnsError(t *testing.T) {
	svc := NewStocksService(&fakeHTTPClient{}, "", platformcache.NewCacheService(), newFakeWatchlistStore())

	_, err := svc.SearchSymbols(context.Background(), "TSLA")
	if err == nil {
		t.Error("expected error when API key is empty")
	}
}

func TestGetSymbols_StoreError_Propagates(t *testing.T) {
	store := newFakeWatchlistStore()
	store.listErr = fmt.Errorf("db failure")
	svc := newTestStocksService(store)
	if _, err := svc.GetSymbols(context.Background(), "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestGetSymbolsPaginated_StoreError_Propagates(t *testing.T) {
	store := newFakeWatchlistStore()
	store.listErr = fmt.Errorf("db failure")
	svc := newTestStocksService(store)
	if _, _, err := svc.GetSymbolsPaginated(context.Background(), "user1", 10, 0); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}
