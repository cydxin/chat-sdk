package chat_sdk

import (
	"net/http"
	"strconv"

	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/service"

	"github.com/cydxin/chat-sdk/response"
	"github.com/gin-gonic/gin"
)

var _ = model.Room{}
var _ = service.RoomDTO{}
var _ = service.RoomMemberListItemDTO{}
var _ = service.GroupInfoDTO{}
var _ = service.RoomNoticeDTO{}

// -------------------- 房间（Room）相关接口 --------------------

type CreateGroupRoomReq struct {
	Name    string   `json:"name" binding:"required"`
	Members []uint64 `json:"members" binding:"required"`
}

// GinHandleCreateGroupRoom 创建群聊房间
// @Summary 创建群聊
// @Description 创建新的群聊房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body CreateGroupRoomReq true "创建参数"
// @Success 200 {object} response.Response{data=model.Room} "房间信息"
// @Failure 400 {object} response.Response "请求错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group [post]
func (c *ChatEngine) GinHandleCreateGroupRoom(ctx *gin.Context) {
	var req CreateGroupRoomReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	_, err := c.RoomService.CreateGroupRoom(req.Name, uid.(uint64), req.Members)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleCreatePrivateRoom 创建私聊房间
// @Summary 创建私聊
// @Description 创建或获取两人私聊房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param target_id query uint64 true "目标用户ID"
// @Success 200 {object} response.Response{data=model.Room} "房间信息"
// @Failure 400 {object} response.Response "请求错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/private [post]
func (c *ChatEngine) GinHandleCreatePrivateRoom(ctx *gin.Context) {
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

	room, err := c.RoomService.CreatePrivateRoom(uid.(uint64), targetID)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(room))
}

// GinHandleGetUserRooms 获取用户参与的房间列表
// @Summary 获取用户房间列表
// @Description 获取当前用户参与的所有房间
// @Tags 房间
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.RoomDTO} "房间列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/list [get]
func (c *ChatEngine) GinHandleGetUserRooms(ctx *gin.Context) {
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	rooms, err := c.RoomService.GetUserRooms(uint(uid.(uint64)))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(rooms))
}

// GinHandleGetGroupRooms 获取用户参与的群聊列表
// @Summary 获取用户群聊列表
// @Description 获取当前用户参与的所有群聊（仅 Type=2）
// @Tags 房间
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.RoomDTO} "群聊列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group/list [get]
func (c *ChatEngine) GinHandleGetGroupRooms(ctx *gin.Context) {
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	rooms, err := c.RoomService.GetGroupList(uint(uid.(uint64)))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(rooms))
}

type RoomMemberReq struct {
	RoomID  uint64   `json:"room_id" binding:"required" example:"1"`
	UserID  uint64   `json:"user_id" example:"1001"`  // remove 用
	UserIDS []uint64 `json:"user_ids" example:"1001"` // add 用（批量）
}

// GinHandleAddRoomMember 添加房间成员
// @Summary 添加房间成员
// @Description 将用户添加到房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body RoomMemberReq true "成员信息"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/add [post]
func (c *ChatEngine) GinHandleAddRoomMember(ctx *gin.Context) {
	var req RoomMemberReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err := c.MemberService.AddRoomMember(req.RoomID, req.UserIDS, uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "成员已添加",
	}))
}

// GinHandleRemoveRoomMember 移除房间成员
// @Summary 移除房间成员
// @Description 将用户从房间移除
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body RoomMemberReq true "成员信息"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/remove [post]
func (c *ChatEngine) GinHandleRemoveRoomMember(ctx *gin.Context) {
	var req RoomMemberReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err := c.MemberService.RemoveRoomMember(req.RoomID, req.UserID, uid.(uint64))

	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "成员已移除",
	}))
}

// GinHandleCheckRoomMember 检查用户是否是房间成员
// @Summary 检查房间成员
// @Description 检查用户是否是房间成员，如果不传 user_id 则检查当前用户
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Param user_id query uint64 false "用户ID (不传则查自己)"
// @Success 200 {object} response.Response{data=map[string]bool} "检查结果"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/check [get]
func (c *ChatEngine) GinHandleCheckRoomMember(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	userIDStr := ctx.Query("user_id")

	rid, err := strconv.ParseUint(roomIDStr, 10, 64)
	if err != nil || rid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id"))
		return
	}

	var targetUserID uint64
	if userIDStr != "" {
		id, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid user_id"))
			return
		}
		targetUserID = id
	} else {
		uid, exists := ctx.Get("user_id")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
			return
		}
		targetUserID = uid.(uint64)
	}

	ok, err := c.RoomService.CheckRoomMember(uint(rid), uint(targetUserID))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{"is_member": ok}))
}

