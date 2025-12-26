package chat_sdk

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/response"
	"github.com/cydxin/chat-sdk/service"
)

/*
	HTTP处理 更建议自己写HTTP的处理，然后调用对应的service，而不是获得这里的闭包来调用
	实际上更直接的方法就是直接自己写接口，service里不一定通用，只使用service里的WsNotifier去做通知即可
	这样更灵活，也更符合实际业务需求
*/

// HandleRecallMessage 撤回消息的 HTTP Handler
// @Summary 撤回消息
// @Description 撤回指定消息
// @Tags 消息
// @Accept json
// @Produce json
// @Param req body object true "撤回信息（message_id, user_id）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/recall [post]
func (c *ChatEngine) HandleRecallMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MessageID uint64 `json:"message_id"`
			UserID    uint64 `json:"user_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MsgService.RecallMessage(req.MessageID, req.UserID, model.MessageStatusBothDeleted)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "消息已撤回",
		}).WriteJSON(w)
	}
}

// HandleGetRoomMessages 获取房间消息列表
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
func (c *ChatEngine) HandleGetRoomMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomIDStr := r.URL.Query().Get("room_id")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		roomID, _ := strconv.ParseUint(roomIDStr, 10, 64)
		limit, _ := strconv.Atoi(limitStr)
		offset, _ := strconv.Atoi(offsetStr)

		if limit == 0 {
			limit = 20
		}

		messages, err := c.MsgService.GetRoomMessages(roomID, limit, offset)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(messages).WriteJSON(w)
	}
}

// HandleSendFriendRequest 发送好友申请
// @Summary 发送好友申请
// @Description 向目标用户发送好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body object true "好友申请（from_user, to_user, message）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/request [post]
func (c *ChatEngine) HandleSendFriendRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FromUser uint64 `json:"from_user"`
			ToUser   uint64 `json:"to_user"`
			Message  string `json:"message"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.SendFriendRequest(req.FromUser, req.ToUser, req.Message)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "好友申请已发送",
		}).WriteJSON(w)
	}
}

// HandleAcceptFriendRequest 同意好友申请
// @Summary 同意好友申请
// @Description 同意指定的好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body object true "申请信息（request_id, user_id）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/accept [post]
func (c *ChatEngine) HandleAcceptFriendRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RequestID uint64 `json:"request_id"`
			UserID    uint64 `json:"user_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.AcceptFriendRequest(req.RequestID, req.UserID)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "已同意好友申请",
		}).WriteJSON(w)
	}
}

// HandleRejectFriendRequest 拒绝好友申请
// @Summary 拒绝好友申请
// @Description 拒绝指定的好友申请
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body object true "申请信息（request_id, user_id）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/reject [post]
func (c *ChatEngine) HandleRejectFriendRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RequestID uint64 `json:"request_id"`
			UserID    uint64 `json:"user_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.RejectFriendRequest(req.RequestID, req.UserID)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "已拒绝好友申请",
		}).WriteJSON(w)
	}
}

// HandleDeleteFriend 删除好友
// @Summary 删除好友
// @Description 删除好友关系
// @Tags 好友
// @Accept json
// @Produce json
// @Param req body object true "好友信息（user1, user2）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/delete [post]
func (c *ChatEngine) HandleDeleteFriend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			User1 uint64 `json:"user1"`
			User2 uint64 `json:"user2"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.DeleteFriend(req.User1, req.User2)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "已删除好友",
		}).WriteJSON(w)
	}
}

// HandleGetFriendList 获取好友列表
// @Summary 获取好友列表
// @Description 获取用户的好友列表
// @Tags 好友
// @Accept json
// @Produce json
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response{data=[]model.Friend} "好友列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/list [get]
func (c *ChatEngine) HandleGetFriendList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		friends, err := c.MemberService.GetFriendList(uint64(userID))
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(friends).WriteJSON(w)
	}
}

// HandleGetPendingRequests 获取待处理的好友申请
// @Summary 获取待处理好友申请
// @Description 获取用户待处理的好友申请列表
// @Tags 好友
// @Accept json
// @Produce json
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response{data=[]model.FriendApply} "好友申请列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/pending [get]
func (c *ChatEngine) HandleGetPendingRequests() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		requests, err := c.MemberService.GetPendingRequests(uint64(userID))
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(requests).WriteJSON(w)
	}
}

// HandleAddRoomMember 添加房间成员
// @Summary 添加房间成员
// @Description 将用户添加到房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body object true "成员信息（room_id, user_id, operator_id）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/add [post]
func (c *ChatEngine) HandleAddRoomMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RoomID     uint64 `json:"room_id"`
			UserID     uint64 `json:"user_id"`
			OperatorID uint64 `json:"operator_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.AddRoomMember(req.RoomID, req.UserID, req.OperatorID)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "成员已添加",
		}).WriteJSON(w)
	}
}

