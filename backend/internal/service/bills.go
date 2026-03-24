package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// BillStore defines the data-access contract for bills.
type BillStore interface {
	List(ctx context.Context, userID string, limit, offset int) ([]model.Bill, int, error)
	Get(ctx context.Context, id string, userID string) (model.Bill, error)
	Create(ctx context.Context, b model.BillCreate) (model.Bill, error)
	Update(ctx context.Context, id string, userID string, u model.BillUpdate) (int64, error)
	Delete(ctx context.Context, id string, userID string) (int64, error)
	ListActive(ctx context.Context, userID string) ([]model.Bill, error)
}

// BillsService manages bills via a store.
type BillsService struct {
	store BillStore
}

// NewBillsService creates a new BillsService backed by the given store.
func NewBillsService(store BillStore) *BillsService {
	return &BillsService{store: store}
}

// List returns a page of bills for the given user along with the total count.
func (s *BillsService) List(ctx context.Context, userID string, limit, offset int) ([]model.Bill, int, error) {
	bills, total, err := s.store.List(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list bills: %w", err)
	}
	return bills, total, nil
}

// Get returns a single bill by ID, scoped to the given user.
func (s *BillsService) Get(ctx context.Context, id string, userID string) (model.Bill, error) {
	bill, err := s.store.Get(ctx, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Bill{}, apperrors.ErrBillNotFound
		}
		return model.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return bill, nil
}

