package chat_sdk

import (
	"net/http"
	"strconv"
	"strings"

	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/service"

	"github.com/cydxin/chat-sdk/response"
	"github.com/gin-gonic/gin"
)

var _ = model.Message{}
var _ = service.ConversationListItemDTO{}
var _ = service.MessageListItemDTO{}

// -------------------- 消息（Message）相关接口 --------------------

// GinHandleGetMessageConversations 获取消息列表（会话列表）
// @Summary 获取消息列表
// @Description 获取当前用户的会话列表（未删除的会话），包含头像、名称、room、最后一条消息、未读数
// @Tags 消息
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.ConversationListItemDTO} "会话列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/conversations [get]
func (c *ChatEngine) GinHandleGetMessageConversations(ctx *gin.Context) {
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	list, err := c.ConversationService.GetConversationList(uid.(uint64))
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(list))
}

// GinHandleHideConversation 隐藏会话（从消息列表不展示）
// @Summary 隐藏会话
// @Description 将当前用户某个房间的会话从消息列表隐藏（仅影响自己；新消息会自动重新展示）
// @Tags 消息
// @Accept json
// @Produce json
// @Param room_id query uint64 true "房间ID"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/conversation/hide [post]
func (c *ChatEngine) GinHandleHideConversation(ctx *gin.Context) {
	ridStr := ctx.Query("room_id")
	rid, err := strconv.ParseUint(ridStr, 10, 64)
	if err != nil || rid == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	if err := c.ConversationService.SoftDeleteConversation(uid.(uint64), rid); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]any{"message": "ok"}))
}

type RecallReqBody struct {
	MessageIDs []uint64 `json:"message_ids" binding:"required" example:"[123,456,789]"`
	Status     uint8    `json:"status" binding:"required" example:"1"`
}

// GinHandleRecallMessage 撤回/删除消息（批量）
// @Summary 撤回/删除消息（批量）
// @Description 批量撤回/删除消息，body 传 message_ids + status
// @Tags 消息
// @Accept json
// @Produce json
// @Param req body RecallReqBody true "批量操作"
// @Success 200 {object} response.Response "成功响应"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/recall [post]
func (c *ChatEngine) GinHandleRecallMessage(ctx *gin.Context) {

	var req RecallReqBody
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	if len(req.MessageIDs) == 0 {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "message_ids is required"))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id未找到"))
		return
	}

	okIDs, failedMap, err := c.MsgService.RecallMessages(req.MessageIDs, uid.(uint64), req.Status)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodePermissionDeny, err.Error()))
		return
	}

	type itemResult struct {
		MessageID uint64 `json:"message_id"`
		Error     string `json:"error,omitempty"`
	}
	failList := make([]itemResult, 0, len(failedMap))
	for mid, e := range failedMap {
		if mid == 0 {
			continue
		}
		failList = append(failList, itemResult{MessageID: mid, Error: e})
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]any{
		"message":     "ok",
		"success_ids": okIDs,
		"failed":      failList,
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
// @Param mess_id query int false "偏移量 以你要查询的ID为基准，向前查询，不传则向后"
// @Success 200 {object} response.Response{data=[]service.MessageListItemDTO} "消息列表"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /message/list [get]
func (c *ChatEngine) GinHandleGetRoomMessages(ctx *gin.Context) {
	roomIDStr := ctx.Query("room_id")
	limitStr := ctx.Query("limit")
	messIdStr := ctx.Query("mess_id")

	roomID, _ := strconv.ParseUint(roomIDStr, 10, 64)
	limit, _ := strconv.Atoi(limitStr)
	messId, _ := strconv.Atoi(messIdStr)

	if limit == 0 {
		limit = 20
	}

	messages, err := c.MsgService.GetRoomMessagesDTO(roomID, limit, messId)
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

// --- 转发/合并转发 ---

type ForwardMessageReq struct {
	ToRoomIDs []uint64 `json:"to_room_ids" binding:"required"`
	Mode      string   `json:"mode" example:"merge"` // merge/single
	Items     []struct {
		MessageID uint64 `json:"message_id"`
	} `json:"items" binding:"required"`
	Comment string `json:"comment"`
}

// GinHandleForwardMessages 转发消息（支持合并转发）
// @Summary 转发消息
// @Description 支持逐条转发(single) 或 合并转发(merge)
// @Tags 消息
// @Accept json
// @Produce json
// @Param req body ForwardMessageReq true "转发请求"
// @Success 200 {object} response.Response{data=map[string]any} "创建的消息ID列表"
// @Security BearerAuth
// @Router /message/forward [post]
func (c *ChatEngine) GinHandleForwardMessages(ctx *gin.Context) {
	var req ForwardMessageReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}

	items := make([]service.ForwardItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.ForwardItem{MessageID: it.MessageID})
	}

	created, err := c.MsgService.ForwardMessages(ctx.Request.Context(), service.ForwardReq{
		FromUserID: uid.(uint64),
		ToRoomIDs:  req.ToRoomIDs,
		Mode:       service.ForwardMode(strings.ToLower(strings.TrimSpace(req.Mode))),
		Items:      items,
		Comment:    req.Comment,
	})
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]any{"message_ids": created}))
}
