package dto

import (
	"time"

	"github.com/wishwall/wishwall/internal/model"
)

// CreateCapsuleRequest 创建时光胶囊入参。RecipientUsername 选填：指定共同开启胶囊的收件人。
type CreateCapsuleRequest struct {
	Title             string    `json:"title" binding:"required,min=2,max=100"`
	Content           string    `json:"content" binding:"required,min=5,max=2000"`
	ImageURLs         []string  `json:"image_urls"`
	AudioURL          string    `json:"audio_url" binding:"omitempty,max=255"`
	UnlockAt          time.Time `json:"unlock_at" binding:"required"`
	RecipientUsername string    `json:"recipient_username" binding:"omitempty,max=50"`
}

// CapsuleReplyRequest 收件人回信入参（只能回信一次）。
type CapsuleReplyRequest struct {
	Content string `json:"content" binding:"required,min=1,max=2000"`
}

// CapsuleResponse 胶囊返回结构。未解锁或收件人视角下 content/image_urls/audio_url 置空。
type CapsuleResponse struct {
	ID                uint64   `json:"id"`
	UserID            uint64   `json:"user_id"`
	RecipientID       *uint64  `json:"recipient_id"`
	OwnerUsername     string   `json:"owner_username"`
	OwnerNickname     string   `json:"owner_nickname"`
	RecipientUsername string   `json:"recipient_username"`
	RecipientNickname string   `json:"recipient_nickname"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	ImageURLs         []string `json:"image_urls"`
	AudioURL          string   `json:"audio_url"`
	UnlockAt          string   `json:"unlock_at"`
	Status            string   `json:"status"`
	UnlockedAt        *string  `json:"unlocked_at"`
	CreatedAt         string   `json:"created_at"`
	IsOwner           bool     `json:"is_owner"`
	CanReply          bool     `json:"can_reply"`
	ReplyContent      string   `json:"reply_content"`
	ReplyAt           *string  `json:"reply_at"`
	ReplyWithdrawn    bool     `json:"reply_withdrawn"`
}

// CapsuleResponseNames 胶囊主人与收件人的用户名信息（由 service 解析后传入）。
type CapsuleResponseNames struct {
	OwnerUsername     string
	OwnerNickname     string
	RecipientUsername string
	RecipientNickname string
}

// ToCapsuleResponse 从模型构造返回结构。viewerID 为当前查看者，names 为解析后的双方用户名。
// 解锁前任何人（含主人与收件人）都看不到正文、图片与语音；收件人在到期且未回信时可回信一次。
func ToCapsuleResponse(c *model.TimeCapsule, viewerID uint64, names CapsuleResponseNames) CapsuleResponse {
	unlocked := c.Status == "unlocked" || !c.UnlockAt.After(time.Now())
	isOwner := c.UserID == viewerID
	isRecipient := c.RecipientID != nil && *c.RecipientID == viewerID

	resp := CapsuleResponse{
		ID:                c.ID,
		UserID:            c.UserID,
		RecipientID:       c.RecipientID,
		OwnerUsername:     names.OwnerUsername,
		OwnerNickname:     names.OwnerNickname,
		RecipientUsername: names.RecipientUsername,
		RecipientNickname: names.RecipientNickname,
		Title:             c.Title,
		ImageURLs:         []string{},
		UnlockAt:          c.UnlockAt.Format("2006-01-02 15:04:05"),
		Status:            c.Status,
		CreatedAt:         c.CreatedAt.Format("2006-01-02 15:04:05"),
		IsOwner:           isOwner,
	}
	if unlocked {
		resp.Status = "unlocked"
		resp.Content = c.Content
		resp.ImageURLs = decodeStrings(c.ImageURLs)
		resp.AudioURL = c.AudioURL
	}
	if c.UnlockedAt != nil {
		s := c.UnlockedAt.Format("2006-01-02 15:04:05")
		resp.UnlockedAt = &s
	}
	// 回信：已发出后收件人不能再改；主人撤回后双方都看不到正文，仅保留撤回标记。
	resp.ReplyWithdrawn = c.ReplyWithdrawn
	if c.ReplyAt != nil {
		s := c.ReplyAt.Format("2006-01-02 15:04:05")
		resp.ReplyAt = &s
		if !c.ReplyWithdrawn {
			resp.ReplyContent = c.ReplyContent
		}
	}
	resp.CanReply = isRecipient && unlocked && c.ReplyAt == nil
	return resp
}
