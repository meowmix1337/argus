package service

import userssvc "github.com/meowmix1337/argus/backend/internal/domain/users/service"

// UserService is an alias for the domain users service. Deprecated: import domain/users/service directly.
type UserService = userssvc.UserService

// NewUserService creates a new UserService.
var NewUserService = userssvc.NewUserService
