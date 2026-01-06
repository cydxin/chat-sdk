package chat_sdk

import (
	"net/http"
	"strconv"

	model "github.com/cydxin/chat-sdk/models"

	"github.com/cydxin/chat-sdk/response"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

var _ = model.Moment{}

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

type CommentMomentReq struct {
	MomentID uint64  `json:"moment_id" binding:"required"`
	Content  string  `json:"content" binding:"required"`
	ParentID *uint64 `json:"parent_id"`
}

// GinHandleCommentMoment 评论动态
// @Summary 评论动态
// @Tags 朋友圈
// @Accept json
// @Produce json
// @Param req body CommentMomentReq true "评论内容"
// @Success 200 {object} response.Response "成功"
// @Security BearerAuth
// @Router /moment/comment [post]
func (c *ChatEngine) GinHandleCommentMoment(ctx *gin.Context) {
	var req CommentMomentReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, response.Error(response.CodeParamError, err.Error()))
		return
	}
	uid, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, response.Error(response.CodeTokenInvalid, "user_id not found"))
		return
	}
	if err := c.MomentService.AddComment(uid.(uint64), req.MomentID, req.Content, req.ParentID); err != nil {
		ctx.JSON(http.StatusOK, response.Error(response.CodeInternalError, err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, response.Success(nil))
}

// GinHandleListMomentComments 获取动态评论
// @Summary 获取动态评论
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
