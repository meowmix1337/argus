package handler

// UpdateWatchedReposRequest is the body for PUT /api/integrations/github/repos.
// Repos is a list of "owner/repo" full names to watch; an empty list deselects all.
type UpdateWatchedReposRequest struct {
	Repos []string `json:"repos" validate:"dive,required"`
}
