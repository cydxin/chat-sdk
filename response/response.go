package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// Response 统一响应结构
type Response struct {
	Code int         `json:"code" example:"0"`                    // 业务状态码
	Msg  string      `json:"msg" example:"success"`               // 提示消息
	Data interface{} `json:"data,omitempty" swaggertype:"object"` // 响应数据
}

// 业务状态码定义
// 使用说明：
// - 中间件层：使用 HTTP 状态码（401/403/500）
// - 业务层：HTTP 200 + 业务状态码
const (
	CodeSuccess        = 0     // 成功
	CodeParamError     = 10001 // 参数错误
	CodeUserNotFound   = 10002 // 用户不存在
	CodePasswordError  = 10003 // 密码错误（登录失败）
	CodeTokenInvalid   = 10004 // Token 无效/过期
	CodePermissionDeny = 10005 // 权限不足
	CodeInternalError  = 99999 // 内部错误
)

// Success 成功响应
func Success(data interface{}, args ...string) *Response {
	msg := "success"
	for _, arg := range args {
		msg = arg
	}
	return &Response{
		Code: CodeSuccess,
		Msg:  msg,
		Data: data,
	}
}

// Error 错误响应
func Error(code int, msg string) *Response {
	return &Response{
		Code: code,
		Msg:  msg,
	}
}

// WriteJSON 写入 JSON 响应（默认 HTTP 200）
func (r *Response) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // 业务层统一返回 200
	if err := json.NewEncoder(w).Encode(r); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// WriteJSONWithStatus 写入 JSON 响应（指定 HTTP 状态码）
// 用于中间件层面的鉴权失败等场景（如 401）
func (r *Response) WriteJSONWithStatus(w http.ResponseWriter, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(r); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
