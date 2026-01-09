package chat_sdk

import (
	"net/http"
	"strconv"

	"github.com/cydxin/chat-sdk/response"
	"github.com/gin-gonic/gin"
)

// -------------------- 通知（Notification）相关接口 --------------------

// GinHandleListNotifications 拉取通知（默认近 2 天）
// @Summary 拉取通知
// @Tags 通知
// @Accept json
// @Produce json
// @Param days query int false "近 N 天(默认2)"
// @Param cursor query uint64 false "游标(上一页最小id)"
// @Param limit query int false "条数(默认50,最大200)"
// @Param room_id query uint64 false "按房间过滤"
// @Param unread_only query bool false "只看未读"
// @Success 200 {object} response.Response{data=map[string]interface{}} "data.items + data.next_cursor"
// @Security BearerAuth
// @Router /notification/list [get]
func (c *ChatEngine) GinHandleListNotifications(ctx *gin.Context) {
	uidAny, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	uid := uidAny.(uint64)

	// 默认：近 2 天；但如果没传 room_id，则获取自己全部的一天内通知
	days, _ := strconv.Atoi(ctx.DefaultQuery("days", "2"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "50"))
	cursor, _ := strconv.ParseUint(ctx.DefaultQuery("cursor", "0"), 10, 64)
	unreadOnly := ctx.DefaultQuery("unread_only", "false") == "true"

	var roomID *uint64
	if ridStr := ctx.Query("room_id"); ridStr != "" {
		rid, err := strconv.ParseUint(ridStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, "invalid room_id"))
			return
		}
		roomID = &rid
	} else {
		// 没传 room_id：强制取近 1 天 + 不按房间过滤（roomID=nil）
		days = 1
		roomID = nil
	}

	items, nextCursor, err := c.NotificationService.ListUserNotifications(uid, days, cursor, limit, roomID, unreadOnly)
	if err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	}))
}

type MarkNotificationsReadReq struct {
	IDs []uint64 `json:"ids" binding:"required"`
}

// GinHandleMarkNotificationsRead 标记通知已读
// @Summary 标记通知已读
// @Tags 通知
// @Accept json
// @Produce json
// @Param req body MarkNotificationsReadReq true "请求参数"
// @Success 200 {object} response.Response
// @Security BearerAuth
// @Router /notification/read [post]
func (c *ChatEngine) GinHandleMarkNotificationsRead(ctx *gin.Context) {
	uidAny, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	uid := uidAny.(uint64)

	var req MarkNotificationsReadReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}

	if err := c.NotificationService.MarkReadByIDs(uid, req.IDs); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, response.Success(nil))
}
