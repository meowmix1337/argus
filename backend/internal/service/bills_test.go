package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// strPtr / intPtr return pointers to string/int literals — used for nullable
// bill fields in tests. Defined as typed helpers to avoid generic-wrapper lint warnings.
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// ---- fake stores ----

type fakeBillStore struct {
	bills     map[string]model.Bill
	updateN   int64
	deleteN   int64
	updateErr error
	deleteErr error
	getErr    error
	listErr   error
	createErr error
}

func newFakeBillStore(bills ...model.Bill) *fakeBillStore {
	s := &fakeBillStore{bills: make(map[string]model.Bill)}
	for _, b := range bills {
		s.bills[b.ID] = b
	}
	return s
}

func (f *fakeBillStore) List(_ context.Context, _ string, _, _ int) ([]model.Bill, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	out := make([]model.Bill, 0, len(f.bills))
	for _, b := range f.bills {
		out = append(out, b)
	}
	return out, len(out), nil
}

func (f *fakeBillStore) Get(_ context.Context, id string, _ string) (model.Bill, error) {
	if f.getErr != nil {
		return model.Bill{}, f.getErr
	}
	b, ok := f.bills[id]
	if !ok {
		return model.Bill{}, sql.ErrNoRows
	}
	return b, nil
}

func (f *fakeBillStore) Create(_ context.Context, b model.BillCreate) (model.Bill, error) {
	if f.createErr != nil {
		return model.Bill{}, f.createErr
	}
	bill := model.Bill{ID: b.ID, UserID: b.UserID, Name: b.Name, CategoryID: b.CategoryID, RecurrenceType: b.RecurrenceType}
	f.bills[b.ID] = bill
	return bill, nil
}

func (f *fakeBillStore) Update(_ context.Context, _ string, _ string, _ model.BillUpdate) (int64, error) {
	return f.updateN, f.updateErr
}

func (f *fakeBillStore) Delete(_ context.Context, _ string, _ string) (int64, error) {
	return f.deleteN, f.deleteErr
}

func (f *fakeBillStore) ListActive(_ context.Context, _ string) ([]model.Bill, error) {
	out := make([]model.Bill, 0, len(f.bills))
	for _, b := range f.bills {
		out = append(out, b)
	}
	return out, nil
}

type fakeBillPaymentStore struct {
	payments  []model.BillPayment
	createFn  func(model.BillPaymentCreate) (model.BillPayment, error)
	deleteN   int64
	deleteErr error
}

func (f *fakeBillPaymentStore) Create(_ context.Context, p model.BillPaymentCreate) (model.BillPayment, error) {
	if f.createFn != nil {
		return f.createFn(p)
	}
	payment := model.BillPayment{
		ID:              p.ID,
		BillID:          p.BillID,
		UserID:          p.UserID,
		ComputedDueDate: p.ComputedDueDate,
		PaidDate:        p.PaidDate,
		Note:            p.Note,
	}
	f.payments = append(f.payments, payment)
	return payment, nil
}

func (f *fakeBillPaymentStore) Delete(_ context.Context, _ string, _ string) (int64, error) {
	return f.deleteN, f.deleteErr
}

func (f *fakeBillPaymentStore) ListForYear(_ context.Context, _ string, _ int) ([]model.BillPayment, error) {
	return f.payments, nil
}

func (f *fakeBillPaymentStore) ListForMonth(_ context.Context, _ string, _, _ int) ([]model.BillPayment, error) {
	return f.payments, nil
}

// ---- paymentKey ----

func TestPaymentKey_Format(t *testing.T) {
	if got := paymentKey("bill-abc", "2024-06-15"); got != "bill-abc|2024-06-15" {
		t.Errorf("paymentKey = %q, want %q", got, "bill-abc|2024-06-15")
	}
}

func TestPaymentKey_Deterministic(t *testing.T) {
	k1 := paymentKey("b1", "2024-01-01")
	k2 := paymentKey("b1", "2024-01-01")
	if k1 != k2 {
		t.Error("paymentKey must be deterministic for the same inputs")
	}
}

// ---- buildPaymentMap ----

func TestBuildPaymentMap_Empty(t *testing.T) {
	if m := buildPaymentMap(nil); len(m) != 0 {
		t.Errorf("expected empty map for nil input, got len=%d", len(m))
	}
}