// Create adds a new bill for the given user.
func (s *BillsService) Create(ctx context.Context, userID string, u model.BillUpdate) (model.Bill, error) {
	if err := validateBillFields(u); err != nil {
		return model.Bill{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("failed to generate UUID v7 for bill", "error", err)
		return model.Bill{}, fmt.Errorf("generate bill id: %w", err)
	}

	bill, err := s.store.Create(ctx, model.BillCreate{
		ID:             id.String(),
		UserID:         userID,
		Name:           u.Name,
		Amount:         u.Amount,
		CategoryID:     u.CategoryID,
		RecurrenceType: u.RecurrenceType,
		DueDate:        u.DueDate,
		DueDay:         u.DueDay,
		DueMonth:       u.DueMonth,
		AnchorDate:     u.AnchorDate,
		Notes:          u.Notes,
	})
	if err != nil {
		return model.Bill{}, fmt.Errorf("create bill: %w", err)
	}
	return bill, nil
}

// Update performs a full-replacement update of a bill by ID, scoped to the given user.
func (s *BillsService) Update(ctx context.Context, id string, userID string, u model.BillUpdate) (model.Bill, error) {
	if err := validateBillFields(u); err != nil {
		return model.Bill{}, err
	}

	n, err := s.store.Update(ctx, id, userID, u)
	if err != nil {
		return model.Bill{}, fmt.Errorf("update bill: %w", err)
	}
	if n == 0 {
		return model.Bill{}, apperrors.ErrBillNotFound
	}

	bill, err := s.store.Get(ctx, id, userID)
	if err != nil {
		return model.Bill{}, fmt.Errorf("fetch updated bill: %w", err)
	}
	return bill, nil
}

// Delete soft-deletes a bill by ID, scoped to the given user.
func (s *BillsService) Delete(ctx context.Context, id string, userID string) error {
	n, err := s.store.Delete(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("delete bill: %w", err)
	}
	if n == 0 {
		return apperrors.ErrBillNotFound
	}
	return nil
}

// ListCurrentMonth returns all bills that have an occurrence in the current calendar
// month, sorted ascending by their computed due date within that month.
func (s *BillsService) ListCurrentMonth(ctx context.Context, userID string) ([]model.BillDue, error) {
	bills, err := s.store.ListActive(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active bills: %w", err)
	}

	now := time.Now().UTC()
	year, month := now.Year(), now.Month()

	var result []model.BillDue
	for _, b := range bills {
		dueDate, ok := computeCurrentMonthDueDate(b, year, month)
		if !ok {
			continue
		}
		result = append(result, model.BillDue{
			ID:              b.ID,
			Name:            b.Name,
			Amount:          b.Amount,
			CategoryID:      b.CategoryID,
			RecurrenceType:  b.RecurrenceType,
			Notes:           b.Notes,
			ComputedDueDate: dueDate,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ComputedDueDate < result[j].ComputedDueDate
	})

	return result, nil
}

// validateBillFields validates shared bill fields for create and update.
func validateBillFields(u model.BillUpdate) error {
	u.Name = strings.TrimSpace(u.Name)
	if u.Name == "" {
		return fmt.Errorf("%w: bill name cannot be empty", apperrors.ErrBillValidation)
	}

	validCategories := map[string]bool{
		"rent": true, "utilities": true, "subscriptions": true,
		"insurance": true, "loans": true, "medical": true, "other": true,
	}
	if !validCategories[u.CategoryID] {
		return fmt.Errorf("%w: invalid category %q", apperrors.ErrBillValidation, u.CategoryID)
	}

	validRecurrence := map[string]bool{
		"once": true, "weekly": true, "biweekly": true,
		"monthly": true, "quarterly": true, "annual": true,
	}
	if !validRecurrence[u.RecurrenceType] {
		return fmt.Errorf("%w: invalid recurrence type %q", apperrors.ErrBillValidation, u.RecurrenceType)
	}

	switch u.RecurrenceType {
	case "once":
		if u.DueDate == nil {
			return fmt.Errorf("%w: due_date required for recurrence type 'once'", apperrors.ErrBillValidation)
		}
		if _, err := time.Parse("2006-01-02", *u.DueDate); err != nil {
			return fmt.Errorf("%w: due_date must be YYYY-MM-DD", apperrors.ErrBillValidation)
		}
	case "monthly":
		if u.DueDay == nil {
			return fmt.Errorf("%w: due_day required for recurrence type 'monthly'", apperrors.ErrBillValidation)
		}
	case "annual":
		if u.DueDay == nil || u.DueMonth == nil {
			return fmt.Errorf("%w: due_day and due_month required for recurrence type 'annual'", apperrors.ErrBillValidation)
		}
	case "weekly", "biweekly", "quarterly":
		if u.AnchorDate == nil {
			return fmt.Errorf("%w: anchor_date required for recurrence type %q", apperrors.ErrBillValidation, u.RecurrenceType)
		}
		if _, err := time.Parse("2006-01-02", *u.AnchorDate); err != nil {
			return fmt.Errorf("%w: anchor_date must be YYYY-MM-DD", apperrors.ErrBillValidation)
		}
	}

	return nil
}

// computeCurrentMonthDueDate returns the computed due date (YYYY-MM-DD) for a bill's
// occurrence within the given year+month, or ("", false) if none falls in that month.
func computeCurrentMonthDueDate(b model.Bill, year int, month time.Month) (string, bool) {
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := monthStart.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()

	switch b.RecurrenceType {
	case "once":
		if b.DueDate == nil {
			return "", false
		}
		t, err := time.Parse("2006-01-02", *b.DueDate)
		if err != nil {
			return "", false
		}
		if t.Year() == year && t.Month() == month {
			return *b.DueDate, true
		}
		return "", false

	case "monthly":
		if b.DueDay == nil {
			return "", false
		}
		day := *b.DueDay
		if day > lastDay {
			day = lastDay
		}
		return fmt.Sprintf("%04d-%02d-%02d", year, int(month), day), true

	case "annual":
		if b.DueMonth == nil || b.DueDay == nil {
			return "", false
		}
		if time.Month(*b.DueMonth) != month {
			return "", false
		}
		day := *b.DueDay
		if day > lastDay {
			day = lastDay
		}
		return fmt.Sprintf("%04d-%02d-%02d", year, int(month), day), true

	case "weekly":
		return firstOccurrenceInMonth(b.AnchorDate, 7, year, month)

	case "biweekly":
		return firstOccurrenceInMonth(b.AnchorDate, 14, year, month)

	case "quarterly":
		return quarterlyOccurrenceInMonth(b.AnchorDate, year, month)
	}

	return "", false
}

// firstOccurrenceInMonth finds the first occurrence of anchor + n*intervalDays
// that falls within the given year/month, using UTC day arithmetic.
func firstOccurrenceInMonth(anchorDate *string, intervalDays int, year int, month time.Month) (string, bool) {
	if anchorDate == nil {
		return "", false
	}
	anchor, err := time.Parse("2006-01-02", *anchorDate)
	if err != nil {
		return "", false
	}
	anchor = time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.UTC)

	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).AddDate(0, 0, -1)

	diffDays := int(monthStart.Sub(anchor).Hours()) / 24

	// First n such that anchor + n*interval >= monthStart
	var startN int
	if diffDays <= 0 {
		startN = 0
	} else {
		startN = diffDays / intervalDays
	}

	// Check startN (may land just before monthStart) and startN+1
	for _, n := range []int{startN, startN + 1} {
		if n < 0 {
			continue
		}
		t := anchor.AddDate(0, 0, n*intervalDays)
		if !t.Before(monthStart) && !t.After(monthEnd) {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// quarterlyOccurrenceInMonth returns the due date if the bill falls in this month
// based on 3-month intervals from the anchor date (using calendar-month arithmetic).
func quarterlyOccurrenceInMonth(anchorDate *string, year int, month time.Month) (string, bool) {
	if anchorDate == nil {
		return "", false
	}
	anchor, err := time.Parse("2006-01-02", *anchorDate)
	if err != nil {
		return "", false
	}

	anchorMonths := anchor.Year()*12 + int(anchor.Month()) - 1 // 0-based
	targetMonths := year*12 + int(month) - 1                   // 0-based
	diff := targetMonths - anchorMonths

	if diff < 0 || diff%3 != 0 {
		return "", false
	}

	// Due day is same day-of-month as anchor, capped to last day of target month
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := monthStart.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()
	day := anchor.Day()
	if day > lastDay {
		day = lastDay
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, int(month), day), true
}
