// Package api 提供 HTTP API 处理器
// 包含对话、健康检查、设置等核心功能的 HTTP 端点
package api

import (
	"net/http"
	"time"
)

// RegisterHealth 注册健康检查端点
// 添加 /healthz 路由，返回服务启动时间和健康状态
// 参数: mux - HTTP 多路复用器, startedAt - 服务启动时间
func RegisterHealth(mux *http.ServeMux, startedAt time.Time) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// 仅允许 GET 方法请求健康检查端点
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		// 返回服务健康状态，包含启动时间
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"started_at": startedAt.UTC().Format(time.RFC3339),
		})
	})
}