func TestBuildPaymentMap_KeyedByBillAndDueDate(t *testing.T) {
	payments := []model.BillPayment{
		{ID: "p1", BillID: "b1", ComputedDueDate: "2024-06-01"},
		{ID: "p2", BillID: "b2", ComputedDueDate: "2024-06-15"},
	}
	m := buildPaymentMap(payments)
	if p, ok := m["b1|2024-06-01"]; !ok || p.ID != "p1" {
		t.Error("expected payment 'p1' under key 'b1|2024-06-01'")
	}
	if p, ok := m["b2|2024-06-15"]; !ok || p.ID != "p2" {
		t.Error("expected payment 'p2' under key 'b2|2024-06-15'")
	}
}

// ---- applyPayment ----

func TestApplyPayment_Unpaid(t *testing.T) {
	due := &model.BillDue{ID: "b1", ComputedDueDate: "2024-06-01"}
	applyPayment(due, map[string]model.BillPayment{})
	if due.IsPaid || due.PaymentID != nil || due.PaidDate != nil {
		t.Error("expected IsPaid=false and nil fields for an unpaid bill")
	}
}

func TestApplyPayment_Paid(t *testing.T) {
	paidDate := "2024-06-02"
	note := "on time"
	payID := "p1"
	due := &model.BillDue{ID: "b1", ComputedDueDate: "2024-06-01"}
	pmap := map[string]model.BillPayment{
		"b1|2024-06-01": {ID: payID, PaidDate: paidDate, Note: &note},
	}
	applyPayment(due, pmap)

	if !due.IsPaid {
		t.Error("expected IsPaid=true")
	}
	if due.PaidDate == nil || *due.PaidDate != paidDate {
		t.Errorf("PaidDate = %v, want %q", due.PaidDate, paidDate)
	}
	if due.PaidNote == nil || *due.PaidNote != note {
		t.Errorf("PaidNote = %v, want %q", due.PaidNote, note)
	}
	if due.PaymentID == nil || *due.PaymentID != payID {
		t.Errorf("PaymentID = %v, want %q", due.PaymentID, payID)
	}
}

// ---- computeCurrentMonthDueDate ----

