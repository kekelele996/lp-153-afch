package dto

import (
	"testing"
	"time"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/model"
)

// TestToCapsuleResponseMasking 解锁前正文/图片/语音对主人与收件人一律打码；解锁后双方可见。
func TestToCapsuleResponseMasking(t *testing.T) {
	recipient := uint64(2)
	capsule := &model.TimeCapsule{
		ID:                1,
		UserID:            1,
		Title:             "给鲍勃",
		Content:           "秘密正文内容",
		ImageURLs:         `["http://img/a.png","http://img/b.png"]`,
		AudioURL:          "http://audio/a.mp3",
		UnlockAt:          time.Now().Add(24 * time.Hour),
		Status:            constants.CapsuleStatusLocked,
		RecipientID:       &recipient,
		RecipientUsername: "bob",
	}

	for _, viewerIsOwner := range []bool{true, false} {
		resp := ToCapsuleResponse(CapsuleResponseInput{Capsule: capsule, ViewerIsOwner: viewerIsOwner})
		if resp.Content != "" || resp.AudioURL != "" || len(resp.ImageURLs) != 0 {
			t.Fatalf("locked capsule must be fully masked (owner=%v): %+v", viewerIsOwner, resp)
		}
		if resp.Title != "给鲍勃" {
			t.Fatalf("title must stay visible: %+v", resp)
		}
	}

	capsule.Status = constants.CapsuleStatusUnlocked
	for _, viewerIsOwner := range []bool{true, false} {
		resp := ToCapsuleResponse(CapsuleResponseInput{Capsule: capsule, ViewerIsOwner: viewerIsOwner})
		if resp.Content != "秘密正文内容" || resp.AudioURL != "http://audio/a.mp3" || len(resp.ImageURLs) != 2 {
			t.Fatalf("unlocked capsule must be readable by both (owner=%v): %+v", viewerIsOwner, resp)
		}
		if viewerIsOwner && resp.Role != "owner" {
			t.Fatalf("role should be owner, got %s", resp.Role)
		}
		if !viewerIsOwner && resp.Role != "recipient" {
			t.Fatalf("role should be recipient, got %s", resp.Role)
		}
	}
}

// TestToCapsuleReplyResponseWithdrawn 回信撤回后：主人可见原文，收件人只见 withdrawn 状态。
func TestToCapsuleReplyResponseWithdrawn(t *testing.T) {
	withdrawnAt := time.Now()
	reply := &model.CapsuleReply{
		ID: 1, CapsuleID: 10, UserID: 2,
		Content: "给主人的回信", Status: constants.ReplyStatusWithdrawn,
		WithdrawnAt: &withdrawnAt,
	}

	ownerView := ToCapsuleReplyResponse(reply, false)
	if ownerView.Content != "给主人的回信" || ownerView.WithdrawnAt == nil {
		t.Fatalf("owner must keep reply content: %+v", ownerView)
	}

	recipientView := ToCapsuleReplyResponse(reply, true)
	if recipientView.Content != "" || recipientView.Status != constants.ReplyStatusWithdrawn {
		t.Fatalf("recipient must see masked withdrawn reply: %+v", recipientView)
	}
}

// TestNormalizedRecipient 收件人用户名首尾空格被裁剪，空串表示不指定。
func TestNormalizedRecipient(t *testing.T) {
	if got := (CreateCapsuleRequest{RecipientUsername: "  bob  "}).NormalizedRecipient(); got != "bob" {
		t.Fatalf("expected trimmed bob, got %q", got)
	}
	if got := (CreateCapsuleRequest{RecipientUsername: "   "}).NormalizedRecipient(); got != "" {
		t.Fatalf("expected empty recipient, got %q", got)
	}
}
