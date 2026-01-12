package chat_sdk

import (
	"net/http"
	"strconv"
	"strings"

	model "github.com/cydxin/chat-sdk/models"

	"github.com/cydxin/chat-sdk/response"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

var _ = model.User{}

// -------------------- 用户（User）相关接口 --------------------

// GinHandleGetUserInfo 获取用户信息 (Gin 版本)
// @Summary 获取用户信息
// @Description 根据 user_id 查询用户详情，如果不传 user_id 则查询当前登录用户
// @Tags 用户
// @Accept json
// @Produce json
// @Param user_id query uint64 false "用户ID (不传则查自己)"
// @Success 200 {object} response.Response{data=service.UserDTO} "查询成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 401 {object} response.Response "未登录"
// @Security BearerAuth
// @Router /user/info [get]
func (c *ChatEngine) GinHandleGetUserInfo(ctx *gin.Context) {
	// 1. 尝试从 Query 获取目标 user_id (查别人)
	userIDStr := ctx.Query("user_id")
	var targetUserID uint64

	if userIDStr != "" {
		// 如果传了 user_id，解析它
		id, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || id == 0 {
			ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid user_id"))
			return
		}
		targetUserID = id
	} else {
		// 2. 如果没传 user_id，从 Context 获取当前用户 ID (查自己)
		// 注意：这需要配合 GinAuthMiddleware 使用
		currentUserID, exists := ctx.Get("user_id")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found in context"))
			return
		}

		// 类型断言
		switch v := currentUserID.(type) {
		case uint64:
			targetUserID = v
		case float64: // 有些 JSON 解析可能会变成 float64
			targetUserID = uint64(v)
		case int:
			targetUserID = uint64(v)
		default:
			// 尝试转字符串再转数字，或者直接报错
			ctx.JSON(http.StatusInternalServerError, response.Error(response.CodeInternalError, "invalid user_id type"))
			return
		}
	}

	// 3. 调用 Service 查询
	u, err := c.UserService.GetUser(targetUserID)
	if err != nil {
		// 区分一下错误类型可能更好，这里简单处理
		ctx.JSON(http.StatusOK, response.Error(response.CodeUserNotFound, err.Error()))
		return
	}

	// 4. 返回结果
	ctx.JSON(http.StatusOK, response.Success(u))
}

// GinHandleUserRegister 用户注册
// @Summary 用户注册
// @Description 创建新用户账号：username + (phone/email 二选一) + password + code + nickname
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.RegisterReq true "注册信息"
// @Success 200 {object} response.Response "注册成功"
// @Failure 400 {object} response.Response "请求错误"
// @Router /user/register [post]
func (c *ChatEngine) GinHandleUserRegister(ctx *gin.Context) {
	var req service.RegisterReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	if c.config == nil || c.config.RDB == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeRedisNotConfigured, "r 服务暂未开启"))
		return
	}

	err := c.UserService.Register(ctx.Request.Context(), req)
	if err != nil {
		code := response.CodeInternalError
		switch {
		case strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "cannot"):
			code = response.CodeParamError
		case strings.Contains(err.Error(), "verification code"):
			code = response.CodeVerifyCodeInvalid
		case strings.Contains(err.Error(), "存在"):
			code = response.CodeUserAlreadyExists
		case strings.Contains(err.Error(), "redis"):
			code = response.CodeRedisNotConfigured
		}
		ctx.JSON(http.StatusOK, response.Error(code, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleUserLogin 用户登录
// @Summary 用户登录
// @Description 用户登录并返回 token（account 支持 username/phone/email；password 或 code 二选一）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.LoginReq true "登录信息"
// @Success 200 {object} response.Response{data=service.LoginResp} "登录响应（token + 用户信息）"
// @Failure 401 {object} response.Response "认证失败"
// @Router /user/login [post]
func (c *ChatEngine) GinHandleUserLogin(ctx *gin.Context) {
	var req service.LoginReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeParamError, err.Error()))
		return
	}

	resp, err := c.UserService.LoginWithToken(ctx.Request.Context(), req)
	if err != nil {
		code := response.CodePasswordError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "cannot") {
			code = response.CodeParamError
		} else if strings.Contains(err.Error(), "verification code") {
			code = response.CodeVerifyCodeInvalid
		}
		ctx.JSON(http.StatusOK, response.Error(code, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(resp))
}

// --- 验证码 ---

type SendVerifyCodeReq struct {
	Purpose    string `json:"purpose" binding:"required" example:"register"`       // register/forgot_password
	Identifier string `json:"identifier" binding:"required" example:"13800138000"` // 手机号或邮箱
}