func TestComputeCurrentMonthDueDate_Once_MatchingMonth(t *testing.T) {
	b := model.Bill{RecurrenceType: "once", DueDate: strPtr("2024-06-15")}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.June)
	if !ok || date != "2024-06-15" {
		t.Errorf("expected '2024-06-15', got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Once_WrongMonth(t *testing.T) {
	b := model.Bill{RecurrenceType: "once", DueDate: strPtr("2024-06-15")}
	_, ok := computeCurrentMonthDueDate(b, 2024, time.July)
	if ok {
		t.Error("expected no occurrence in July for a June one-time bill")
	}
}

func TestComputeCurrentMonthDueDate_Once_NilDueDate(t *testing.T) {
	b := model.Bill{RecurrenceType: "once"}
	_, ok := computeCurrentMonthDueDate(b, 2024, time.June)
	if ok {
		t.Error("expected no occurrence when DueDate is nil")
	}
}

func TestComputeCurrentMonthDueDate_Monthly_Normal(t *testing.T) {
	b := model.Bill{RecurrenceType: "monthly", DueDay: intPtr(15)}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.March)
	if !ok || date != "2024-03-15" {
		t.Errorf("expected '2024-03-15', got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Monthly_CapsToLastDayOfFebruary(t *testing.T) {
	// DueDay=31 in February 2024 (leap year, 29 days) must clamp to the 29th.
	b := model.Bill{RecurrenceType: "monthly", DueDay: intPtr(31)}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.February)
	if !ok || date != "2024-02-29" {
		t.Errorf("expected '2024-02-29' (capped), got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Monthly_NilDueDay(t *testing.T) {
	b := model.Bill{RecurrenceType: "monthly"}
	_, ok := computeCurrentMonthDueDate(b, 2024, time.March)
	if ok {
		t.Error("expected no occurrence when DueDay is nil")
	}
}

func TestComputeCurrentMonthDueDate_Annual_MatchingMonth(t *testing.T) {
	b := model.Bill{RecurrenceType: "annual", DueDay: intPtr(10), DueMonth: intPtr(3)}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.March)
	if !ok || date != "2024-03-10" {
		t.Errorf("expected '2024-03-10', got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Annual_WrongMonth(t *testing.T) {
	b := model.Bill{RecurrenceType: "annual", DueDay: intPtr(10), DueMonth: intPtr(3)}
	_, ok := computeCurrentMonthDueDate(b, 2024, time.April)
	if ok {
		t.Error("expected no occurrence in April for a March annual bill")
	}
}

func TestComputeCurrentMonthDueDate_Weekly_HasOccurrence(t *testing.T) {
	// Anchor 2024-01-01 (Monday), weekly — January has multiple occurrences.
	b := model.Bill{RecurrenceType: "weekly", AnchorDate: strPtr("2024-01-01")}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.January)
	if !ok || date == "" {
		t.Errorf("expected a weekly occurrence in January 2024, got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Biweekly_HasOccurrence(t *testing.T) {
	b := model.Bill{RecurrenceType: "biweekly", AnchorDate: strPtr("2024-01-01")}
	date, ok := computeCurrentMonthDueDate(b, 2024, time.January)
	if !ok || date == "" {
		t.Errorf("expected a biweekly occurrence in January 2024, got %q ok=%v", date, ok)
	}
}

func TestComputeCurrentMonthDueDate_Quarterly_MatchingMonths(t *testing.T) {
	// Quarterly from 2024-01-15: appears in Jan, Apr, Jul, Oct.
	b := model.Bill{RecurrenceType: "quarterly", AnchorDate: strPtr("2024-01-15")}
	for _, month := range []time.Month{time.January, time.April, time.July, time.October} {
		date, ok := computeCurrentMonthDueDate(b, 2024, month)
		if !ok || date == "" {
			t.Errorf("expected quarterly occurrence in %v, got %q ok=%v", month, date, ok)
		}
	}
}

func TestComputeCurrentMonthDueDate_Quarterly_NonMatchingMonths(t *testing.T) {
	b := model.Bill{RecurrenceType: "quarterly", AnchorDate: strPtr("2024-01-15")}
	for _, month := range []time.Month{time.February, time.March, time.May, time.June} {
		_, ok := computeCurrentMonthDueDate(b, 2024, month)
		if ok {
			t.Errorf("expected no quarterly occurrence in %v (anchor is January)", month)
		}
	}
}

// ---- validateBillFields ----

func TestValidateBillFields_EmptyName(t *testing.T) {
	u := model.BillUpdate{Name: "   ", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for empty name, got %v", err)
	}
}

func TestValidateBillFields_InvalidCategory(t *testing.T) {
	u := model.BillUpdate{Name: "Groceries", CategoryID: "food", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for unknown category, got %v", err)
	}
}

func TestValidateBillFields_InvalidRecurrenceType(t *testing.T) {
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "daily"}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for invalid recurrence type, got %v", err)
	}
}

func TestValidateBillFields_Once_NoDueDate(t *testing.T) {
	u := model.BillUpdate{Name: "One-off", CategoryID: "other", RecurrenceType: "once"}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for missing due_date, got %v", err)
	}
}

func TestValidateBillFields_Once_InvalidDateFormat(t *testing.T) {
	// MM-DD-YYYY is not the expected YYYY-MM-DD format.
	u := model.BillUpdate{Name: "One-off", CategoryID: "other", RecurrenceType: "once", DueDate: strPtr("06-15-2024")}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for bad date format, got %v", err)
	}
}

func TestValidateBillFields_Monthly_NoDueDay(t *testing.T) {
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly"}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for missing due_day, got %v", err)
	}
}

func TestValidateBillFields_Annual_MissingDayAndMonth(t *testing.T) {
	u := model.BillUpdate{Name: "Insurance", CategoryID: "insurance", RecurrenceType: "annual"}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for missing due_day and due_month, got %v", err)
	}
}

func TestValidateBillFields_Weekly_NoAnchorDate(t *testing.T) {
	u := model.BillUpdate{Name: "Loan", CategoryID: "loans", RecurrenceType: "weekly"}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for missing anchor_date, got %v", err)
	}
}

func TestValidateBillFields_Weekly_InvalidAnchorDateFormat(t *testing.T) {
	u := model.BillUpdate{Name: "Loan", CategoryID: "loans", RecurrenceType: "weekly", AnchorDate: strPtr("01/15/2024")}
	if err := validateBillFields(u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for bad anchor_date format, got %v", err)
	}
}