// HandleRemoveRoomMember 移除房间成员
// @Summary 移除房间成员
// @Description 将用户从房间移除
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body object true "成员信息（room_id, user_id, operator_id）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/remove [post]
func (c *ChatEngine) HandleRemoveRoomMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RoomID     uint64 `json:"room_id"`
			UserID     uint64 `json:"user_id"`
			OperatorID uint64 `json:"operator_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.MemberService.RemoveRoomMember(req.RoomID, req.UserID, req.OperatorID)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "成员已移除",
		}).WriteJSON(w)
	}
}

// HandleUserRegister 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.RegisterReq true "注册信息"
// @Success 200 {object} response.Response "注册成功"
// @Failure 400 {object} response.Response "请求错误"
// @Router /user/register [post]
func (c *ChatEngine) HandleUserRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req service.RegisterReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		err := c.UserService.Register(req)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(nil).WriteJSON(w)
	}
}

// HandleUserLogin 用户登录
// @Summary 用户登录
// @Description 用户登录并返回 token
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body service.LoginReq true "登录信息"
// @Success 200 {object} response.Response{data=service.LoginResp} "登录响应（token + 用户信息）"
// @Failure 401 {object} response.Response "认证失败"
// @Router /user/login [post]
func (c *ChatEngine) HandleUserLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req service.LoginReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		resp, err := c.UserService.LoginWithToken(r.Context(), req)
		if err != nil {
			// 登录失败返回 HTTP 401 + 统一 response 格式
			response.Error(response.CodePasswordError, err.Error()).WriteJSONWithStatus(w, http.StatusUnauthorized)
			return
		}

		response.Success(resp).WriteJSON(w)
	}
}

// HandleGetUserInfo 获取用户信息
// @Summary 获取用户信息
// @Description 根据 user_id 查询用户详情
// @Description
// @Description 业务状态码说明:
// @Description - code=0: 查询成功
// @Description - code=10001: 参数错误（user_id 无效）
// @Description - code=10002: 用户不存在
// @Description - code=10004: Token 无效（返回 HTTP 401）
// @Tags 用户
// @Accept json
// @Produce json
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response{data=service.UserDTO} "查询成功 (code=0)"
// @Failure 400 {object} response.Response "参数错误 (code=10001)"
// @Failure 401 {object} response.Response "未登录或Token无效 (code=10004)"
// @Failure 404 {object} response.Response "用户不存在 (code=10002)"
// @Security BearerAuth
// @Security QueryToken
// @Router /user/info [get]
func (c *ChatEngine) HandleGetUserInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || userID == 0 {
			response.Error(response.CodeParamError, "invalid user_id").WriteJSON(w)
			return
		}

		u, err := c.UserService.GetUser(userID)
		if err != nil {
			response.Error(response.CodeUserNotFound, err.Error()).WriteJSON(w)
			return
		}

		response.Success(u).WriteJSON(w)
	}
}

// HandleUpdateUserInfo 更新用户信息
// @Summary 更新用户信息
// @Description 更新用户资料（昵称/签名/性别等）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "更新信息（user_id + 可选字段）"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/update [post]
func (c *ChatEngine) HandleUpdateUserInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID uint64 `json:"user_id"`
			service.UpdateUserReq
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}
		if req.UserID == 0 {
			response.Error(response.CodeParamError, "user_id is required").WriteJSON(w)
			return
		}

		u, err := c.UserService.UpdateUser(req.UserID, req.UpdateUserReq)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(u).WriteJSON(w)
	}
}

// HandleUpdateUserAvatar 更新用户头像
// @Summary 更新用户头像
// @Description 更新用户头像 URL
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "头像更新（user_id + avatar）"
// @Success 200 {object} response.Response{data=service.UserDTO} "更新后的用户信息"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/avatar [post]
func (c *ChatEngine) HandleUpdateUserAvatar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID uint64 `json:"user_id"`
			Avatar string `json:"avatar"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}

		if req.UserID == 0 {
			response.Error(response.CodeParamError, "user_id is required").WriteJSON(w)
			return
		}

		u, err := c.UserService.UpdateAvatar(req.UserID, req.Avatar)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(u).WriteJSON(w)
	}
}

