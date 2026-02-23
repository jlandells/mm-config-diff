package main

import (
	"fmt"
	"net/http"
)

// Exit codes
const (
	ExitSuccess     = 0
	ExitConfigError = 1 // missing flags, invalid input, auth failure
	ExitAPIError    = 2 // connection failure, unexpected API response
	ExitDriftFound  = 3 // diff completed and differences were found
	ExitOutputError = 4 // unable to write output file
)

// ExitError represents an error with an associated exit code.
type ExitError struct {
	Code    int
	Message string
	Err     error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// NewExitError creates a new ExitError.
func NewExitError(code int, message string, err error) *ExitError {
	return &ExitError{Code: code, Message: message, Err: err}
}

// ClassifyAPIError maps HTTP status codes and connection errors to user-facing messages.
func ClassifyAPIError(statusCode int, serverURL string, err error) *ExitError {
	switch {
	case statusCode == http.StatusUnauthorized:
		return NewExitError(ExitAPIError, "error: authentication failed. Check your token or credentials.", err)
	case statusCode == http.StatusForbidden:
		return NewExitError(ExitAPIError, "error: permission denied. This operation requires a System Administrator account.", err)
	case statusCode == http.StatusNotFound:
		return NewExitError(ExitAPIError, "error: the requested resource was not found on the server.", err)
	case statusCode >= 500:
		return NewExitError(ExitAPIError, fmt.Sprintf("error: the Mattermost server returned an unexpected error (HTTP %d). Check server logs for details.", statusCode), err)
	case statusCode == 0 && err != nil:
		return NewExitError(ExitAPIError, fmt.Sprintf("error: unable to connect to %s. Check the URL and network connectivity.", serverURL), err)
	default:
		return NewExitError(ExitAPIError, fmt.Sprintf("error: unexpected API error (HTTP %d).", statusCode), err)
	}
}