// GinHandleGetRoomMemberList 获取群成员列表
// @Summary 获取群成员列表
// @Description 获取指定房间(群)成员列表，展示名按：好友备注 > 群昵称 > 用户昵称 > 用户名
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Success 200 {object} response.Response{data=[]service.RoomMemberListItemDTO} "成员列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/list [get]
func (c *ChatEngine) GinHandleGetRoomMemberList(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	rid, err := strconv.ParseUint(roomIDStr, 10, 64)
	if err != nil || rid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "不存在的房间"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	list, err := c.RoomService.GetRoomMemberList(rid, uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(list))
}

// -------------------- 群昵称（我在群里的昵称） --------------------

type SetMyGroupNicknameReq struct {
	RoomID   uint64 `json:"room_id" binding:"required" example:"1"`
	Nickname string `json:"nickname" example:"我在群里的昵称"`
}

// GinHandleSetMyGroupNickname 设置我在群里的昵称
// @Summary 设置群昵称
// @Description 设置当前用户在指定房间(群)中的昵称（room_user.nickname），仅影响自己视角
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body SetMyGroupNicknameReq true "请求参数"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/nickname [post]
func (c *ChatEngine) GinHandleSetMyGroupNickname(ctx *gin.Context) {
	var req SetMyGroupNicknameReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	if err := c.RoomService.SetMyGroupNickname(uid.(uint64), req.RoomID, req.Nickname); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}

// -------------------- 群设置相关接口 --------------------