func TestValidateBillFields_ValidMonthly(t *testing.T) {
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if err := validateBillFields(u); err != nil {
		t.Errorf("unexpected error for valid monthly bill: %v", err)
	}
}

func TestValidateBillFields_ValidOnce(t *testing.T) {
	u := model.BillUpdate{Name: "One-off fee", CategoryID: "other", RecurrenceType: "once", DueDate: strPtr("2024-06-15")}
	if err := validateBillFields(u); err != nil {
		t.Errorf("unexpected error for valid once bill: %v", err)
	}
}

// ---- BillsService methods ----

func TestBillsService_Get_NotFound(t *testing.T) {
	// Empty store → Get returns sql.ErrNoRows → service maps it to ErrBillNotFound.
	svc := NewBillsService(newFakeBillStore(), &fakeBillPaymentStore{})
	_, err := svc.Get(context.Background(), "nonexistent", "user1")
	if !errors.Is(err, apperrors.ErrBillNotFound) {
		t.Errorf("expected ErrBillNotFound, got %v", err)
	}
}

func TestBillsService_Delete_NotFound(t *testing.T) {
	// deleteN=0 (default) simulates no rows affected → ErrBillNotFound.
	store := newFakeBillStore()
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if err := svc.Delete(context.Background(), "nonexistent", "user1"); !errors.Is(err, apperrors.ErrBillNotFound) {
		t.Errorf("expected ErrBillNotFound, got %v", err)
	}
}

func TestBillsService_Delete_Success(t *testing.T) {
	store := newFakeBillStore()
	store.deleteN = 1
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if err := svc.Delete(context.Background(), "b1", "user1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestBillsService_MarkPaid_NoteTooLong(t *testing.T) {
	// Validation runs before touching the store, so an existing bill isn't required.
	store := newFakeBillStore(model.Bill{ID: "b1"})
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	longNote := "this note is definitely longer than the 32-character limit"
	_, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "2024-06-01", &longNote)
	if !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for long note, got %v", err)
	}
}

func TestBillsService_MarkPaid_InvalidPaidDate(t *testing.T) {
	store := newFakeBillStore(model.Bill{ID: "b1"})
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	_, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "not-a-date", nil)
	if !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for bad paid_date, got %v", err)
	}
}

func TestBillsService_MarkPaid_InvalidComputedDueDate(t *testing.T) {
	store := newFakeBillStore(model.Bill{ID: "b1"})
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	_, err := svc.MarkPaid(context.Background(), "user1", "b1", "not-a-date", "2024-06-01", nil)
	if !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation for bad computed_due_date, got %v", err)
	}
}

func TestBillsService_Unmark_NotFound(t *testing.T) {
	// deleteN=0 (default) → ErrBillPaymentNotFound.
	svc := NewBillsService(newFakeBillStore(), &fakeBillPaymentStore{})
	if err := svc.Unmark(context.Background(), "user1", "nonexistent"); !errors.Is(err, apperrors.ErrBillPaymentNotFound) {
		t.Errorf("expected ErrBillPaymentNotFound, got %v", err)
	}
}

// ---- ListForMonth ----

func TestBillsService_ListForMonth_MonthlyBillIncluded(t *testing.T) {
	// A monthly bill with DueDay=15 must appear in every month.
	dueDay := 15
	bill := model.Bill{ID: "b1", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &dueDay}
	store := newFakeBillStore(bill)
	svc := NewBillsService(store, &fakeBillPaymentStore{})

	dues, err := svc.ListForMonth(context.Background(), "user1", 2024, 6)
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(dues) != 1 {
		t.Fatalf("expected 1 bill due in June, got %d", len(dues))
	}
	if dues[0].ComputedDueDate != "2024-06-15" {
		t.Errorf("ComputedDueDate = %q, want %q", dues[0].ComputedDueDate, "2024-06-15")
	}
}

