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

// ExistsByAccount 检查 username/phone/email 任意一种是否存在（用于注册唯一性校验）。
func (dao *UserDAO) ExistsByAccount(username, phone, email string) (bool, error) {
	username = strings.TrimSpace(username)
	phone = strings.TrimSpace(phone)
	email = strings.TrimSpace(email)
	if strings.Contains(email, "@") {
		email = strings.ToLower(email)
	}

	if username != "" {
		exists, err := dao.ExistsByUsername(username)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	if phone != "" {
		exists, err := dao.ExistsByPhone(phone)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	if email != "" {
		exists, err := dao.ExistsByEmail(email)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}
