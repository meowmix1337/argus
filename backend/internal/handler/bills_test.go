package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/handler"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
	"github.com/meowmix1337/argus/backend/internal/session"
	"github.com/meowmix1337/argus/backend/internal/validate"
)

// fakeBillStore is an in-memory BillStore for handler tests.
type fakeBillStore struct {
	bills  []model.Bill
	nextID int
}

func newFakeBillStore() *fakeBillStore {
	return &fakeBillStore{nextID: 1}
}

func (f *fakeBillStore) List(_ context.Context, userID string, limit, offset int) ([]model.Bill, int, error) {
	var result []model.Bill
	for _, b := range f.bills {
		if b.UserID == userID {
			result = append(result, b)
		}
	}
	total := len(result)
	if offset >= total {
		return []model.Bill{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return result[offset:end], total, nil
}

func (f *fakeBillStore) Get(_ context.Context, id string, userID string) (model.Bill, error) {
	for _, b := range f.bills {
		if b.ID == id && b.UserID == userID {
			return b, nil
		}
	}
	return model.Bill{}, nil
}

func (f *fakeBillStore) Create(_ context.Context, b model.BillCreate) (model.Bill, error) {
	bill := model.Bill{
		ID:             b.ID,
		UserID:         b.UserID,
		Name:           b.Name,
		Amount:         b.Amount,
		CategoryID:     b.CategoryID,
		RecurrenceType: b.RecurrenceType,
		DueDate:        b.DueDate,
		DueDay:         b.DueDay,
		DueMonth:       b.DueMonth,
		AnchorDate:     b.AnchorDate,
		Notes:          b.Notes,
		CreatedAt:      "2026-01-01T00:00:00.000Z",
		UpdatedAt:      "2026-01-01T00:00:00.000Z",
	}
	f.bills = append(f.bills, bill)
	return bill, nil
}

func (f *fakeBillStore) Update(_ context.Context, id string, userID string, u model.BillUpdate) (int64, error) {
	for i, b := range f.bills {
		if b.ID == id && b.UserID == userID {
			f.bills[i].Name = u.Name
			f.bills[i].Amount = u.Amount
			f.bills[i].CategoryID = u.CategoryID
			f.bills[i].RecurrenceType = u.RecurrenceType
			f.bills[i].DueDate = u.DueDate
			f.bills[i].DueDay = u.DueDay
			f.bills[i].DueMonth = u.DueMonth
			f.bills[i].AnchorDate = u.AnchorDate
			f.bills[i].Notes = u.Notes
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeBillStore) Delete(_ context.Context, id string, userID string) (int64, error) {
	for i, b := range f.bills {
		if b.ID == id && b.UserID == userID {
			f.bills = append(f.bills[:i], f.bills[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeBillStore) ListActive(_ context.Context, userID string) ([]model.Bill, error) {
	var result []model.Bill
	for _, b := range f.bills {
		if b.UserID == userID {
			result = append(result, b)
		}
	}
	return result, nil
}

// sessionContext returns a context with an injected session for userID.
func sessionContext(userID string) context.Context {
	return context.WithValue(context.Background(), middleware.SessionKey, session.Data{UserID: userID})
}

func setupBillsRouter(store service.BillStore) chi.Router {
	svc := service.NewBillsService(store)
	h := handler.NewBillsHandler(svc, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)
	return r
}

func TestBillsHandler_List_HappyPath(t *testing.T) {
	store := newFakeBillStore()
	day := 15
	store.bills = []model.Bill{
		{ID: "bill-1", UserID: "user-1", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &day},
	}

	r := setupBillsRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/bills", nil).WithContext(sessionContext("user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp handler.BillListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Bills) != 1 {
		t.Fatalf("expected 1 bill, got %d", len(resp.Bills))
	}
	if resp.Total != 1 {
		t.Fatalf("expected total=1, got %d", resp.Total)
	}
}

func TestBillsHandler_List_NoSession(t *testing.T) {
	r := setupBillsRouter(newFakeBillStore())
	req := httptest.NewRequest(http.MethodGet, "/api/bills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBillsHandler_Create_HappyPath(t *testing.T) {
	store := newFakeBillStore()
	r := setupBillsRouter(store)

	day := 1
	body := handler.CreateBillRequest{
		Name:           "Netflix",
		CategoryID:     "subscriptions",
		RecurrenceType: "monthly",
		DueDay:         &day,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/bills", bytes.NewReader(b)).WithContext(sessionContext("user-1"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp handler.BillResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "Netflix" {
		t.Fatalf("expected name=Netflix, got %s", resp.Name)
	}
}

func TestBillsHandler_Create_NoSession(t *testing.T) {
	r := setupBillsRouter(newFakeBillStore())
	body := []byte(`{"name":"x","categoryId":"other","recurrenceType":"once","dueDate":"2026-01-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/bills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBillsHandler_Update_HappyPath(t *testing.T) {
	store := newFakeBillStore()
	day := 5
	store.bills = []model.Bill{
		{ID: "bill-1", UserID: "user-1", Name: "Old Name", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &day},
	}

	r := setupBillsRouter(store)
	newDay := 10
	body := handler.UpdateBillRequest{
		Name:           "New Name",
		CategoryID:     "utilities",
		RecurrenceType: "monthly",
		DueDay:         &newDay,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/bills/bill-1", bytes.NewReader(b)).WithContext(sessionContext("user-1"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBillsHandler_Update_NotFound(t *testing.T) {
	r := setupBillsRouter(newFakeBillStore())
	day := 1
	body := handler.UpdateBillRequest{
		Name:           "X",
		CategoryID:     "other",
		RecurrenceType: "monthly",
		DueDay:         &day,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/bills/nonexistent", bytes.NewReader(b)).WithContext(sessionContext("user-1"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBillsHandler_Delete_HappyPath(t *testing.T) {
	store := newFakeBillStore()
	day := 5
	store.bills = []model.Bill{
		{ID: "bill-1", UserID: "user-1", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &day},
	}

	r := setupBillsRouter(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/bills/bill-1", nil).WithContext(sessionContext("user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestBillsHandler_Delete_NoSession(t *testing.T) {
	r := setupBillsRouter(newFakeBillStore())
	req := httptest.NewRequest(http.MethodDelete, "/api/bills/bill-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
