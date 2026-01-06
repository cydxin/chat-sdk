package chat_sdk

import (
	"net/http"
	"strconv"

	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/service"

	"github.com/cydxin/chat-sdk/response"
	"github.com/gin-gonic/gin"
)

var _ = model.Friend{}
var _ = service.FriendApplyDTO{}

// -------------------- 好友（Friend）相关接口 --------------------

type SendFriendRequestReq struct {
	ToUser  uint64 `json:"to_user" binding:"required" example:"1001"`
	Message string `json:"message" example:"你好，交个朋友"`
}

// GinHandleSendFriendRequest 发送好友申请
// @Summary 发送好友申请
// @Description 向目标用户发送好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body SendFriendRequestReq true "好友申请"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/request [post]
func (c *ChatEngine) GinHandleSendFriendRequest(ctx *gin.Context) {
	var req SendFriendRequestReq

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err := c.MemberService.SendFriendRequest(uid.(uint64), req.ToUser, req.Message)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{}, "好友申请已发送"))
}

// GinHandleAcceptFriendRequest 同意好友申请
// @Summary 同意好友申请
// @Description 同意指定的好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param request_id query uint64 true "申请ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/accept [post]
func (c *ChatEngine) GinHandleAcceptFriendRequest(ctx *gin.Context) {
	reqIDStr := ctx.Query("request_id")
	reqID, err := strconv.ParseUint(reqIDStr, 10, 64)
	if err != nil || reqID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid request_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err = c.MemberService.AcceptFriendRequest(reqID, uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "已同意好友申请",
	}))
}

// GinHandleRejectFriendRequest 拒绝好友申请
// @Summary 拒绝好友申请
// @Description 拒绝指定的好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param request_id query uint64 true "申请ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/reject [post]
func (c *ChatEngine) GinHandleRejectFriendRequest(ctx *gin.Context) {
	reqIDStr := ctx.Query("request_id")
	reqID, err := strconv.ParseUint(reqIDStr, 10, 64)
	if err != nil || reqID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid request_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err = c.MemberService.RejectFriendRequest(reqID, uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "已拒绝好友申请",
	}))
}

// GinHandleDeleteFriend 删除好友
// @Summary 删除好友
// @Description 删除好友关系
// @Tags 好友
// @Accept json
// @Produce json
// @Param friend_id query uint64 true "好友ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/delete [post]
func (c *ChatEngine) GinHandleDeleteFriend(ctx *gin.Context) {
	friendIDStr := ctx.Query("friend_id")
	friendID, err := strconv.ParseUint(friendIDStr, 10, 64)
	if err != nil || friendID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid friend_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err = c.MemberService.DeleteFriend(uid.(uint64), friendID)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "已删除好友",
	}))
}

// GinHandleGetFriendList 获取好友列表
// @Summary 获取好友列表
// @Description 获取当前用户的好友列表
// @Tags 好友
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.UserDTO} "好友列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/list [get]
func (c *ChatEngine) GinHandleGetFriendList(ctx *gin.Context) {
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	friends, err := c.MemberService.GetFriendList(uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(friends))
}

// GinHandleGetPendingRequests 获取好友申请
// @Summary 获取好友申请
// @Description 获取当前用户的好友申请列表
// @Tags 好友
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.FriendApplyDTO} "好友申请列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/pending [get]
func (c *ChatEngine) GinHandleGetPendingRequests(ctx *gin.Context) {
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	requests, err := c.MemberService.GetPendingRequests(uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(requests))
}

// GinHandleCheckFriendship 检查是否好友
// @Summary 检查好友关系
// @Description 检查当前用户与目标用户是否是好友
// @Tags 好友
// @Accept json
// @Produce json
// @Param target_id query uint64 true "目标用户 ID"
// @Success 200 {object} response.Response{data=map[string]bool} "好友关系检查结果"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/check [get]
func (c *ChatEngine) GinHandleCheckFriendship(ctx *gin.Context) {
	targetIDStr := ctx.Query("target_id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 64)
	if err != nil || targetID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid target_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	ok, err := c.MemberService.CheckFriendship(uid.(uint64), targetID)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{"is_friend": ok}))
}

// GinHandleMemberSearchUsers 搜索用户 (MemberService版本)
// @Summary 搜索用户 (Member)
// @Description 搜索用户，返回用户基本信息列表（用于添加好友等场景）
// @Tags 用户
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键字"
// @Param limit query int false "返回条数"
// @Success 200 {object} response.Response{data=[]service.UserBasicDTO} "用户列表"
// @Failure 500 {string} string "服务器错误"
// @Security BearerAuth
// @Router /member/search [get]
func (c *ChatEngine) GinHandleMemberSearchUsers(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	limitStr := ctx.Query("limit")

	limit, _ := strconv.Atoi(limitStr)

	var curID int64
	if uid, exists := ctx.Get("user_id"); exists {
		curID = int64(uid.(uint64))
	}

	users, err := c.MemberService.SearchUsers(keyword, curID, limit)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(users))
}

type SetFriendRemarkReq struct {
	FriendID uint64 `json:"friend_id" binding:"required" example:"1002"`
	Remark   string `json:"remark" example:"老板"`
}

// GinHandleSetFriendRemark 设置好友备注
// @Summary 设置好友备注
// @Description 设置当前用户对某个好友的备注（仅影响自己视角）
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body SetFriendRemarkReq true "请求参数"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/remark [post]
func (c *ChatEngine) GinHandleSetFriendRemark(ctx *gin.Context) {
	var req SetFriendRemarkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	if err := c.MemberService.SetFriendRemark(uid.(uint64), req.FriendID, req.Remark); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}
