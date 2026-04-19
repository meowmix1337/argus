package repository

import "github.com/meowmix1337/argus/backend/internal/model"

// sqliteUserSummaryRow mirrors the columns needed for a lightweight user profile.
// Used by SQLiteUsersRepository and SQLiteFollowRepository.
type sqliteUserSummaryRow struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	AvatarURL string `db:"avatar_url"`
}

func (r *sqliteUserSummaryRow) toModel() model.UserSummary {
	return model.UserSummary{
		ID:        r.ID,
		Name:      r.Name,
		AvatarURL: r.AvatarURL,
	}
}