// GinHandleSendVerifyCode 发送验证码（写入 Redis；实际短信/邮件发送由调用方对接）
// @Summary 发送验证码
// @Description 发送验证码到手机号/邮箱（identifier=手机号/邮箱），purpose=register/forgot_password
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body SendVerifyCodeReq true "发送验证码请求"
// @Success 200 {object} response.Response{data=service.SendCodeResult} "发送成功"
// @Failure 400 {object} response.Response "请求错误"
// @Router /user/code/send [post]
func (c *ChatEngine) GinHandleSendVerifyCode(ctx *gin.Context) {
	var req SendVerifyCodeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	if c.config == nil || c.config.RDB == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeRedisNotConfigured, "r 服务暂未开启"))
		return
	}

	purpose := service.VerifyCodePurpose(strings.TrimSpace(req.Purpose))
	svc := service.NewVerifyCodeService(c.config.RDB)
	ret, err := svc.SendCode(ctx.Request.Context(), purpose, req.Identifier)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	// 非 Debug 环境不返回验证码
	if c.config == nil || !c.config.Service.Debug {
		ret.Code = ""
	}
	ctx.JSON(http.StatusOK, response.Success(ret))
}

// --- 忘记密码 ---

// GinHandleForgotPassword 忘记密码
// @Summary 忘记密码
// @Description 通过验证码重置密码（identifier 支持 phone/email/username；推荐 phone/email）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.ForgotPasswordReq true "忘记密码请求"
// @Success 200 {object} response.Response "重置成功"
// @Failure 400 {object} response.Response "请求错误"
// @Router /user/password/forgot [post]
func (c *ChatEngine) GinHandleForgotPassword(ctx *gin.Context) {
	var req service.ForgotPasswordReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	if c.config == nil || c.config.RDB == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeRedisNotConfigured, "r 服务暂未开启"))
		return
	}

	err := c.UserService.ForgotPassword(ctx.Request.Context(), req)
	if err != nil {
		code := response.CodeInternalError
		switch {
		case strings.Contains(err.Error(), "required"):
			code = response.CodeParamError
		case strings.Contains(err.Error(), "verification code"):
			code = response.CodeVerifyCodeInvalid
		case strings.Contains(err.Error(), "not found"):
			code = response.CodeUserNotFound
		case strings.Contains(err.Error(), "redis"):
			code = response.CodeRedisNotConfigured
		}
		ctx.JSON(http.StatusOK, response.Error(code, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]any{"message": "密码已重置"}))
}

// GinHandleUpdateUserInfo 更新用户信息
// @Summary 更新用户信息
// @Description 更新当前用户资料（昵称/签名/性别等）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.UpdateUserReq true "更新信息（可选字段）"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/update [post]
func (c *ChatEngine) GinHandleUpdateUserInfo(ctx *gin.Context) {

	var req service.UpdateUserReq
	// 对
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	u, err := c.UserService.UpdateUser(uid.(uint64), req)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(u))
}

type UpdateUserAvatarReq struct {
	Avatar string `json:"avatar" binding:"required" example:"https://example.com/avatar.jpg"`
}

// GinHandleUpdateUserAvatar 更新用户头像
// @Summary 更新用户头像
// @Description 更新当前用户头像 URL
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body UpdateUserAvatarReq true "头像更新"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/avatar [post]
func (c *ChatEngine) GinHandleUpdateUserAvatar(ctx *gin.Context) {
	var req UpdateUserAvatarReq

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	u, err := c.UserService.UpdateAvatar(uid.(uint64), req.Avatar)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(u))
}

type UpdateUserPasswordReq struct {
	OldPassword string `json:"old_password" binding:"required" example:"123456"`
	NewPassword string `json:"new_password" binding:"required" example:"123456"`
}

// GinHandleUpdateUserPassword 修改密码
// @Summary 修改用户密码
// @Description 修改当前用户密码（验证码/鉴权由调用层保证）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body UpdateUserPasswordReq true "密码修改"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/password [post]
func (c *ChatEngine) GinHandleUpdateUserPassword(ctx *gin.Context) {
	var req UpdateUserPasswordReq

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "用户未找到"))
		return
	}

	if strings.TrimSpace(req.NewPassword) == "" {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "新密码必填"))
		return
	}

	if strings.TrimSpace(req.OldPassword) == "" {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "旧密码必填"))
		return
	}

	if err := c.UserService.UpdatePassword(uid.(uint64), req.NewPassword, req.OldPassword); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "密码已更新",
	}))
}

// GinHandleSearchUsers 搜索用户
// @Summary 搜索用户
// @Description 按关键字搜索用户（username/nickname/uid），自动排除当前用户
// @Tags 用户
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键字"
// @Param limit query int false "返回条数"
// @Param offset query int false "偏移量"
// @Success 200 {object} response.Response{data=[]service.UserDTO} "用户列表"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /user/search [get]
func (c *ChatEngine) GinHandleSearchUsers(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	limitStr := ctx.Query("limit")
	offsetStr := ctx.Query("offset")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	var excludeID uint64
	if uid, exists := ctx.Get("user_id"); exists {
		excludeID = uid.(uint64)
	}

	users, err := c.UserService.SearchUsers(keyword, excludeID, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(users))
}
