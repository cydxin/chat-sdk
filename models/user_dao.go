package models

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// UserDAO 封装 User 相关的数据库操作
type UserDAO struct {
	db *gorm.DB
}

func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

func (dao *UserDAO) Create(user *User) error {
	return dao.db.Create(user).Error
}

func (dao *UserDAO) FindByID(id uint64) (*User, error) {
	var u User
	if err := dao.db.Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (dao *UserDAO) FindByUID(uid string) (*User, error) {
	var u User
	if err := dao.db.Where("uid = ?", uid).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (dao *UserDAO) FindByUsername(username string) (*User, error) {
	var u User
	if err := dao.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (dao *UserDAO) FindByPhone(phone string) (*User, error) {
	if phone == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var u User
	if err := dao.db.Where("phone = ?", phone).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (dao *UserDAO) FindByEmail(email string) (*User, error) {
	if email == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var u User
	if err := dao.db.Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (dao *UserDAO) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := dao.db.Model(&User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (dao *UserDAO) ExistsByPhone(phone string) (bool, error) {
	if phone == "" {
		return false, nil
	}
	var count int64
	err := dao.db.Model(&User{}).Where("phone = ?", phone).Count(&count).Error
	return count > 0, err
}

func (dao *UserDAO) ExistsByEmail(email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	var count int64
	err := dao.db.Model(&User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (dao *UserDAO) UpdateAvatar(id uint64, avatar string) error {
	return dao.db.Model(&User{}).Where("id = ?", id).Update("avatar", avatar).Error
}

func (dao *UserDAO) UpdateFields(id uint64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return dao.db.Model(&User{}).Where("id = ?", id).Updates(updates).Error
}

func (dao *UserDAO) UpdatePassword(id uint64, hashedPassword string) error {
	return dao.db.Model(&User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

// SearchUsers 按关键字搜索用户（username/nickname/uid），可排除某个 userID。
// 注意：返回的是完整 User 结构体（含 Password），上层请自行转 DTO/脱敏。
func (dao *UserDAO) SearchUsers(keyword string, excludeUserID uint64, limit, offset int) ([]User, error) {
	keyword = strings.TrimSpace(keyword)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	q := dao.db.Model(&User{})
	if excludeUserID > 0 {
		q = q.Where("id <> ?", excludeUserID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR nickname LIKE ? OR uid LIKE ?", like, like, like)
	}

	var users []User
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}

func (dao *UserDAO) IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (dao *UserDAO) FindByAccount(account string) (*User, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if strings.Contains(account, "@") {
		return dao.FindByEmail(strings.ToLower(account))
	}

	u, err := dao.FindByUsername(account)
	if err == nil {
		return u, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dao.FindByPhone(account)
	}
	return nil, err
}

// ExistsByAccount 检查 username/phone/email 任意一种是否已存在（用于注册唯一性校验）。
// 返回：
// - kind: 0-都不存在 1-username 已存在 2-phone 已存在 3-email 已存在
// - value: 命中的值（便于上层拼错误文案）
func (dao *UserDAO) ExistsByAccount(username, phone, email string) (kind uint8, value string, err error) {
	username = strings.TrimSpace(username)
	phone = strings.TrimSpace(phone)
	email = strings.TrimSpace(email)
	if strings.Contains(email, "@") {
		email = strings.ToLower(email)
	}

	// 组装 OR 查询（一次 SQL 搞定）
	q := dao.db.Model(&User{}).Select("username, phone, email")
	first := true
	if username != "" {
		q = q.Where("username = ?", username)
		first = false
	}
	if phone != "" {
		if first {
			q = q.Where("phone = ?", phone)
			first = false
		} else {
			q = q.Or("phone = ?", phone)
		}
	}
	if email != "" {
		if first {
			q = q.Where("email = ?", email)
			first = false
		} else {
			q = q.Or("email = ?", email)
		}
	}
	if first {
		return 0, "", nil
	}

	var hit User
	if err := q.Limit(1).Take(&hit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", nil
		}
		return 0, "", err
	}

	// 判断命中字段（优先级：username > phone > email）
	if username != "" && hit.Username == username {
		return 1, username, nil
	}
	if phone != "" && hit.Phone == phone {
		return 2, phone, nil
	}
	if email != "" && strings.ToLower(hit.Email) == email {
		return 3, email, nil
	}
	// 理论不会到这里（命中了但不匹配传入值），保守返回存在
	return 1, hit.Username, nil
}

// UserBrief 用于返回给业务层的用户展示信息
type UserBrief struct {
	UserID   uint64 `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// OnlineUserBriefGetter 用于从内存/WS 在线列表中读取用户信息。
// 返回值：
//   - brief: 如果 ok=true 则代表命中在线缓存
//   - ok: 是否命中
//   - err: 获取过程错误（一般应返回 nil）
type OnlineUserBriefGetter func(userID uint64) (brief UserBrief, ok bool, err error)

// BatchGetUserBriefsPreferOnline 批量获取用户展示信息：优先在线缓存，未命中再批量查库。
// 返回 map[userID]UserBrief（只保证包含传入 ids 中的项；缺失则给空 nickname/avatar）。
func (dao *UserDAO) BatchGetUserBriefsPreferOnline(ids []uint64, onlineGetter OnlineUserBriefGetter) (map[uint64]UserBrief, error) {
	out := make(map[uint64]UserBrief, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// 1) 先走在线缓存
	miss := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		if onlineGetter != nil {
			b, ok, err := onlineGetter(id)
			if err != nil {
				return nil, err
			}
			if ok {
				out[id] = b
				continue
			}
		}
		miss = append(miss, id)
	}

	if len(miss) == 0 {
		return out, nil
	}

	// 2) 批量查库（只取必要字段）
	type row struct {
		ID       uint64
		Nickname string
		Avatar   string
	}
	var rows []row
	if err := dao.db.Model(&User{}).
		Select("id, nickname, avatar").
		Where("id IN ?", miss).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = UserBrief{UserID: r.ID, Nickname: r.Nickname, Avatar: r.Avatar}
	}

	// 3) 保证所有 miss 都有默认值
	for _, id := range miss {
		if _, ok := out[id]; !ok {
			out[id] = UserBrief{UserID: id, Nickname: "", Avatar: ""}
		}
	}

	return out, nil
}