// HandleUpdateUserPassword 修改密码
// @Summary 修改用户密码
// @Description 修改用户密码（验证码/鉴权由调用层保证）
// @Tags 用户
// @Accept json
// @Produce json
// @Param req body object true "密码修改（user_id + new_password）"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "请求错误"
// @Security BearerAuth
// @Router /user/password [post]
func (c *ChatEngine) HandleUpdateUserPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID      uint64 `json:"user_id"`
			NewPassword string `json:"new_password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}
		if req.UserID == 0 {
			response.Error(response.CodeParamError, "user_id is required").WriteJSON(w)
			return
		}
		if strings.TrimSpace(req.NewPassword) == "" {
			response.Error(response.CodeParamError, "new_password is required").WriteJSON(w)
			return
		}

		if err := c.UserService.UpdatePassword(req.UserID, req.NewPassword); err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{
			"message": "密码已更新",
		}).WriteJSON(w)
	}
}

// HandleSearchUsers 搜索用户
// @Summary 搜索用户
// @Description 按关键字搜索用户（username/nickname/uid）
// @Tags 用户
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键字"
// @Param user_id query uint64 false "排除的用户ID"
// @Param limit query int false "返回条数"
// @Param offset query int false "偏移量"
// @Success 200 {object} response.Response{data=[]service.UserDTO} "用户列表"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /user/search [get]
func (c *ChatEngine) HandleSearchUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword := r.URL.Query().Get("keyword")
		excludeStr := r.URL.Query().Get("user_id")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		excludeID, _ := strconv.ParseUint(excludeStr, 10, 64)
		limit, _ := strconv.Atoi(limitStr)
		offset, _ := strconv.Atoi(offsetStr)

		users, err := c.UserService.SearchUsers(keyword, excludeID, limit, offset)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(users).WriteJSON(w)
	}
}

// HandleCreateGroupRoom 创建群聊房间
// @Summary 创建群聊
// @Description 创建新的群聊房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body object true "群聊信息（name, creator, members）"
// @Success 200 {object} response.Response{data=model.Room} "房间信息"
// @Failure 400 {object} response.Response "请求错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/group [post]
func (c *ChatEngine) HandleCreateGroupRoom() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string   `json:"name"`
			Creator uint64   `json:"creator"`
			Members []uint64 `json:"members"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}
		if req.Creator == 0 {
			response.Error(response.CodeParamError, "creator is required").WriteJSON(w)
			return
		}

		room, err := c.RoomService.CreateGroupRoom(req.Name, req.Creator, req.Members)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(room).WriteJSON(w)
	}
}

