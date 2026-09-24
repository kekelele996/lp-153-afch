package model

import "time"

// CapsuleReply 时光胶囊收件人的回信。每个胶囊至多一封；状态机：active -> withdrawn（仅胶囊主人可撤回）。
// 撤回后内容对收件人不再可见，但主人仍可看到原文（内容列不清除，仅置状态）。
type CapsuleReply struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	CapsuleID   uint64     `gorm:"uniqueIndex;not null" json:"capsule_id"`
	UserID      uint64     `gorm:"index;not null" json:"user_id"` // 回信人（即胶囊收件人）
	Content     string     `gorm:"type:text;not null" json:"content"`
	Status      string     `gorm:"size:20;index;not null;default:active" json:"status"`
	WithdrawnAt *time.Time `json:"withdrawn_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
