package service

import userssvc "github.com/meowmix1337/argus/backend/internal/domain/users/service"

// AuthService is an alias for the domain users auth service. Deprecated: import domain/users/service directly.
type AuthService = userssvc.AuthService

// GoogleUser is an alias for the domain users GoogleUser type. Deprecated: import domain/users/service directly.
type GoogleUser = userssvc.GoogleUser

// NewAuthService creates a new AuthService.
var NewAuthService = userssvc.NewAuthService
