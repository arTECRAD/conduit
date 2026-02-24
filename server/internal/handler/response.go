package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIResponse is the standard envelope for all API responses.
type APIResponse[T any] struct {
	Data  T         `json:"data"`
	Error *APIError `json:"error"`
}

// APIError holds a machine-readable error code and human-readable message.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondOK sends a 200 (or custom status) JSON response.
func respondOK(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse[interface{}]{Data: data, Error: nil})
}

// respondCreated sends a 201 JSON response.
func respondCreated(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, APIResponse[interface{}]{Data: data, Error: nil})
}

// respondError sends a JSON error response with the given status code.
func respondError(c echo.Context, statusCode int, code, message string) error {
	return c.JSON(statusCode, APIResponse[interface{}]{
		Data:  nil,
		Error: &APIError{Code: code, Message: message},
	})
}
