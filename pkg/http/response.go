// Package http provides the user service's JSON response envelopes.
package http

import "github.com/labstack/echo/v4"

// Error is the structured error payload returned by HTTP handlers.
type Error struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ErrorResponse wraps an API error with its request trace identifier.
type ErrorResponse struct {
	Error   Error  `json:"error"`
	TraceID string `json:"trace_id"`
}

// Response wraps successful API data in the service's standard envelope.
type Response struct {
	Data interface{} `json:"data,omitempty"`
}

// JSON writes data using the standard success envelope.
func JSON(c echo.Context, status int, data interface{}) error {
	return c.JSON(status, Response{Data: data})
}

// ErrorJSON writes a structured error and trace identifier to the response.
func ErrorJSON(c echo.Context, status int, code, message, traceID string, details interface{}) error {
	return c.JSON(status, ErrorResponse{Error: Error{Code: code, Message: message, Details: details}, TraceID: traceID})
}
