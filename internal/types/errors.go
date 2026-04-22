package types

import (
	"fmt"
	"strings"
)

type SessionError struct {
	Msg   string
	Cause error
}

func (e *SessionError) Error() string { return e.Msg }
func (e *SessionError) Unwrap() error { return e.Cause }

type BrowserError struct {
	Msg   string
	Cause error
}

func (e *BrowserError) Error() string { return e.Msg }
func (e *BrowserError) Unwrap() error { return e.Cause }

type UserDisplayableError struct {
	Msg string
}

func (e *UserDisplayableError) Error() string { return e.Msg }

func NewUserDisplayableError(raw string) *UserDisplayableError {
	return &UserDisplayableError{Msg: extractUserMessage(raw)}
}

func extractUserMessage(raw string) string {
	if strings.Contains(raw, "[[null,[[1]]]]") {
		return "Quota exceeded or generation limit reached"
	}
	if strings.Contains(raw, "[[null,[[2]]]]") {
		return "Rate limited — try again later"
	}
	return "Server error: operation rejected by NotebookLM"
}

func NewSessionError(msg string, cause error) *SessionError {
	return &SessionError{Msg: msg, Cause: cause}
}

func NewBrowserError(msg string, cause error) *BrowserError {
	return &BrowserError{Msg: msg, Cause: cause}
}

func WrapSession(cause error, format string, args ...any) *SessionError {
	return &SessionError{Msg: fmt.Sprintf(format, args...), Cause: cause}
}
