package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeBillStore is an in-memory BillStore for handler tests.
type fakeBillStore struct {
	bills         map[string]model.Bill
	getErr        error
	listErr       error
	listActiveErr error
	createResult  model.Bill
	createErr     error
	updateRows    int64
	updateErr     error
	deleteRows    int64
	deleteErr     error
}

func (f *fakeBillStore) Get(_ context.Context, id, _ string) (model.Bill, error) {
	if f.getErr != nil {
		return model.Bill{}, f.getErr
	}
	b, ok := f.bills[id]
	if !ok {
		return model.Bill{}, fmt.Errorf("not found")
	}
	return b, nil
}

func (f *fakeBillStore) List(_ context.Context, _ string, limit, offset int) ([]model.Bill, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	all := make([]model.Bill, 0, len(f.bills))
	for _, b := range f.bills {
		all = append(all, b)
	}
	total := len(all)
	if offset >= total {
		return []model.Bill{}, total, nil
	}
	end := offset + limit
	if limit == 0 || end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (f *fakeBillStore) Create(_ context.Context, _ model.BillCreate) (model.Bill, error) {
	return f.createResult, f.createErr
}

func (f *fakeBillStore) Update(_ context.Context, _, _ string, _ model.BillUpdate) (int64, error) {
	return f.updateRows, f.updateErr
}

func (f *fakeBillStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return f.deleteRows, f.deleteErr
}

func (f *fakeBillStore) ListActive(_ context.Context, _ string) ([]model.Bill, error) {
	if f.listActiveErr != nil {
		return nil, f.listActiveErr
	}
	return []model.Bill{}, nil
}

// fakeBillPaymentStore is an in-memory BillPaymentStore for handler tests.
type fakeBillPaymentStore struct {
	createResult    model.BillPayment
	createErr       error
	deleteRows      int64
	deleteErr       error
	listForMonthErr error
	listForYearErr  error
}

func (f *fakeBillPaymentStore) Create(_ context.Context, _ model.BillPaymentCreate) (model.BillPayment, error) {
	return f.createResult, f.createErr
}

func (f *fakeBillPaymentStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return f.deleteRows, f.deleteErr
}

func (f *fakeBillPaymentStore) ListForYear(_ context.Context, _ string, _ int) ([]model.BillPayment, error) {
	return nil, f.listForYearErr
}

func (f *fakeBillPaymentStore) ListForMonth(_ context.Context, _ string, _, _ int) ([]model.BillPayment, error) {
	return nil, f.listForMonthErr
}

// newTestBillsHandler builds a BillsHandler wired to the given stores.
func newTestBillsHandler(billStore service.BillStore, paymentStore service.BillPaymentStore) *BillsHandler {
	svc := service.NewBillsService(billStore, paymentStore)
	return NewBillsHandler(svc, validator.New())
}

func TestListBills(t *testing.T) {
	allBills := map[string]model.Bill{
		"b1": {ID: "b1", Name: "Netflix", CategoryID: "subscriptions", RecurrenceType: "monthly"},
		"b2": {ID: "b2", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly"},
		"b3": {ID: "b3", Name: "Electric", CategoryID: "utilities", RecurrenceType: "monthly"},
	}

	tests := []struct {
		name       string
		query      string
		noSession  bool
		listErr    error
		wantStatus int
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
			name:       "no params uses default pagination",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  defaultBillLimit,
			wantOffset: 0,
		},
		{
			name:       "limit=2 returns first page",
			query:      "?limit=2&offset=0",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  2,
			wantOffset: 0,
		},
		{
			name:       "limit exceeding max is clamped to maxBillLimit",
			query:      "?limit=9999",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  maxBillLimit,
			wantOffset: 0,
		},
		{
			name:       "service error returns 500",
			listErr:    fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeBillStore{bills: allBills, listErr: tt.listErr}
			h := newTestBillsHandler(store, &fakeBillPaymentStore{})

			req := httptest.NewRequest(http.MethodGet, "/api/bills"+tt.query, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp BillListResponse
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
		})
	}
}

func TestListDue(t *testing.T) {
	tests := []struct {
		name            string
		noSession       bool
		listActiveErr   error
		listForMonthErr error
		wantStatus      int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "authenticated returns 200",
			wantStatus: http.StatusOK,
		},
		{
			name:          "list active error returns 500",
			listActiveErr: fmt.Errorf("db error"),
			wantStatus:    http.StatusInternalServerError,
		},
		{
			name:            "payment store error returns 500",
			listForMonthErr: fmt.Errorf("db error"),
			wantStatus:      http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(
				&fakeBillStore{bills: map[string]model.Bill{}, listActiveErr: tt.listActiveErr},
				&fakeBillPaymentStore{listForMonthErr: tt.listForMonthErr},
			)

			req := httptest.NewRequest(http.MethodGet, "/api/bills/due", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.ListDue(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp BillsDueResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
		})
	}
}

func TestListDueYear(t *testing.T) {
	tests := []struct {
		name           string
		noSession      bool
		listActiveErr  error
		listForYearErr error
		wantStatus     int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "authenticated returns 200",
			wantStatus: http.StatusOK,
		},
		{
			name:          "list active error returns 500",
			listActiveErr: fmt.Errorf("db error"),
			wantStatus:    http.StatusInternalServerError,
		},
		{
			name:           "payment store error returns 500",
			listForYearErr: fmt.Errorf("db error"),
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(
				&fakeBillStore{bills: map[string]model.Bill{}, listActiveErr: tt.listActiveErr},
				&fakeBillPaymentStore{listForYearErr: tt.listForYearErr},
			)

			req := httptest.NewRequest(http.MethodGet, "/api/bills/due/year", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.ListDueYear(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp BillsYearResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Months) != 12 {
				t.Errorf("months = %d, want 12", len(resp.Months))
			}
		})
	}
}

