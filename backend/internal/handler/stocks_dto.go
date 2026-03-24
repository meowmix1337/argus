package handler

// AddSymbolRequest is the JSON body for POST /api/stocks/watchlist.
type AddSymbolRequest struct {
	Symbol string `json:"symbol" validate:"required,min=1,max=20"`
}

// WatchlistResponse is the JSON response for watchlist operations.
type WatchlistResponse struct {
	Symbols []string `json:"symbols"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// SearchResponse is the JSON response for GET /api/stocks/search.
type SearchResponse struct {
	Results []SymbolSearchResultDTO `json:"results"`
}

// SymbolSearchResultDTO is a single search result for the frontend.
type SymbolSearchResultDTO struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Type        string `json:"type"`
}
