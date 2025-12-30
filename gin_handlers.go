package chat_sdk

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/cydxin/chat-sdk/response"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

// @title           Chat SDK API
// @version         1.0
// @description     Chat SDK API documentation
// @host            localhost:6789
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

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

// GinHandleRecallMessage 撤回消息
// @Summary 撤回消息
// @Description 撤回指定消息
// @Tags 消息
// @Accept json
// @Produce json
// @Param message_id query uint64 true "消息ID"
// @Param status query uint8 true "4-撤回（会在聊天窗口留下痕迹） 5-删除（自己不可见） 6/7-双删（Sender/非Sender删除)在私聊中互相可以删除，但在群中你只能删除自己的，已经管理员进行删除 "
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/recall [post]
func (c *ChatEngine) GinHandleRecallMessage(ctx *gin.Context) {
	msgIDStr := ctx.Query("message_id")
	msgStatusStr := ctx.Query("status")
	msgID, err := strconv.ParseUint(msgIDStr, 10, 64)
	msgStatus, err := strconv.ParseInt(msgStatusStr, 10, 64)
	if err != nil || msgID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid message_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err = c.MsgService.RecallMessage(msgID, uid.(uint64), uint8(msgStatus))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"message": "消息已撤回",
	}))
}

// GinHandleGetRoomMessages 获取房间消息列表
// @Summary 获取房间消息
// @Description 分页获取房间历史消息
// @Tags 消息
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Param limit query int false "每页数量"
// @Param offset query int false "偏移量"
// @Success 200 {object} response.Response{data=[]service.MessageDTO} "消息列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/list [get]
func (c *ChatEngine) GinHandleGetRoomMessages(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	limitStr := ctx.Query("limit")
	offsetStr := ctx.Query("offset")

	roomID, _ := strconv.ParseUint(roomIDStr, 10, 64)
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit == 0 {
		limit = 20
	}

	messages, err := c.MsgService.GetRoomMessages(roomID, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(messages))
}

// GinHandleGetMessageByID 根据 message_id 获取消息
// @Summary 获取消息详情
// @Description 根据消息ID获取消息详情
// @Tags 消息
// @Accept json
// @Produce json
// @Param message_id query uint64 true "消息ID"
// @Success 200 {object} response.Response{data=service.MessageDTO} "消息详情"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/detail [get]
func (c *ChatEngine) GinHandleGetMessageByID(ctx *gin.Context) {
	msgIDStr := ctx.Query("message_id")
	mid, err := strconv.ParseUint(msgIDStr, 10, 64)
	if err != nil || mid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid message_id"))
		return
	}

	msg, err := c.MsgService.GetMessageByID(mid)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(msg))
}

// GinHandleSendFriendRequest 发送好友申请
// @Summary 发送好友申请
// @Description 向目标用户发送好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body object true "好友申请（to_user, message）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/request [post]
func (c *ChatEngine) GinHandleSendFriendRequest(ctx *gin.Context) {
	var req struct {
		ToUser  uint64 `json:"to_user"`
		Message string `json:"message"`
	}

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
// @Success 200 {object} response.Response{data=[]model.Friend} "好友列表"
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

// GinHandleGetPendingRequests 获取待处理的好友申请
// @Summary 获取待处理好友申请
// @Description 获取当前用户待处理的好友申请列表
// @Tags 好友
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]model.FriendApply} "好友申请列表"
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

// GinHandleCreateGroupRoom 创建群聊房间
// @Summary 创建群聊
// @Description 创建新的群聊房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body object true "群聊信息（name, members）"
// @Success 200 {object} response.Response{data=model.Room} "房间信息"
// @Failure 400 {object} response.Response "请求错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group [post]
func (c *ChatEngine) GinHandleCreateGroupRoom(ctx *gin.Context) {
	var req struct {
		Name    string   `json:"name"`
		Members []uint64 `json:"members"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	room, err := c.RoomService.CreateGroupRoom(req.Name, uid.(uint64), req.Members)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(room))
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
// @Success 200 {object} response.Response{data=[]model.Room} "房间列表"
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

// GinHandleAddRoomMember 添加房间成员
// @Summary 添加房间成员
// @Description 将用户添加到房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/add [post]
func (c *ChatEngine) GinHandleAddRoomMember(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	userIDStr := ctx.Query("user_id")

	roomID, err1 := strconv.ParseUint(roomIDStr, 10, 64)
	userID, err2 := strconv.ParseUint(userIDStr, 10, 64)

	if err1 != nil || err2 != nil || roomID == 0 || userID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id or user_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err := c.MemberService.AddRoomMember(roomID, userID, uid.(uint64))
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
// @Param room_id query uint64 true "房间ID"
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/remove [post]
func (c *ChatEngine) GinHandleRemoveRoomMember(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	userIDStr := ctx.Query("user_id")

	roomID, err1 := strconv.ParseUint(roomIDStr, 10, 64)
	userID, err2 := strconv.ParseUint(userIDStr, 10, 64)

	if err1 != nil || err2 != nil || roomID == 0 || userID == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id or user_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	err := c.MemberService.RemoveRoomMember(roomID, userID, uid.(uint64))
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

// GinHandleUserRegister 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
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

	err := c.UserService.Register(req)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleUserLogin 用户登录
// @Summary 用户登录
// @Description 用户登录并返回 token
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
		ctx.JSON(http.StatusOK, response.Error(response.CodePasswordError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(resp))
}

// GinHandleUpdateUserInfo 更新用户信息
// @Summary 更新用户信息
// @Description 更新当前用户资料（昵称/签名/性别等）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "更新信息（可选字段）"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/update [post]
func (c *ChatEngine) GinHandleUpdateUserInfo(ctx *gin.Context) {
	var req service.UpdateUserReq

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
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

// GinHandleUpdateUserAvatar 更新用户头像
// @Summary 更新用户头像
// @Description 更新当前用户头像 URL
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "头像更新（avatar）"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/avatar [post]
func (c *ChatEngine) GinHandleUpdateUserAvatar(ctx *gin.Context) {
	var req struct {
		Avatar string `json:"avatar"`
	}

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

// GinHandleUpdateUserPassword 修改密码
// @Summary 修改用户密码
// @Description 修改当前用户密码（验证码/鉴权由调用层保证）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "密码修改（new_password）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/password [post]
func (c *ChatEngine) GinHandleUpdateUserPassword(ctx *gin.Context) {
	var req struct {
		NewPassword string `json:"new_password"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	if strings.TrimSpace(req.NewPassword) == "" {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "new_password is required"))
		return
	}

	if err := c.UserService.UpdatePassword(uid.(uint64), req.NewPassword); err != nil {
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

// GinHandleMemberSearchUsers 搜索用户 (MemberService版本)
// @Summary 搜索用户 (Member)
// @Description 搜索用户，返回ID列表
// @Tags 用户
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键字"
// @Param limit query int false "返回条数"
// @Success 200 {array} uint64 "用户ID列表"
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

	ids, err := c.MemberService.SearchUsers(keyword, curID, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, ids)
}

