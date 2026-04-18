package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/session"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeWatchlistStore is an in-memory WatchlistStore for handler tests.
type fakeWatchlistStore struct {
	symbols        map[string][]string
	existsResult   bool
	err            error
	listSymbolsErr error
	removeErr      error
}

func (f *fakeWatchlistStore) ListSymbols(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	if f.listSymbolsErr != nil {
		return nil, 0, f.listSymbolsErr
	}
	if f.err != nil {
		return nil, 0, f.err
	}
	all := f.symbols[userID]
	total := len(all)
	if limit == 0 || total == 0 {
		return all, total, nil
	}
	if offset >= total {
		return []string{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (f *fakeWatchlistStore) Exists(_ context.Context, _, _ string) (bool, error) {
	return f.existsResult, f.err
}

func (f *fakeWatchlistStore) Add(_ context.Context, _, _ string) error { return f.err }

func (f *fakeWatchlistStore) Remove(_ context.Context, _, _ string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	return f.err
}

// newTestStocksHandler builds a StocksHandler wired to the given store.
// httpClient and cache are nil — only store-backed methods are exercised in these tests.
func newTestStocksHandler(store service.WatchlistStore) *StocksHandler {
	svc := service.NewStocksService(nil, "", nil, store)
	return NewStocksHandler(svc, validator.New())
}

// withSession injects a session into the request context, simulating RequireAuth middleware.
func withSession(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.SessionKey, session.Data{UserID: userID})
	return r.WithContext(ctx)
}

// withChiParam injects a chi URL parameter into the request context.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestGetWatchlist(t *testing.T) {
	allSymbols := []string{"AAPL", "TSLA", "MSFT", "GOOG", "AMZN"}

	store := &fakeWatchlistStore{
		symbols: map[string][]string{"user1": allSymbols},
	}
	h := newTestStocksHandler(store)

	tests := []struct {
		name       string
		query      string
		noSession  bool
		wantStatus int
		wantLen    int
		wantTotal  int
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "no params uses defaults and returns all symbols",
			wantStatus: http.StatusOK,
			wantLen:    5, // only 5 symbols, less than defaultWatchlistLimit
			wantTotal:  5,
			wantLimit:  defaultWatchlistLimit,
			wantOffset: 0,
		},
		{
			name:       "limit=2 offset=0 returns first page",
			query:      "?limit=2&offset=0",
			wantStatus: http.StatusOK,
			wantLen:    2,
			wantTotal:  5,
			wantLimit:  2,
			wantOffset: 0,
		},
		{
			name:       "limit=2 offset=2 returns second page",
			query:      "?limit=2&offset=2",
			wantStatus: http.StatusOK,
			wantLen:    2,
			wantTotal:  5,
			wantLimit:  2,
			wantOffset: 2,
		},
		{
			name:       "offset past end returns empty symbols with correct total",
			query:      "?limit=5&offset=10",
			wantStatus: http.StatusOK,
			wantLen:    0,
			wantTotal:  5,
			wantLimit:  5,
			wantOffset: 10,
		},
		{
			name:       "limit exceeding max is clamped to maxWatchlistLimit",
			query:      "?limit=999",
			wantStatus: http.StatusOK,
			wantLen:    5,
			wantTotal:  5,
			wantLimit:  maxWatchlistLimit,
			wantOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/stocks/watchlist"+tt.query, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.GetWatchlist(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp WatchlistResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Total != tt.wantTotal {
				t.Errorf("total = %d, want %d", resp.Total, tt.wantTotal)
			}
			if resp.Limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", resp.Limit, tt.wantLimit)
			}
			if resp.Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", resp.Offset, tt.wantOffset)
			}
			if len(resp.Symbols) != tt.wantLen {
				t.Errorf("len(symbols) = %d, want %d", len(resp.Symbols), tt.wantLen)
			}
		})
	}
}

func TestAddSymbol(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		noSession      bool
		storeErr       error
		listSymbolsErr error
		wantStatus     int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty symbol field returns 400",
			body:       `{"symbol":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid symbol returns 201",
			body:       `{"symbol":"TSLA"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "store error returns 400",
			body:       `{"symbol":"TSLA"}`,
			storeErr:   fmt.Errorf("db error"),
			wantStatus: http.StatusBadRequest,
		},
		{
			// Add succeeds but GetSymbolsPaginated fails → 500
			name:           "list after add error returns 500",
			body:           `{"symbol":"TSLA"}`,
			listSymbolsErr: fmt.Errorf("db error"),
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeWatchlistStore{err: tt.storeErr, listSymbolsErr: tt.listSymbolsErr}
			h := newTestStocksHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/stocks/watchlist", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.AddSymbol(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestRemoveSymbol(t *testing.T) {
	tests := []struct {
		name           string
		symbol         string
		noSession      bool
		existsResult   bool
		storeErr       error
		removeErr      error
		listSymbolsErr error
		wantStatus     int
	}{
		{
			name:       "no session returns 401",
			symbol:     "TSLA",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:         "symbol found returns 200",
			symbol:       "TSLA",
			existsResult: true,
			wantStatus:   http.StatusOK,
		},
		{
			name:         "symbol not found returns 404",
			symbol:       "TSLA",
			existsResult: false,
			wantStatus:   http.StatusNotFound,
		},
		{
			// Exists returns (true, nil), Remove returns error → 500
			name:         "remove error returns 500",
			symbol:       "TSLA",
			existsResult: true,
			removeErr:    fmt.Errorf("db error"),
			wantStatus:   http.StatusInternalServerError,
		},
		{
			// Exists returns (true, err) → service wraps non-ErrSymbolNotFound → 500
			name:         "exists store error returns 500",
			symbol:       "TSLA",
			existsResult: true,
			storeErr:     fmt.Errorf("db error"),
			wantStatus:   http.StatusInternalServerError,
		},
		{
			// Remove succeeds but GetSymbolsPaginated fails → 500
			name:           "list after remove error returns 500",
			symbol:         "TSLA",
			existsResult:   true,
			listSymbolsErr: fmt.Errorf("db error"),
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeWatchlistStore{existsResult: tt.existsResult, err: tt.storeErr, removeErr: tt.removeErr, listSymbolsErr: tt.listSymbolsErr}
			h := newTestStocksHandler(store)

			req := httptest.NewRequest(http.MethodDelete, "/api/stocks/watchlist/"+tt.symbol, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "symbol", tt.symbol)
			w := httptest.NewRecorder()
			h.RemoveSymbol(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSearchSymbols(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing q param returns 400",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "q too long returns 400",
			query:      "?q=" + strings.Repeat("A", 51),
			wantStatus: http.StatusBadRequest,
		},
		{
			// apiKey is "" when constructed with newTestStocksHandler → service returns error → 500
			name:       "service error returns 500",
			query:      "?q=AAPL",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeWatchlistStore{}
			h := newTestStocksHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/stocks/search"+tt.query, nil)
			w := httptest.NewRecorder()
			h.SearchSymbols(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetStockQuotes(t *testing.T) {
	tests := []struct {
		name       string
		noSession  bool
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			// apiKey is "" when constructed with newTestStocksHandler → Fetch returns error → 500
			name:       "no API key returns 500",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeWatchlistStore{}
			h := newTestStocksHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/stocks", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Get(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
