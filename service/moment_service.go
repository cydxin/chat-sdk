package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

type MomentService struct{ *Service }

func NewMomentService(s *Service) *MomentService { return &MomentService{Service: s} }

// CreateMomentReq 创建动态请求
type CreateMomentReq struct {
	Title  string   `json:"title"`
	Images []string `json:"images"` // 最多9张
	Video  string   `json:"video"`  // 单个视频URL
}

type MomentMediaDTO struct {
	Type uint8  `json:"type"` // 1-图片 2-视频
	URL  string `json:"url"`
	Sort int    `json:"sort"`
}

type MomentDTO struct {
	ID          uint64           `json:"id"`
	UserID      uint64           `json:"user_id"`
	Title       string           `json:"title"`
	MediaType   uint8            `json:"media_type"`
	ImagesCount uint8            `json:"images_count"`
	CommentsCnt uint64           `json:"comments_cnt"`
	Medias      []MomentMediaDTO `json:"medias"`
	CreatedAt   time.Time        `json:"created_at"`
}

func toMomentDTO(m models.Moment, medias []models.MomentMedia) MomentDTO {
	dto := MomentDTO{
		ID:          m.ID,
		UserID:      m.UserID,
		Title:       m.Title,
		MediaType:   m.MediaType,
		ImagesCount: m.ImagesCount,
		CommentsCnt: m.CommentsCnt,
		CreatedAt:   m.CreatedAt,
	}
	dto.Medias = make([]MomentMediaDTO, len(medias))
	sort.Slice(medias, func(i, j int) bool { return medias[i].SortOrder < medias[j].SortOrder })
	for i, mm := range medias {
		dto.Medias[i] = MomentMediaDTO{Type: mm.Type, URL: mm.URL, Sort: mm.SortOrder}
	}
	return dto
}

// CreateMoment 发布动态（图片最多9张 或 视频1个）
func (s *MomentService) CreateMoment(userID uint64, req CreateMomentReq) (MomentDTO, error) {
	imagesCount := len(req.Images)
	hasVideo := strings.TrimSpace(req.Video) != ""

	if imagesCount == 0 && !hasVideo {
		return MomentDTO{}, errors.New("必须提供图片或视频")
	}
	if imagesCount > 0 && hasVideo {
		return MomentDTO{}, errors.New("图片与视频互斥")
	}
	if imagesCount > 9 {
		return MomentDTO{}, errors.New("最多9张图片")
	}

	var mediaType uint8 = 1
	if hasVideo {
		mediaType = 2
	}

	var result MomentDTO
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		m := models.Moment{
			UserID:      userID,
			Title:       req.Title,
			MediaType:   mediaType,
			ImagesCount: uint8(imagesCount),
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}

		// 保存媒体
		var medias []models.MomentMedia
		if hasVideo {
			medias = []models.MomentMedia{{
				MomentID:  m.ID,
				Type:      2,
				URL:       req.Video,
				SortOrder: 0,
			}}
		} else {
			medias = make([]models.MomentMedia, imagesCount)
			for i, u := range req.Images {
				medias[i] = models.MomentMedia{MomentID: m.ID, Type: 1, URL: u, SortOrder: i}
			}
		}
		if len(medias) > 0 {
			if err := tx.Create(&medias).Error; err != nil {
				return err
			}
		}

		result = toMomentDTO(m, medias)
		return nil
	})

	return result, err
}

// ListFriendMoments 列表：自己 + 好友的动态（按时间倒序）
func (s *MomentService) ListFriendMoments(userID uint64, limit, offset int) ([]MomentDTO, error) {
	if limit <= 0 {
		limit = 20
	}

	// 获取好友ID（双向容错）
	var a, b []uint64
	s.DB.Model(&models.Friend{}).Where("user_id = ? AND status = 1", userID).Pluck("friend_id", &a)
	s.DB.Model(&models.Friend{}).Where("friend_id = ? AND status = 1", userID).Pluck("user_id", &b)
	idset := map[uint64]struct{}{userID: {}}
	for _, id := range a {
		idset[id] = struct{}{}
	}
	for _, id := range b {
		idset[id] = struct{}{}
	}
	ids := make([]uint64, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		ids = []uint64{userID}
	}

	// 查询动态
	var moments []models.Moment
	if err := s.DB.Where("user_id IN ?", ids).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&moments).Error; err != nil {
		return nil, err
	}
	if len(moments) == 0 {
		return []MomentDTO{}, nil
	}

	// 拉取媒体
	momentIDs := make([]uint64, len(moments))
	for i, m := range moments {
		momentIDs[i] = m.ID
	}
	var medias []models.MomentMedia
	if err := s.DB.Where("moment_id IN ?", momentIDs).Order("sort_order ASC").Find(&medias).Error; err != nil {
		return nil, err
	}
	mediaMap := make(map[uint64][]models.MomentMedia)
	for _, mm := range medias {
		mediaMap[mm.MomentID] = append(mediaMap[mm.MomentID], mm)
	}

	// 拼装 DTO
	dtos := make([]MomentDTO, len(moments))
	for i, m := range moments {
		dtos[i] = toMomentDTO(m, mediaMap[m.ID])
	}
	return dtos, nil
}

// AddComment 发表评论或回复
func (s *MomentService) AddComment(userID, momentID uint64, content string, parentID *uint64) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("评论内容不能为空")
	}

	// 校验父评论属于同一动态
	if parentID != nil {
		var pc models.MomentComment
		if err := s.DB.First(&pc, *parentID).Error; err != nil {
			return err
		}
		if pc.MomentID != momentID {
			return fmt.Errorf("父评论不属于该动态")
		}
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		c := models.MomentComment{MomentID: momentID, UserID: userID, ParentID: parentID, Content: content}
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Moment{}).Where("id = ?", momentID).
			UpdateColumn("comments_cnt", gorm.Expr("comments_cnt + 1")).Error; err != nil {
			return err
		}
		return nil
	})
}

type CommentDTO struct {
	ID        uint64    `json:"id"`
	MomentID  uint64    `json:"moment_id"`
	UserID    uint64    `json:"user_id"`
	ParentID  *uint64   `json:"parent_id,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ListComments 获取某条动态下的评论（时间升序，便于前端构建树）
func (s *MomentService) ListComments(momentID uint64, limit, offset int) ([]CommentDTO, error) {
	if limit <= 0 {
		limit = 50
	}
	var cs []models.MomentComment
	if err := s.DB.Where("moment_id = ?", momentID).Order("created_at ASC").Limit(limit).Offset(offset).Find(&cs).Error; err != nil {
		return nil, err
	}
	dtos := make([]CommentDTO, len(cs))
	for i, c := range cs {
		dtos[i] = CommentDTO{ID: c.ID, MomentID: c.MomentID, UserID: c.UserID, ParentID: c.ParentID, Content: c.Content, CreatedAt: c.CreatedAt}
	}
	return dtos, nil
}
