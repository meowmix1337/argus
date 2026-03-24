package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/service"
	"github.com/meowmix1337/argus/backend/internal/session"
)

// fakeWatchlistStore is an in-memory WatchlistStore for handler tests.
type fakeWatchlistStore struct {
	symbols map[string][]string
	err     error
}

func (f *fakeWatchlistStore) ListSymbols(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
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
	return false, f.err
}

func (f *fakeWatchlistStore) Add(_ context.Context, _, _ string) error { return f.err }

func (f *fakeWatchlistStore) Remove(_ context.Context, _, _ string) error { return f.err }

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
