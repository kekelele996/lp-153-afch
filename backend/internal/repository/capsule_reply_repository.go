package repository

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/wishwall/wishwall/internal/model"
)

// CapsuleReplyRepository 时光胶囊回信仓储接口。
type CapsuleReplyRepository interface {
	Create(reply *model.CapsuleReply) error
	FindByCapsuleID(capsuleID uint64) (*model.CapsuleReply, error)
	ListByCapsuleIDs(capsuleIDs []uint64) ([]model.CapsuleReply, error)
	Update(reply *model.CapsuleReply) error
	DeleteByCapsuleID(capsuleID uint64) error
}

type capsuleReplyRepository struct {
	db *gorm.DB
}

// NewCapsuleReplyRepository 构造胶囊回信仓储。
func NewCapsuleReplyRepository(db *gorm.DB) CapsuleReplyRepository {
	return &capsuleReplyRepository{db: db}
}

func (r *capsuleReplyRepository) Create(reply *model.CapsuleReply) error {
	if err := r.db.Create(reply).Error; err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("create capsule reply: %w", ErrDuplicate)
		}
		return fmt.Errorf("create capsule reply: %w", err)
	}
	return nil
}

func (r *capsuleReplyRepository) FindByCapsuleID(capsuleID uint64) (*model.CapsuleReply, error) {
	var reply model.CapsuleReply
	if err := r.db.Where("capsule_id = ?", capsuleID).First(&reply).Error; err != nil {
		if isRecordNotFound(err) {
			return nil, fmt.Errorf("find reply by capsule %d: %w", capsuleID, ErrNotFound)
		}
		return nil, fmt.Errorf("find reply by capsule %d: %w", capsuleID, err)
	}
	return &reply, nil
}

// ListByCapsuleIDs 批量查询多个胶囊的回信（列表页展示回信状态，N+1 合并为一次查询）。
func (r *capsuleReplyRepository) ListByCapsuleIDs(capsuleIDs []uint64) ([]model.CapsuleReply, error) {
	var items []model.CapsuleReply
	if len(capsuleIDs) == 0 {
		return items, nil
	}
	if err := r.db.Where("capsule_id IN ?", capsuleIDs).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list replies by capsules %v: %w", capsuleIDs, err)
	}
	return items, nil
}

func (r *capsuleReplyRepository) Update(reply *model.CapsuleReply) error {
	if err := r.db.Save(reply).Error; err != nil {
		return fmt.Errorf("update capsule reply %d: %w", reply.ID, err)
	}
	return nil
}

// DeleteByCapsuleID 删除胶囊时级联清理回信。
func (r *capsuleReplyRepository) DeleteByCapsuleID(capsuleID uint64) error {
	if err := r.db.Where("capsule_id = ?", capsuleID).Delete(&model.CapsuleReply{}).Error; err != nil {
		return fmt.Errorf("delete reply of capsule %d: %w", capsuleID, err)
	}
	return nil
}
