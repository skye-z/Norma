package errs

import "net/http"

type Severity string

const (
	SeverityUser   Severity = "user"
	SeverityRetry  Severity = "retryable"
	SeverityManual Severity = "manual_intervention"
)

type Error struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Severity    Severity `json:"severity"`
	Retryable   bool     `json:"retryable"`
	UserVisible bool     `json:"user_visible"`
	Status      int      `json:"-"`
}

func (e *Error) Error() string { return e.Message }

func New(code, message string, status int, severity Severity, retryable bool, userVisible bool) *Error {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return &Error{Code: code, Message: message, Status: status, Severity: severity, Retryable: retryable, UserVisible: userVisible}
}

func Invalid(message string) *Error {
	return New("invalid_request", message, http.StatusBadRequest, SeverityUser, false, true)
}
func Forbidden(message string) *Error {
	return New("forbidden", message, http.StatusForbidden, SeverityUser, false, true)
}
func Internal(code, message string) *Error {
	return New(code, message, http.StatusInternalServerError, SeverityManual, false, false)
}
func Retryable(code, message string) *Error {
	return New(code, message, http.StatusBadGateway, SeverityRetry, true, true)
}
