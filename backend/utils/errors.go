package config

import "net/http"

// 错误严重级别
type Severity string

const (
	SeverityUser   Severity = "user"                // 用户输入错误
	SeverityRetry  Severity = "retryable"           // 可重试错误
	SeverityManual Severity = "manual_intervention" // 需人工介入
)

// 应用错误
type Error struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Severity    Severity `json:"severity"`
	Retryable   bool     `json:"retryable"`
	UserVisible bool     `json:"user_visible"`
	Status      int      `json:"-"`
}

func (e *Error) Error() string { return e.Message }

// 创建错误
func New(code, message string, status int, severity Severity, retryable bool, userVisible bool) *Error {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return &Error{Code: code, Message: message, Status: status, Severity: severity, Retryable: retryable, UserVisible: userVisible}
}

// 客户端错误 400
func Invalid(message string) *Error {
	return New("invalid_request", message, http.StatusBadRequest, SeverityUser, false, true)
}

// 禁止访问 403
func Forbidden(message string) *Error {
	return New("forbidden", message, http.StatusForbidden, SeverityUser, false, true)
}

// 服务器错误 500
func Internal(code, message string) *Error {
	return New(code, message, http.StatusInternalServerError, SeverityManual, false, false)
}

// 可重试错误 502
func Retryable(message string) *Error {
	return New("retryable", message, http.StatusBadGateway, SeverityRetry, true, true)
}
