package model

import "time"

// TimeCapsule 时光胶囊实体。状态机：locked -> unlocked（到解锁时间自动解锁）。
// 可指定收件人（RecipientID）：解锁前收件人仅可见标题与剩余时间，到期后双方均可阅读。
type TimeCapsule struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	UserID      uint64     `gorm:"index;not null" json:"user_id"`
	RecipientID *uint64    `gorm:"index" json:"recipient_id"`
	Title       string     `gorm:"size:100;not null" json:"title"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	ImageURLs   string     `gorm:"type:text;not null;default:'[]'" json:"-"`
	AudioURL    string     `gorm:"size:255" json:"audio_url"`
	UnlockAt    time.Time  `gorm:"not null" json:"unlock_at"`
	Status      string     `gorm:"size:20;index;not null;default:locked" json:"status"`
	UnlockedAt  *time.Time `json:"unlocked_at"`
	// 收件人到期后的回信（最多一封），以时间戳是否为空判定是否已回信。
	ReplyContent   string     `gorm:"type:text;not null;default:''" json:"-"`
	ReplyAt        *time.Time `json:"reply_at"`
	ReplyWithdrawn bool       `gorm:"not null;default:false" json:"reply_withdrawn"`
	CreatedAt      time.Time  `json:"created_at"`
}