func TestCreateBill(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		noSession  bool
		createErr  error
		wantStatus int
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
			name:       "missing name returns 400",
			body:       `{"categoryId":"subscriptions","recurrenceType":"monthly"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid categoryId returns 400",
			body:       `{"name":"Netflix","categoryId":"invalid","recurrenceType":"monthly"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid body returns 201",
			body:       `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "service error returns 500",
			body:       `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			createErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dueDay := 1
			store := &fakeBillStore{
				createResult: model.Bill{
					ID:             "bill-1",
					Name:           "Netflix",
					CategoryID:     "subscriptions",
					RecurrenceType: "monthly",
					DueDay:         &dueDay,
				},
				createErr: tt.createErr,
			}
			h := newTestBillsHandler(store, &fakeBillPaymentStore{})

			req := httptest.NewRequest(http.MethodPost, "/api/bills", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Create(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}

			var resp BillResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ID == "" {
				t.Error("expected non-empty bill ID in response")
			}
		})
	}
}

func TestMarkPaid(t *testing.T) {
	existingBill := model.Bill{
		ID:             "bill-1",
		UserID:         "user1",
		Name:           "Netflix",
		CategoryID:     "subscriptions",
		RecurrenceType: "monthly",
	}
	successPayment := model.BillPayment{
		ID:              "pay-1",
		BillID:          "bill-1",
		UserID:          "user1",
		ComputedDueDate: "2026-04-01",
		PaidDate:        "2026-04-01",
		CreatedAt:       "2026-04-01T00:00:00Z",
	}

	tests := []struct {
		name         string
		billID       string
		body         string
		noSession    bool
		billStore    *fakeBillStore
		paymentStore *fakeBillPaymentStore
		wantStatus   int
	}{
		{
			name:         "no session returns 401",
			billID:       "bill-1",
			body:         `{"computedDueDate":"2026-04-01","paidDate":"2026-04-01"}`,
			noSession:    true,
			billStore:    &fakeBillStore{},
			paymentStore: &fakeBillPaymentStore{},
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "empty body returns 400",
			billID:       "bill-1",
			body:         "",
			billStore:    &fakeBillStore{},
			paymentStore: &fakeBillPaymentStore{},
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:   "bill not found returns 404",
			billID: "bill-missing",
			body:   `{"computedDueDate":"2026-04-01","paidDate":"2026-04-01"}`,
			billStore: &fakeBillStore{
				getErr: sql.ErrNoRows,
			},
			paymentStore: &fakeBillPaymentStore{},
			wantStatus:   http.StatusNotFound,
		},
		{
			name:   "already paid returns 409",
			billID: "bill-1",
			body:   `{"computedDueDate":"2026-04-01","paidDate":"2026-04-01"}`,
			billStore: &fakeBillStore{
				bills: map[string]model.Bill{"bill-1": existingBill},
			},
			paymentStore: &fakeBillPaymentStore{
				createErr: fmt.Errorf("UNIQUE constraint failed: bill_payments_unique"),
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:   "success returns 201 with payment",
			billID: "bill-1",
			body:   `{"computedDueDate":"2026-04-01","paidDate":"2026-04-01"}`,
			billStore: &fakeBillStore{
				bills: map[string]model.Bill{"bill-1": existingBill},
			},
			paymentStore: &fakeBillPaymentStore{
				createResult: successPayment,
			},
			wantStatus: http.StatusCreated,
		},
		{
			// ErrBillValidation from service (invalid date format) → 400
			name:   "invalid date format returns 400",
			billID: "bill-1",
			body:   `{"computedDueDate":"not-a-date","paidDate":"2026-04-01"}`,
			billStore: &fakeBillStore{
				bills: map[string]model.Bill{"bill-1": existingBill},
			},
			paymentStore: &fakeBillPaymentStore{},
			wantStatus:   http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(tt.billStore, tt.paymentStore)

			req := httptest.NewRequest(http.MethodPost, "/api/bills/"+tt.billID+"/pay", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.billID)
			w := httptest.NewRecorder()
			h.MarkPaid(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}

			var resp BillPaymentResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ID == "" {
				t.Error("expected non-empty payment ID in response")
			}
			if resp.BillID != tt.billID {
				t.Errorf("billId = %q, want %q", resp.BillID, tt.billID)
			}
		})
	}
}

