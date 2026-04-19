package service

import (
	usersrepo "github.com/meowmix1337/argus/backend/internal/domain/users/repository"
	userssvc "github.com/meowmix1337/argus/backend/internal/domain/users/service"
)

// UserSettingsService is an alias for the domain users settings service. Deprecated: import domain/users/service directly.
type UserSettingsService = userssvc.UserSettingsService

// UserSettingsStore is an alias for the domain users settings store interface. Deprecated: import domain/users/repository directly.
type UserSettingsStore = usersrepo.UserSettingsStore

// NewUserSettingsService creates a new UserSettingsService.
var NewUserSettingsService = userssvc.NewUserSettingsService