// -------------------- 朋友圈（Moment）相关接口 --------------------

// GinHandleCreateMoment 发布动态
// @Summary 发布朋友圈动态
// @Description 标题 + 图片(最多9张) 或 视频(1个)
// @Tags 朋友圈
// @Accept json
// @Produce json
// @Param req body service.CreateMomentReq true "动态内容（title, images(最多9) 或 video 二选一）"
// @Success 200 {object} response.Response{data=service.MomentDTO} "创建成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 401 {object} response.Response "未登录"
// @Security BearerAuth
// @Router /moment/create [post]
func (c *ChatEngine) GinHandleCreateMoment(ctx *gin.Context) {
	var req service.CreateMomentReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	dto, err := c.MomentService.CreateMoment(uid.(uint64), req)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(dto))
}

// GinHandleListFriendMoments 动态列表（自己 + 好友）
// @Summary 朋友圈动态列表
// @Description 获取自己与好友发布的动态（按时间倒序）
// @Tags 朋友圈
// @Accept json
// @Produce json
// @Param limit query int false "每页数量"
// @Param offset query int false "偏移量"
// @Success 200 {object} response.Response{data=[]service.MomentDTO} "动态列表"
// @Failure 401 {object} response.Response "未登录"
// @Security BearerAuth
// @Router /moment/list [get]
func (c *ChatEngine) GinHandleListFriendMoments(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	offset, _ := strconv.Atoi(ctx.Query("offset"))

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	list, err := c.MomentService.ListFriendMoments(uid.(uint64), limit, offset)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(list))
}

// GinHandleAddMomentComment 新增评论/回复
// @Summary 发表评论或回复
// @Description 对动态发表评论，或对评论进行二级回复
// @Tags 朋友圈
// @Accept json
// @Produce json
// @Param req body object true "评论内容（moment_id, content, parent_id 可选）"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 401 {object} response.Response "未登录"
// @Security BearerAuth
// @Router /moment/comment/add [post]
func (c *ChatEngine) GinHandleAddMomentComment(ctx *gin.Context) {
	var req struct {
		MomentID uint64  `json:"moment_id"`
		Content  string  `json:"content"`
		ParentID *uint64 `json:"parent_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	if req.MomentID == 0 || strings.TrimSpace(req.Content) == "" {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "moment_id 与 content 必填"))
		return
	}

	if err := c.MomentService.AddComment(uid.(uint64), req.MomentID, req.Content, req.ParentID); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(map[string]interface{}{"message": "评论成功"}))
}

// GinHandleListMomentComments 评论列表
// @Summary 获取评论列表
// @Description 获取某条动态下的评论（时间升序）
// @Tags 朋友圈
// @Accept json
// @Produce json
// @Param moment_id query uint64 true "动态ID"
// @Param limit query int false "每页数量"
// @Param offset query int false "偏移量"
// @Success 200 {object} response.Response{data=[]service.CommentDTO} "评论列表"
// @Security BearerAuth
// @Router /moment/comment/list [get]
func (c *ChatEngine) GinHandleListMomentComments(ctx *gin.Context) {
	midStr := ctx.Query("moment_id")
	mid, err := strconv.ParseUint(midStr, 10, 64)
	if err != nil || mid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid moment_id"))
		return
	}
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	offset, _ := strconv.Atoi(ctx.Query("offset"))

	list, err := c.MomentService.ListComments(mid, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(list))
}