type UpdateGroupInfoReq struct {
	RoomID uint64 `json:"room_id" binding:"required"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type SetGroupAdminReq struct {
	RoomID       uint64 `json:"room_id" binding:"required"`
	TargetUserID uint64 `json:"target_user_id" binding:"required"`
	IsAdmin      bool   `json:"is_admin"`
}

type SetGroupMuteReq struct {
	RoomID          uint64 `json:"room_id" binding:"required"`
	DurationMinutes int    `json:"duration_minutes"` // 0 to cancel
}

type SetGroupMuteScheduledReq struct {
	RoomID          uint64 `json:"room_id" binding:"required"`
	StartTime       string `json:"start_time" binding:"required"` // HH:MM
	DurationMinutes int    `json:"duration_minutes" binding:"required"`
}

type SetUserMuteReq struct {
	RoomID          uint64 `json:"room_id" binding:"required"`
	TargetUserID    uint64 `json:"target_user_id" binding:"required"`
	DurationMinutes int    `json:"duration_minutes"` // 0 to cancel
}

// GinHandleUpdateGroupInfo 更新群信息
// @Summary 更新群信息
// @Tags Room
// @Accept json
// @Produce json
// @Param req body UpdateGroupInfoReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/group/update [post]
func (c *ChatEngine) GinHandleUpdateGroupInfo(ctx *gin.Context) {
	var req UpdateGroupInfoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.UpdateGroupInfo(uid.(uint64), req.RoomID, req.Name, req.Avatar); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleSetGroupAdmin 设置管理员
// @Summary 设置管理员
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetGroupAdminReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/admin/set [post]
func (c *ChatEngine) GinHandleSetGroupAdmin(ctx *gin.Context) {
	var req SetGroupAdminReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.SetGroupAdmin(uid.(uint64), req.RoomID, req.TargetUserID, req.IsAdmin); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleSetGroupMute 设置群禁言（倒计时）
// @Summary 设置群禁言（倒计时）
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetGroupMuteReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/group [post]
func (c *ChatEngine) GinHandleSetGroupMute(ctx *gin.Context) {
	var req SetGroupMuteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.SetGroupMuteCountdown(uid.(uint64), req.RoomID, req.DurationMinutes); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleSetGroupMuteScheduled 设置群禁言（定时）
// @Summary 设置群禁言（定时）
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetGroupMuteScheduledReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/group/scheduled [post]
func (c *ChatEngine) GinHandleSetGroupMuteScheduled(ctx *gin.Context) {
	var req SetGroupMuteScheduledReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.SetGroupMuteScheduled(uid.(uint64), req.RoomID, req.StartTime, req.DurationMinutes); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleSetUserMute 设置用户禁言
// @Summary 设置用户禁言
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetUserMuteReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/user [post]
func (c *ChatEngine) GinHandleSetUserMute(ctx *gin.Context) {
	var req SetUserMuteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.SetUserMute(uid.(uint64), req.RoomID, req.TargetUserID, req.DurationMinutes); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleCancelGroupMute 取消群禁言（倒计时）
// @Summary 取消群禁言（倒计时）
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetGroupMuteReq true "请求参数（room_id 必填，duration_minutes 忽略）"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/group/cancel [post]
func (c *ChatEngine) GinHandleCancelGroupMute(ctx *gin.Context) {
	var req SetGroupMuteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.CancelGroupMuteCountdown(uid.(uint64), req.RoomID); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleCancelGroupMuteScheduled 取消群禁言（定时）
// @Summary 取消群禁言（定时）
// @Tags Room
// @Accept json
// @Produce json
// @Param req body SetGroupMuteReq true "请求参数（room_id 必填）"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/group/scheduled/cancel [post]
func (c *ChatEngine) GinHandleCancelGroupMuteScheduled(ctx *gin.Context) {
	var req SetGroupMuteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.CancelGroupMuteScheduled(uid.(uint64), req.RoomID); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

type CancelUserMuteReq struct {
	RoomID       uint64 `json:"room_id" binding:"required"`
	TargetUserID uint64 `json:"target_user_id" binding:"required"`
}

// GinHandleCancelUserMute 取消用户禁言
// @Summary 取消用户禁言
// @Tags Room
// @Accept json
// @Produce json
// @Param req body CancelUserMuteReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /room/mute/user/cancel [post]
func (c *ChatEngine) GinHandleCancelUserMute(ctx *gin.Context) {
	var req CancelUserMuteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.RoomService.CancelUserMute(uid.(uint64), req.RoomID, req.TargetUserID); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleGetGroupInfo 获取群基础信息
// @Summary 获取群基础信息
// @Description 根据 room_id 获取群聊基础信息（不含成员列表）
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "群ID(房间ID)"
// @Success 200 {object} response.Response{data=service.GroupInfoDTO} "群信息"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group/info [get]
func (c *ChatEngine) GinHandleGetGroupInfo(ctx *gin.Context) {
	ridStr := ctx.Query("room_id")
	rid, err := strconv.ParseUint(ridStr, 10, 64)
	if err != nil || rid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id"))
		return
	}

	info, err := c.RoomService.GetGroupInfo(rid)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(info))
}

// GinHandleQuitGroup  退出群聊
// @Summary 退出指定群聊
// @Description
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "群ID(房间ID)"
// @Success 200 {object} response.Response "群信息"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group/quit [get]
func (c *ChatEngine) GinHandleQuitGroup(ctx *gin.Context) {
	ridStr := ctx.Query("room_id")
	rid, err := strconv.ParseUint(ridStr, 10, 64)
	if err != nil || rid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id"))
		return
	}
	uidStr, _ := ctx.Get("user_id")
	uid := uidStr.(uint64)
	err = c.RoomService.QuitGroup(rid, uid)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// -------------------- 群公告（Room Notice） --------------------

type CreateRoomNoticeReq struct {
	RoomID   uint64 `json:"room_id" binding:"required" example:"1"`
	Title    string `json:"title" example:"公告标题"`
	Content  string `json:"content" binding:"required" example:"公告内容"`
	IsPinned bool   `json:"is_pinned" example:"false"`
}

// GinHandleCreateRoomNotice 发布群公告
// @Summary 发布群公告
// @Description 群主/管理员发布一条群公告，并通过通知服务推送给群成员
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body CreateRoomNoticeReq true "公告内容"
// @Success 200 {object} response.Response{data=service.RoomNoticeDTO}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Security BearerAuth
// @Router /room/notice/create [post]
func (c *ChatEngine) GinHandleCreateRoomNotice(ctx *gin.Context) {
	var req CreateRoomNoticeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if c.RoomNoticeService == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, "RoomNoticeService not initialized"))
		return
	}
	out, err := c.RoomNoticeService.CreateNotice(req.RoomID, uid.(uint64), req.Title, req.Content, req.IsPinned)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(out))
}

type ListRoomNoticeReq struct {
	RoomID uint64 `json:"room_id" binding:"required" example:"1"`
	Limit  int    `json:"limit" example:"20"`
}

// GinHandleListRoomNotices 获取群公告列表
// @Summary 获取群公告列表
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body ListRoomNoticeReq true "请求参数"
// @Success 200 {object} response.Response{data=[]service.RoomNoticeDTO}
// @Security BearerAuth
// @Router /room/notice/list [post]
func (c *ChatEngine) GinHandleListRoomNotices(ctx *gin.Context) {
	var req ListRoomNoticeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	if c.RoomNoticeService == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, "RoomNoticeService not initialized"))
		return
	}
	list, err := c.RoomNoticeService.ListNotices(req.RoomID, req.Limit)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(list))
}

type DeletedRRoomNoticeReq struct {
	RoomIDS []uint64 `json:"room_ids" binding:"required" `
}

// GinHandleDeleteRoomNotices 删除群公告
// @Summary 删除指定群公告
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body DeletedRRoomNoticeReq true "请求参数"
// @Success 200 {object} response.Response{}
// @Security BearerAuth
// @Router /room/notice/delete [post]
func (c *ChatEngine) GinHandleDeleteRoomNotices(ctx *gin.Context) {
	var req DeletedRRoomNoticeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	if c.RoomNoticeService == nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, "房间通知服务未开启"))
		return
	}
	err := c.RoomNoticeService.DeleteNotices(req.RoomIDS)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, nil)
}
