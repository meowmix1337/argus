package handler

// CreateTaskRequest is the JSON body for POST /api/tasks.
type CreateTaskRequest struct {
	Text     string `json:"text"     validate:"required,min=1,max=1000"`
	Priority string `json:"priority,omitempty" validate:"omitempty,oneof=high medium low"`
}

// UpdateTaskRequest is the JSON body for PATCH /api/tasks/{id}.
type UpdateTaskRequest struct {
	Done     *bool   `json:"done,omitempty"`
	Text     *string `json:"text,omitempty"     validate:"omitempty,min=1,max=1000"`
	Priority *string `json:"priority,omitempty" validate:"omitempty,oneof=high medium low"`
}

// TaskResponse is the JSON response for a single task.
type TaskResponse struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
}

// TaskListResponse is the paginated JSON response for GET /api/tasks.
type TaskListResponse struct {
	Tasks  []TaskResponse `json:"tasks"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}