func TestUnmarkPaid(t *testing.T) {
	tests := []struct {
		name         string
		paymentID    string
		noSession    bool
		paymentStore *fakeBillPaymentStore
		wantStatus   int
	}{
		{
			name:         "no session returns 401",
			paymentID:    "pay-1",
			noSession:    true,
			paymentStore: &fakeBillPaymentStore{},
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:      "payment not found returns 404",
			paymentID: "pay-missing",
			paymentStore: &fakeBillPaymentStore{
				deleteRows: 0,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:      "success returns 204",
			paymentID: "pay-1",
			paymentStore: &fakeBillPaymentStore{
				deleteRows: 1,
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:      "service error returns 500",
			paymentID: "pay-1",
			paymentStore: &fakeBillPaymentStore{
				deleteErr: fmt.Errorf("db error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(&fakeBillStore{}, tt.paymentStore)

			req := httptest.NewRequest(http.MethodDelete, "/api/bills/payments/"+tt.paymentID, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "paymentId", tt.paymentID)
			w := httptest.NewRecorder()
			h.UnmarkPaid(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestUpdateBill(t *testing.T) {
	dueDay := 1
	existingBill := model.Bill{
		ID:             "bill-1",
		UserID:         "user1",
		Name:           "Netflix",
		CategoryID:     "subscriptions",
		RecurrenceType: "monthly",
		DueDay:         &dueDay,
	}

	tests := []struct {
		name       string
		billID     string
		body       string
		noSession  bool
		store      *fakeBillStore
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			billID:     "bill-1",
			body:       `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			noSession:  true,
			store:      &fakeBillStore{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			billID:     "bill-1",
			body:       "",
			store:      &fakeBillStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "bill not found returns 404",
			billID: "bill-missing",
			body:   `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			store: &fakeBillStore{
				updateRows: 0,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "success returns 200 with updated bill",
			billID: "bill-1",
			body:   `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			store: &fakeBillStore{
				updateRows: 1,
				bills:      map[string]model.Bill{"bill-1": existingBill},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "service error returns 500",
			billID: "bill-1",
			body:   `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"monthly","dueDay":1}`,
			store: &fakeBillStore{
				updateErr: fmt.Errorf("db error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			// ErrBillValidation from service → 400
			name:       "invalid recurrence type returns 400",
			billID:     "bill-1",
			body:       `{"name":"Netflix","categoryId":"subscriptions","recurrenceType":"invalid","dueDay":1}`,
			store:      &fakeBillStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(tt.store, &fakeBillPaymentStore{})

			req := httptest.NewRequest(http.MethodPatch, "/api/bills/"+tt.billID, bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.billID)
			w := httptest.NewRecorder()
			h.Update(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp BillResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ID != tt.billID {
				t.Errorf("id = %q, want %q", resp.ID, tt.billID)
			}
		})
	}
}

func TestDeleteBill(t *testing.T) {
	tests := []struct {
		name       string
		billID     string
		noSession  bool
		store      *fakeBillStore
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			billID:     "bill-1",
			noSession:  true,
			store:      &fakeBillStore{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "bill not found returns 404",
			billID: "bill-missing",
			store: &fakeBillStore{
				deleteRows: 0,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "success returns 204",
			billID: "bill-1",
			store: &fakeBillStore{
				deleteRows: 1,
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "service error returns 500",
			billID: "bill-1",
			store: &fakeBillStore{
				deleteErr: fmt.Errorf("db error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestBillsHandler(tt.store, &fakeBillPaymentStore{})

			req := httptest.NewRequest(http.MethodDelete, "/api/bills/"+tt.billID, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.billID)
			w := httptest.NewRecorder()
			h.Delete(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
