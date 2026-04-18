package validate

import "github.com/go-playground/validator/v10"

// New creates a shared validator instance with any custom validators registered.
func New() *validator.Validate {
	return validator.New()
}
