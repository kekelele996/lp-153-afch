package dto

import (
	"strings"
	"time"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/model"
)

// CreateCapsuleRequest 创建时光胶囊入参。RecipientUsername 选填：指定共同开启的收件人。
type CreateCapsuleRequest struct {
	Title             string    `json:"title" binding:"required,min=2,max=100"`
	Content           string    `json:"content" binding:"required,min=5,max=2000"`
	ImageURLs         []string  `json:"image_urls"`
	AudioURL          string    `json:"audio_url" binding:"omitempty,max=255"`
	UnlockAt          time.Time `json:"unlock_at" binding:"required"`
	RecipientUsername string    `json:"recipient_username" binding:"omitempty,max=50"`
}

// NormalizedRecipient 去除首尾空格后的收件人用户名，空串表示不指定。
func (r CreateCapsuleRequest) NormalizedRecipient() string {
	return strings.TrimSpace(r.RecipientUsername)
}

// CapsuleReplyRequest 收件人回信入参：仅文字，一次机会。
type CapsuleReplyRequest struct {
	Content string `json:"content" binding:"required,min=1,max=2000"`
}

// CapsuleReplyResponse 胶囊回信返回结构。
// 撤回后：主人仍可见正文；收件人只拿到 status=withdrawn 与空正文。
type CapsuleReplyResponse struct {
	ID          uint64  `json:"id"`
	CapsuleID   uint64  `json:"capsule_id"`
	UserID      uint64  `json:"user_id"`
	Content     string  `json:"content"`
	Status      string  `json:"status"`
	WithdrawnAt *string `json:"withdrawn_at"`
	CreatedAt   string  `json:"created_at"`
}

// CapsuleResponse 胶囊返回结构。未解锁时正文/图片/语音全部置空（主人与收件人均不可见）。
type CapsuleResponse struct {
	ID                uint64                `json:"id"`
	UserID            uint64                `json:"user_id"`
	OwnerUsername     string                `json:"owner_username"`
	OwnerNickname     string                `json:"owner_nickname"`
	Title             string                `json:"title"`
	Content           string                `json:"content"`
	ImageURLs         []string              `json:"image_urls"`
	AudioURL          string                `json:"audio_url"`
	UnlockAt          string                `json:"unlock_at"`
	Status            string                `json:"status"`
	UnlockedAt        *string               `json:"unlocked_at"`
	CreatedAt         string                `json:"created_at"`
	RecipientID       *uint64               `json:"recipient_id"`
	RecipientUsername string                `json:"recipient_username"`
	Role              string                `json:"role"` // owner：我封存的；recipient：收到的
	Reply             *CapsuleReplyResponse `json:"reply"`
}

// ToCapsuleReplyResponse 从模型构造回信返回，maskContent 控制撤回后是否对收件人隐藏正文。
func ToCapsuleReplyResponse(r *model.CapsuleReply, maskContent bool) CapsuleReplyResponse {
	resp := CapsuleReplyResponse{
		ID:        r.ID,
		CapsuleID: r.CapsuleID,
		UserID:    r.UserID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if r.WithdrawnAt != nil {
		s := r.WithdrawnAt.Format("2006-01-02 15:04:05")
		resp.WithdrawnAt = &s
	}
	if maskContent && r.Status == constants.ReplyStatusWithdrawn {
		resp.Content = ""
	} else {
		resp.Content = r.Content
	}
	return resp
}

// CapsuleResponseInput 列表/详情组装入参：胶囊 + 视角（viewerIsOwner）+ 回信 + 主人资料。
type CapsuleResponseInput struct {
	Capsule       *model.TimeCapsule
	ViewerIsOwner bool
	Reply         *model.CapsuleReply
	OwnerUsername string
	OwnerNickname string
}

// ToCapsuleResponse 从模型构造返回结构：
//   - 未解锁：主人与收件人均只能看到标题与剩余时间，正文/图片/语音打码；
//   - 已解锁：双方均可阅读全部内容；
//   - 回信撤回后：主人可见回信原文，收件人只见撤回状态。
func ToCapsuleResponse(in CapsuleResponseInput) CapsuleResponse {
	c := in.Capsule
	resp := CapsuleResponse{
		ID:                c.ID,
		UserID:            c.UserID,
		OwnerUsername:     in.OwnerUsername,
		OwnerNickname:     in.OwnerNickname,
		Title:             c.Title,
		UnlockAt:          c.UnlockAt.Format("2006-01-02 15:04:05"),
		Status:            c.Status,
		CreatedAt:         c.CreatedAt.Format("2006-01-02 15:04:05"),
		RecipientID:       c.RecipientID,
		RecipientUsername: c.RecipientUsername,
	}
	if in.ViewerIsOwner {
		resp.Role = "owner"
	} else {
		resp.Role = "recipient"
	}
	if c.UnlockedAt != nil {
		s := c.UnlockedAt.Format("2006-01-02 15:04:05")
		resp.UnlockedAt = &s
	}
	if c.Status == constants.CapsuleStatusLocked {
		// 解锁前正文/图片/语音对双方都不可见
		resp.Content = ""
		resp.ImageURLs = []string{}
		resp.AudioURL = ""
	} else {
		resp.Content = c.Content
		resp.ImageURLs = decodeStrings(c.ImageURLs)
		resp.AudioURL = c.AudioURL
	}
	if in.Reply != nil {
		reply := ToCapsuleReplyResponse(in.Reply, !in.ViewerIsOwner)
		resp.Reply = &reply
	}
	return resp
}
