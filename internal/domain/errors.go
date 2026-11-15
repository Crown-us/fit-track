// Package domain holds the small vocabulary shared by every feature package:
// sentinel errors, validation errors, calendar dates and pagination.
package domain

import (
	"errors"
	"strings"
)

// Sentinel errors that services return and the HTTP layer maps to statuses.
var (
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrForbidden          = errors.New("action not allowed")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrProfileRequired    = errors.New("profile must be completed first")
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError collects every invalid field of one request so clients can
// fix them all in a single round trip.
type ValidationError struct {
	Fields []FieldError
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Validator accumulates field errors.
type Validator struct {
	fields []FieldError
}

// Check records message for field when isValid is false.
func (v *Validator) Check(isValid bool, field, message string) {
	if !isValid {
		v.fields = append(v.fields, FieldError{Field: field, Message: message})
	}
}

// Err returns a *ValidationError when any check failed, otherwise nil.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.fields}
}
