package handler

// CreateLabelRequest is the JSON body for POST /api/labels.
type CreateLabelRequest struct {
	Name  string `json:"name"  validate:"required,min=1,max=16"`
	Color string `json:"color" validate:"omitempty,hexcolor"`
}

// UpdateLabelRequest is the JSON body for PATCH /api/labels/{id}.
type UpdateLabelRequest struct {
	Name  *string `json:"name,omitempty"  validate:"omitempty,min=1,max=16"`
	Color *string `json:"color,omitempty" validate:"omitempty,hexcolor"`
}

// AssignLabelRequest is the JSON body for POST /api/tasks/{taskID}/labels.
type AssignLabelRequest struct {
	LabelID string `json:"label_id" validate:"required,uuid"`
}

// LabelResponse is the JSON response for a single label.
type LabelResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
}
