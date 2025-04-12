// Package api 提供 HTTP API 处理器
// 包含对话、健康检查、设置等核心功能的 HTTP 端点
package api

import (
	"encoding/json"
	"net/http"

	errs "norma/backend/utils"
)

// writeJSON 向客户端写入 JSON 响应
// 参数: w - HTTP 响应写入器, code - HTTP 状态码, v - 要序列化的数据
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr 向客户端写入错误响应
// 自动将错误信息封装为标准 API 错误格式
// 参数: w - HTTP 响应写入器, code - HTTP 错误状态码, msg - 错误消息
func writeErr(w http.ResponseWriter, code int, msg string) {
	apiErr := errs.New("api_error", msg, code, errs.SeverityUser, code >= 500, code < 500)
	writeAPIError(w, apiErr)
}

// writeAPIError 向客户端写入 API 错误响应
// 处理 nil 错误的情况，转换为内部未知错误
// 参数: w - HTTP 响应写入器, apiErr - API 错误对象
func writeAPIError(w http.ResponseWriter, apiErr *errs.Error) {
	if apiErr == nil {
		apiErr = errs.Internal("unknown_error", "unexpected error")
	}
	writeJSON(w, apiErr.Status, map[string]any{"error": apiErr})
}
