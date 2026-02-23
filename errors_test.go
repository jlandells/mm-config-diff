package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestExitError_Error(t *testing.T) {
	tests := []struct {
		name     string
		exitErr  *ExitError
		wantMsg  string
	}{
		{
			name:    "message only",
			exitErr: &ExitError{Code: ExitConfigError, Message: "missing flag"},
			wantMsg: "missing flag",
		},
		{
			name:    "message with wrapped error",
			exitErr: &ExitError{Code: ExitAPIError, Message: "connection failed", Err: errors.New("dial tcp: timeout")},
			wantMsg: "connection failed: dial tcp: timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.exitErr.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestExitError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	exitErr := &ExitError{Code: ExitAPIError, Message: "outer", Err: inner}
	if !errors.Is(exitErr, inner) {
		t.Error("Unwrap should expose the inner error")
	}
}

func TestClassifyAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		serverURL  string
		err        error
		wantCode   int
		wantSubstr string
	}{
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			serverURL:  "https://mm.example.com",
			err:        errors.New("unauthorized"),
			wantCode:   ExitAPIError,
			wantSubstr: "authentication failed",
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			serverURL:  "https://mm.example.com",
			err:        errors.New("forbidden"),
			wantCode:   ExitAPIError,
			wantSubstr: "permission denied",
		},
		{
			name:       "500 Server Error",
			statusCode: http.StatusInternalServerError,
			serverURL:  "https://mm.example.com",
			err:        errors.New("internal error"),
			wantCode:   ExitAPIError,
			wantSubstr: "HTTP 500",
		},
		{
			name:       "502 Bad Gateway",
			statusCode: http.StatusBadGateway,
			serverURL:  "https://mm.example.com",
			err:        errors.New("bad gateway"),
			wantCode:   ExitAPIError,
			wantSubstr: "HTTP 502",
		},
		{
			name:       "connection failure",
			statusCode: 0,
			serverURL:  "https://mm.example.com",
			err:        errors.New("dial tcp: connection refused"),
			wantCode:   ExitAPIError,
			wantSubstr: "unable to connect to https://mm.example.com",
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			serverURL:  "https://mm.example.com",
			err:        errors.New("not found"),
			wantCode:   ExitAPIError,
			wantSubstr: "not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyAPIError(tt.statusCode, tt.serverURL, tt.err)
			if result.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", result.Code, tt.wantCode)
			}
			if !strings.Contains(result.Message, tt.wantSubstr) {
				t.Errorf("Message = %q, want substring %q", result.Message, tt.wantSubstr)
			}
		})
	}
}