func TestBillsService_ListForMonth_PaidBillMarked(t *testing.T) {
	dueDay := 1
	bill := model.Bill{ID: "b1", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &dueDay}
	store := newFakeBillStore(bill)
	paidDate := "2024-06-01"
	payment := model.BillPayment{ID: "p1", BillID: "b1", ComputedDueDate: "2024-06-01", PaidDate: paidDate}
	paymentStore := &fakeBillPaymentStore{payments: []model.BillPayment{payment}}
	svc := NewBillsService(store, paymentStore)

	dues, err := svc.ListForMonth(context.Background(), "user1", 2024, 6)
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(dues) != 1 || !dues[0].IsPaid {
		t.Errorf("expected bill to be marked as paid, got IsPaid=%v", dues[0].IsPaid)
	}
}

func TestBillsService_ListForMonth_OnceRecurrence_WrongMonth_Excluded(t *testing.T) {
	// A one-time bill in June must not appear in July.
	dueDate := "2024-06-15"
	bill := model.Bill{ID: "b1", Name: "Fee", CategoryID: "other", RecurrenceType: "once", DueDate: &dueDate}
	store := newFakeBillStore(bill)
	svc := NewBillsService(store, &fakeBillPaymentStore{})

	dues, err := svc.ListForMonth(context.Background(), "user1", 2024, 7)
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(dues) != 0 {
		t.Errorf("expected 0 bills in July for a June one-time bill, got %d", len(dues))
	}
}

func TestBillsService_ListForMonth_SortedByDueDate(t *testing.T) {
	day15 := 15
	day5 := 5
	bills := []model.Bill{
		{ID: "b1", Name: "Late", CategoryID: "utilities", RecurrenceType: "monthly", DueDay: &day15},
		{ID: "b2", Name: "Early", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &day5},
	}
	store := newFakeBillStore(bills...)
	svc := NewBillsService(store, &fakeBillPaymentStore{})

	dues, err := svc.ListForMonth(context.Background(), "user1", 2024, 6)
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	if len(dues) != 2 {
		t.Fatalf("expected 2 bills, got %d", len(dues))
	}
	if dues[0].ComputedDueDate >= dues[1].ComputedDueDate {
		t.Errorf("expected bills sorted ascending: %q >= %q", dues[0].ComputedDueDate, dues[1].ComputedDueDate)
	}
}

// ---- ListYear ----

func TestBillsService_ListYear_MonthlyBill_AppearsInAllMonths(t *testing.T) {
	dueDay := 10
	bill := model.Bill{ID: "b1", Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: &dueDay}
	store := newFakeBillStore(bill)
	svc := NewBillsService(store, &fakeBillPaymentStore{})

	result, err := svc.ListYear(context.Background(), "user1", 2024)
	if err != nil {
		t.Fatalf("ListYear: %v", err)
	}
	if len(result) != 12 {
		t.Errorf("expected 12 month keys, got %d", len(result))
	}
	for m := 1; m <= 12; m++ {
		if len(result[m]) != 1 {
			t.Errorf("month %d: expected 1 bill, got %d", m, len(result[m]))
		}
	}
}

// ---- List ----

