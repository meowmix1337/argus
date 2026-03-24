package handler

import "github.com/meowmix1337/argus/backend/internal/model"

// CreateBillRequest is the JSON body for POST /api/bills.
type CreateBillRequest struct {
	Name           string   `json:"name"                   validate:"required,min=1,max=200"`
	Amount         *float64 `json:"amount,omitempty"       validate:"omitempty,gt=0"`
	CategoryID     string   `json:"categoryId"             validate:"required,oneof=rent utilities subscriptions insurance loans medical other"`
	RecurrenceType string   `json:"recurrenceType"         validate:"required,oneof=once weekly biweekly monthly quarterly annual"`
	DueDate        *string  `json:"dueDate,omitempty"`
	DueDay         *int     `json:"dueDay,omitempty"       validate:"omitempty,min=1,max=31"`
	DueMonth       *int     `json:"dueMonth,omitempty"     validate:"omitempty,min=1,max=12"`
	AnchorDate     *string  `json:"anchorDate,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
}

// UpdateBillRequest is the JSON body for PATCH /api/bills/{id} (full replacement).
type UpdateBillRequest struct {
	Name           string   `json:"name"                   validate:"required,min=1,max=200"`
	Amount         *float64 `json:"amount,omitempty"       validate:"omitempty,gt=0"`
	CategoryID     string   `json:"categoryId"             validate:"required,oneof=rent utilities subscriptions insurance loans medical other"`
	RecurrenceType string   `json:"recurrenceType"         validate:"required,oneof=once weekly biweekly monthly quarterly annual"`
	DueDate        *string  `json:"dueDate,omitempty"`
	DueDay         *int     `json:"dueDay,omitempty"       validate:"omitempty,min=1,max=31"`
	DueMonth       *int     `json:"dueMonth,omitempty"     validate:"omitempty,min=1,max=12"`
	AnchorDate     *string  `json:"anchorDate,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
}

// BillResponse is the JSON response for a single bill.
type BillResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Amount         *float64 `json:"amount"`
	CategoryID     string   `json:"categoryId"`
	RecurrenceType string   `json:"recurrenceType"`
	DueDate        *string  `json:"dueDate"`
	DueDay         *int     `json:"dueDay"`
	DueMonth       *int     `json:"dueMonth"`
	AnchorDate     *string  `json:"anchorDate"`
	Notes          *string  `json:"notes"`
	CreatedAt      string   `json:"createdAt"`
}

// BillListResponse is the paginated JSON response for GET /api/bills.
type BillListResponse struct {
	Bills  []BillResponse `json:"bills"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func billToResponse(b model.Bill) BillResponse {
	return BillResponse{
		ID:             b.ID,
		Name:           b.Name,
		Amount:         b.Amount,
		CategoryID:     b.CategoryID,
		RecurrenceType: b.RecurrenceType,
		DueDate:        b.DueDate,
		DueDay:         b.DueDay,
		DueMonth:       b.DueMonth,
		AnchorDate:     b.AnchorDate,
		Notes:          b.Notes,
		CreatedAt:      b.CreatedAt,
	}
}

func billsToResponse(bills []model.Bill) []BillResponse {
	resp := make([]BillResponse, 0, len(bills))
	for _, b := range bills {
		resp = append(resp, billToResponse(b))
	}
	return resp
}

func reqToBillUpdate(req CreateBillRequest) model.BillUpdate {
	return model.BillUpdate{
		Name:           req.Name,
		Amount:         req.Amount,
		CategoryID:     req.CategoryID,
		RecurrenceType: req.RecurrenceType,
		DueDate:        req.DueDate,
		DueDay:         req.DueDay,
		DueMonth:       req.DueMonth,
		AnchorDate:     req.AnchorDate,
		Notes:          req.Notes,
	}
}

func updateReqToBillUpdate(req UpdateBillRequest) model.BillUpdate {
	return model.BillUpdate{
		Name:           req.Name,
		Amount:         req.Amount,
		CategoryID:     req.CategoryID,
		RecurrenceType: req.RecurrenceType,
		DueDate:        req.DueDate,
		DueDay:         req.DueDay,
		DueMonth:       req.DueMonth,
		AnchorDate:     req.AnchorDate,
		Notes:          req.Notes,
	}
}
