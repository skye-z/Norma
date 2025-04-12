// Package config 提供错误类型定义和错误创建工具
// 用于统一管理应用程序中的错误信息，包括错误码、严重级别、可重试性等属性
package config

import "net/http"

// Severity 定义错误的严重级别，用于错误分类和后续处理
type Severity string

// 错误严重级别常量
const (
	SeverityUser   Severity = "user"                // 用户级别错误：客户端输入问题
	SeverityRetry  Severity = "retryable"           // 可重试错误：临时性故障，可重试
	SeverityManual Severity = "manual_intervention" // 需人工介入错误：需要运维人员处理
)

// Error 结构体定义应用程序错误的完整信息
// 包含错误码、消息、严重级别、可重试标记、用户可见性及HTTP状态码
type Error struct {
	Code        string   `json:"code"`         // 错误码，如 invalid_request
	Message     string   `json:"message"`      // 错误描述信息
	Severity    Severity `json:"severity"`     // 错误严重级别
	Retryable   bool     `json:"retryable"`    // 是否可重试
	UserVisible bool     `json:"user_visible"` // 是否对用户可见
	Status      int      `json:"-"`            // HTTP状态码，不暴露给JSON
}

// Error 实现 error 接口，返回错误消息
func (e *Error) Error() string { return e.Message }

// New 创建一个新的错误实例
// 参数：code-错误码，message-错误消息，status-HTTP状态码，severity-严重级别，retryable-是否可重试，userVisible-是否对用户可见
// 如果未指定status，默认使用500内部服务器错误
func New(code, message string, status int, severity Severity, retryable bool, userVisible bool) *Error {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return &Error{Code: code, Message: message, Status: status, Severity: severity, Retryable: retryable, UserVisible: userVisible}
}

// Invalid 创建客户端请求错误（400）
// 用于处理无效的用户输入或请求参数
func Invalid(message string) *Error {
	return New("invalid_request", message, http.StatusBadRequest, SeverityUser, false, true)
}

// Forbidden 创建禁止访问错误（403）
// 用于处理权限不足或被禁止访问的资源
func Forbidden(message string) *Error {
	return New("forbidden", message, http.StatusForbidden, SeverityUser, false, true)
}

// Internal 创建内部服务器错误（500）
// 用于处理服务器端故障，不对用户暴露详细信息
func Internal(code, message string) *Error {
	return New(code, message, http.StatusInternalServerError, SeverityManual, false, false)
}

// Retryable 创建可重试错误（502）
// 用于处理临时性故障，如网络超时、外部服务不可用等
func Retryable(code, message string) *Error {
	return New(code, message, http.StatusBadGateway, SeverityRetry, true, true)
}