func TestBillsService_List_ReturnsAll(t *testing.T) {
	store := newFakeBillStore(
		model.Bill{ID: "b1", Name: "Rent"},
		model.Bill{ID: "b2", Name: "Electric"},
	)
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	bills, total, err := svc.List(context.Background(), "user1", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(bills) != 2 {
		t.Errorf("expected 2 bills, got total=%d len=%d", total, len(bills))
	}
}

// ---- Get ----

func TestBillsService_Get_Success(t *testing.T) {
	store := newFakeBillStore(model.Bill{ID: "b1", Name: "Rent"})
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	bill, err := svc.Get(context.Background(), "b1", "user1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if bill.Name != "Rent" {
		t.Errorf("Name = %q, want %q", bill.Name, "Rent")
	}
}

// ---- Create ----

func TestBillsService_Create_InvalidFields_ReturnsValidationError(t *testing.T) {
	svc := NewBillsService(newFakeBillStore(), &fakeBillPaymentStore{})
	// Empty name triggers validation.
	u := model.BillUpdate{Name: "", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if _, err := svc.Create(context.Background(), "user1", u); !errors.Is(err, apperrors.ErrBillValidation) {
		t.Errorf("expected ErrBillValidation, got %v", err)
	}
}

func TestBillsService_Create_Success(t *testing.T) {
	store := newFakeBillStore()
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	bill, err := svc.Create(context.Background(), "user1", u)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if bill.ID == "" {
		t.Error("expected a non-empty ID to be assigned")
	}
	if bill.Name != "Rent" {
		t.Errorf("Name = %q, want %q", bill.Name, "Rent")
	}
}

// ---- Update ----

func TestBillsService_Update_NotFound(t *testing.T) {
	// updateN=0 (default) → service returns ErrBillNotFound.
	store := newFakeBillStore()
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if _, err := svc.Update(context.Background(), "nonexistent", "user1", u); !errors.Is(err, apperrors.ErrBillNotFound) {
		t.Errorf("expected ErrBillNotFound, got %v", err)
	}
}

func TestBillsService_Update_Success(t *testing.T) {
	store := newFakeBillStore(model.Bill{ID: "b1", Name: "Rent"})
	store.updateN = 1 // simulate 1 row updated
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	u := model.BillUpdate{Name: "Rent Updated", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	bill, err := svc.Update(context.Background(), "b1", "user1", u)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if bill.ID != "b1" {
		t.Errorf("ID = %q, want %q", bill.ID, "b1")
	}
}

// ---- MarkPaid ----

func TestBillsService_MarkPaid_BillNotFound(t *testing.T) {
	// store has no bill, so Get returns sql.ErrNoRows → ErrBillNotFound.
	svc := NewBillsService(newFakeBillStore(), &fakeBillPaymentStore{})
	if _, err := svc.MarkPaid(context.Background(), "user1", "nonexistent", "2024-06-01", "2024-06-01", nil); !errors.Is(err, apperrors.ErrBillNotFound) {
		t.Errorf("expected ErrBillNotFound, got %v", err)
	}
}

func TestBillsService_MarkPaid_Success(t *testing.T) {
	store := newFakeBillStore(model.Bill{ID: "b1"})
	paymentStore := &fakeBillPaymentStore{}
	svc := NewBillsService(store, paymentStore)

	payment, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "2024-06-01", nil)
	if err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	if payment.BillID != "b1" {
		t.Errorf("BillID = %q, want %q", payment.BillID, "b1")
	}
	if payment.ID == "" {
		t.Error("expected non-empty payment ID")
	}
}

// ---- Unmark ----

func TestBillsService_MarkPaid_DuplicatePayment_ReturnsAlreadyPaid(t *testing.T) {
	// Simulate paymentStore.Create returning a UNIQUE constraint error.
	store := newFakeBillStore(model.Bill{ID: "b1"})
	paymentStore := &fakeBillPaymentStore{
		createFn: func(_ model.BillPaymentCreate) (model.BillPayment, error) {
			return model.BillPayment{}, fmt.Errorf("UNIQUE constraint failed: bill_payments.bill_id")
		},
	}
	svc := NewBillsService(store, paymentStore)
	if _, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "2024-06-01", nil); !errors.Is(err, apperrors.ErrBillAlreadyPaid) {
		t.Errorf("expected ErrBillAlreadyPaid for duplicate payment, got %v", err)
	}
}

func TestBillsService_Unmark_Success(t *testing.T) {
	paymentStore := &fakeBillPaymentStore{deleteN: 1}
	svc := NewBillsService(newFakeBillStore(), paymentStore)
	if err := svc.Unmark(context.Background(), "user1", "p1"); err != nil {
		t.Fatalf("Unmark: %v", err)
	}
}

// ---- ListCurrentMonth ----

// ---- Error paths ----

func TestBillsService_List_StoreError_Propagates(t *testing.T) {
	store := newFakeBillStore()
	store.listErr = fmt.Errorf("db failure")
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if _, _, err := svc.List(context.Background(), "user1", 10, 0); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestBillsService_Create_StoreError_Propagates(t *testing.T) {
	store := newFakeBillStore()
	store.createErr = fmt.Errorf("db failure")
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if _, err := svc.Create(context.Background(), "user1", u); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestBillsService_Update_StoreError_Propagates(t *testing.T) {
	store := newFakeBillStore()
	store.updateErr = fmt.Errorf("db failure")
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	u := model.BillUpdate{Name: "Rent", CategoryID: "rent", RecurrenceType: "monthly", DueDay: intPtr(1)}
	if _, err := svc.Update(context.Background(), "b1", "user1", u); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestBillsService_ListCurrentMonth_Succeeds(t *testing.T) {
	dueDay := 1
	store := newFakeBillStore(model.Bill{ID: "b1", Name: "Rent", RecurrenceType: "monthly", DueDay: &dueDay})
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	dues, err := svc.ListCurrentMonth(context.Background(), "user1")
	if err != nil {
		t.Fatalf("ListCurrentMonth: %v", err)
	}
	if len(dues) != 1 {
		t.Errorf("expected 1 bill due this month, got %d", len(dues))
	}
}

func TestBillsService_Delete_StoreError_Propagates(t *testing.T) {
	store := newFakeBillStore()
	store.deleteErr = fmt.Errorf("db failure")
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if err := svc.Delete(context.Background(), "b1", "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestBillsService_Get_GenericStoreError_Propagates(t *testing.T) {
	store := newFakeBillStore()
	store.getErr = fmt.Errorf("db failure") // non-sql.ErrNoRows error
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if _, err := svc.Get(context.Background(), "b1", "user1"); err == nil {
		t.Error("expected generic store error to propagate, got nil")
	}
}

func TestBillsService_Unmark_StoreError_Propagates(t *testing.T) {
	paymentStore := &fakeBillPaymentStore{deleteErr: fmt.Errorf("db failure")}
	svc := NewBillsService(newFakeBillStore(), paymentStore)
	if err := svc.Unmark(context.Background(), "user1", "p1"); err == nil {
		t.Error("expected payment store error to propagate, got nil")
	}
}

func TestBillsService_MarkPaid_GenericGetError_Propagates(t *testing.T) {
	// getErr set to a non-sql.ErrNoRows error: should propagate, not be mapped to ErrBillNotFound.
	store := newFakeBillStore()
	store.getErr = fmt.Errorf("db failure")
	svc := NewBillsService(store, &fakeBillPaymentStore{})
	if _, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "2024-06-01", nil); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestBillsService_MarkPaid_CreateNonUniqueError_Propagates(t *testing.T) {
	// paymentStore returns a non-UNIQUE constraint error → should propagate (not map to ErrBillAlreadyPaid).
	store := newFakeBillStore(model.Bill{ID: "b1"})
	paymentStore := &fakeBillPaymentStore{
		createFn: func(_ model.BillPaymentCreate) (model.BillPayment, error) {
			return model.BillPayment{}, fmt.Errorf("foreign key constraint failed")
		},
	}
	svc := NewBillsService(store, paymentStore)
	if _, err := svc.MarkPaid(context.Background(), "user1", "b1", "2024-06-01", "2024-06-01", nil); err == nil {
		t.Error("expected non-UNIQUE store error to propagate, got nil")
	}
}

func TestFirstOccurrenceInMonth_InvalidAnchorDate_ReturnsFalse(t *testing.T) {
	// Invalid anchor date string must return ("", false) without panicking.
	_, ok := firstOccurrenceInMonth(func() *string { s := "bad-date"; return &s }(), 7, 2024, time.January)
	if ok {
		t.Error("expected ok=false for invalid anchor date, got true")
	}
}

func TestFirstOccurrenceInMonth_DiffDaysPositive_ReturnsOccurrence(t *testing.T) {
	// Anchor in the past (2024-01-01), check March 2024 — diffDays > 0 path.
	anchor := "2024-01-01"
	_, ok := firstOccurrenceInMonth(&anchor, 7, 2024, time.March)
	if !ok {
		t.Error("expected weekly occurrence in March for anchor 2024-01-01")
	}
}

func TestBillsService_ListYear_AnnualBill_AppearsOnlyOnce(t *testing.T) {
	dueDay, dueMonth := 15, 3
	bill := model.Bill{ID: "b1", Name: "Insurance", CategoryID: "insurance", RecurrenceType: "annual", DueDay: &dueDay, DueMonth: &dueMonth}
	store := newFakeBillStore(bill)
	svc := NewBillsService(store, &fakeBillPaymentStore{})

	result, err := svc.ListYear(context.Background(), "user1", 2024)
	if err != nil {
		t.Fatalf("ListYear: %v", err)
	}
	total := 0
	for _, dues := range result {
		total += len(dues)
	}
	if total != 1 {
		t.Errorf("expected annual bill to appear exactly once, appeared %d times", total)
	}
	if len(result[3]) != 1 {
		t.Error("expected annual bill to appear in March (month 3)")
	}
}
