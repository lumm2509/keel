package keel

import transporthttp "github.com/lumm2509/keel/transport/http"

// ApiError is the standard structured error returned by HTTP handlers.
type ApiError = transporthttp.ApiError

// NewApiError creates a new ApiError with the given HTTP status code and message.
func NewApiError(status int, message string, data any) *ApiError {
	return transporthttp.NewApiError(status, message, data)
}

// NewNotFoundError creates a 404 ApiError.
func NewNotFoundError(message string, data any) *ApiError {
	return transporthttp.NewNotFoundError(message, data)
}

// NewBadRequestError creates a 400 ApiError.
func NewBadRequestError(message string, data any) *ApiError {
	return transporthttp.NewBadRequestError(message, data)
}

// NewUnauthorizedError creates a 401 ApiError.
func NewUnauthorizedError(message string, data any) *ApiError {
	return transporthttp.NewUnauthorizedError(message, data)
}

// NewForbiddenError creates a 403 ApiError.
func NewForbiddenError(message string, data any) *ApiError {
	return transporthttp.NewForbiddenError(message, data)
}

// NewInternalServerError creates a 500 ApiError.
func NewInternalServerError(message string, data any) *ApiError {
	return transporthttp.NewInternalServerError(message, data)
}

// NewTooManyRequestsError creates a 429 ApiError.
func NewTooManyRequestsError(message string, data any) *ApiError {
	return transporthttp.NewTooManyRequestsError(message, data)
}
