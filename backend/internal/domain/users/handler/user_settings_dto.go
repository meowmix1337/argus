package handler

// UpsertSettingsRequest is the JSON body for PUT /api/settings.
type UpsertSettingsRequest struct {
	Latitude       *float64 `json:"latitude"         validate:"omitempty,min=-90,max=90"`
	Longitude      *float64 `json:"longitude"        validate:"omitempty,min=-180,max=180"`
	CalendarICSURL *string  `json:"calendar_ics_url" validate:"omitempty,url"`
	Timezone       *string  `json:"timezone"         validate:"omitempty,max=64"`
}

// SetNewsCategoriesRequest is the JSON body for PUT /api/settings/news-categories.
type SetNewsCategoriesRequest struct {
	CategoryIDs []string `json:"category_ids" validate:"required,min=1,max=9,dive,required"`
}

// UserSettingsResponse is the JSON response for user settings.
type UserSettingsResponse struct {
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	CalendarICSURL *string  `json:"calendar_ics_url"`
	Timezone       *string  `json:"timezone"`
}

// NewsCategoryTypeResponse is the JSON representation of a news category type.
type NewsCategoryTypeResponse struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
}

// NewsCategoriesResponse is the JSON response containing available and selected news categories.
type NewsCategoriesResponse struct {
	Available []NewsCategoryTypeResponse `json:"available"`
	Selected  []NewsCategoryTypeResponse `json:"selected"`
}