// HandleCreatePrivateRoom 创建私聊房间
// @Summary 创建私聊
// @Description 创建或获取两人私聊房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param req body object true "私聊信息（user_id, target_id）"
// @Success 200 {object} response.Response{data=model.Room} "房间信息"
// @Failure 400 {object} response.Response "请求错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/private [post]
func (c *ChatEngine) HandleCreatePrivateRoom() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID     uint64 `json:"user_id"`
			TargetID   uint64 `json:"target_id"`
			OperatorID uint64 `json:"operator_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(response.CodeParamError, err.Error()).WriteJSON(w)
			return
		}
		if req.UserID == 0 || req.TargetID == 0 {
			response.Error(response.CodeParamError, "user_id and target_id are required").WriteJSON(w)
			return
		}

		room, err := c.RoomService.CreatePrivateRoom(req.UserID, req.TargetID)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(room).WriteJSON(w)
	}
}

// HandleGetUserRooms 获取用户参与的房间列表
// @Summary 获取用户房间列表
// @Description 获取用户参与的所有房间
// @Tags 房间
// @Accept json
// @Produce json
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response{data=[]model.Room} "房间列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/list [get]
func (c *ChatEngine) HandleGetUserRooms() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		uid, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || uid == 0 {
			response.Error(response.CodeParamError, "invalid user_id").WriteJSON(w)
			return
		}

		rooms, err := c.RoomService.GetUserRooms(uint(uid))
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(rooms).WriteJSON(w)
	}
}

// HandleCheckRoomMember 检查用户是否是房间成员
// @Summary 检查房间成员
// @Description 检查用户是否是房间成员
// @Tags 房间
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Param user_id query uint64 true "用户ID"
// @Success 200 {object} response.Response{data=map[string]bool} "检查结果"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /room/member/check [get]
func (c *ChatEngine) HandleCheckRoomMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomIDStr := r.URL.Query().Get("room_id")
		userIDStr := r.URL.Query().Get("user_id")

		rid, err1 := strconv.ParseUint(roomIDStr, 10, 64)
		uid, err2 := strconv.ParseUint(userIDStr, 10, 64)
		if err1 != nil || err2 != nil || rid == 0 || uid == 0 {
			response.Error(response.CodeParamError, "invalid room_id or user_id").WriteJSON(w)
			return
		}

		ok, err := c.RoomService.CheckRoomMember(uint(rid), uint(uid))
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{"is_member": ok}).WriteJSON(w)
	}
}

// HandleGetMessageByID 根据 message_id 获取消息
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
func (c *ChatEngine) HandleGetMessageByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msgIDStr := r.URL.Query().Get("message_id")
		mid, err := strconv.ParseUint(msgIDStr, 10, 64)
		if err != nil || mid == 0 {
			response.Error(response.CodeParamError, "invalid message_id").WriteJSON(w)
			return
		}

		msg, err := c.MsgService.GetMessageByID(mid)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(msg).WriteJSON(w)
	}
}

// HandleCheckFriendship 检查是否好友
// @Summary 检查好友关系
// @Description 检查两个用户是否是好友
// @Tags 好友
// @Accept json
// @Produce json
// @Param user1 query uint64 true "用户1 ID"
// @Param user2 query uint64 true "用户2 ID"
// @Success 200 {object} response.Response{data=map[string]bool} "好友关系检查结果"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /friend/check [get]
func (c *ChatEngine) HandleCheckFriendship() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u1Str := r.URL.Query().Get("user1")
		u2Str := r.URL.Query().Get("user2")
		u1, err1 := strconv.ParseUint(u1Str, 10, 64)
		u2, err2 := strconv.ParseUint(u2Str, 10, 64)
		if err1 != nil || err2 != nil || u1 == 0 || u2 == 0 {
			response.Error(response.CodeParamError, "invalid user1 or user2").WriteJSON(w)
			return
		}

		ok, err := c.MemberService.CheckFriendship(u1, u2)
		if err != nil {
			response.Error(response.CodeInternalError, err.Error()).WriteJSON(w)
			return
		}

		response.Success(map[string]interface{}{"is_friend": ok}).WriteJSON(w)
	}
}

// HandleMemberSearchUsers 使用 MemberService.SearchUsers（返回 userID 列表；query: keyword, user_id, limit）
func (c *ChatEngine) HandleMemberSearchUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword := r.URL.Query().Get("keyword")
		curStr := r.URL.Query().Get("user_id")
		limitStr := r.URL.Query().Get("limit")

		curID, _ := strconv.ParseInt(curStr, 10, 64)
		limit, _ := strconv.Atoi(limitStr)

		ids, err := c.MemberService.SearchUsers(keyword, curID, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ids); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}
