package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/wishwall/wishwall/internal/model"
)

// UserRepository 用户仓储接口。
type UserRepository interface {
	Create(user *model.User) error
	FindByID(id uint64) (*model.User, error)
	FindByIDs(ids []uint64) ([]model.User, error)
	FindByUsername(username string) (*model.User, error)
	FindByEmail(email string) (*model.User, error)
	FindByAccount(account string) (*model.User, error)
	Update(user *model.User) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造用户仓储。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *model.User) error {
	if err := r.db.Create(user).Error; err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("create user: %w", ErrDuplicate)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *userRepository) FindByID(id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find user by id %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("find user by id %d: %w", id, err)
	}
	return &user, nil
}

// FindByIDs 按 ID 批量查询用户（胶囊「收到的」列表补全主人资料）。
func (r *userRepository) FindByIDs(ids []uint64) ([]model.User, error) {
	var users []model.User
	if len(ids) == 0 {
		return users, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("find users by ids %v: %w", ids, err)
	}
	return users, nil
}

func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find user by username %s: %w", username, ErrNotFound)
		}
		return nil, fmt.Errorf("find user by username %s: %w", username, err)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find user by email %s: %w", email, ErrNotFound)
		}
		return nil, fmt.Errorf("find user by email %s: %w", email, err)
	}
	return &user, nil
}

// FindByAccount 支持用户名或邮箱登录。
func (r *userRepository) FindByAccount(account string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ? OR email = ?", account, account).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find user by account %s: %w", account, ErrNotFound)
		}
		return nil, fmt.Errorf("find user by account %s: %w", account, err)
	}
	return &user, nil
}

func (r *userRepository) Update(user *model.User) error {
	if err := r.db.Save(user).Error; err != nil {
		return fmt.Errorf("update user %d: %w", user.ID, err)
	}
	return nil
}
