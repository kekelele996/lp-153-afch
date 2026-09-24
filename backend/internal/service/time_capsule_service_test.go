package service

import (
	"errors"
	"testing"
	"time"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/dto"
	"github.com/wishwall/wishwall/internal/model"
	"github.com/wishwall/wishwall/internal/repository"
	"github.com/wishwall/wishwall/internal/util"
)

const (
	capsuleOwnerID     uint64 = 1
	capsuleRecipientID uint64 = 2
	capsuleOutsiderID  uint64 = 3
)

func newTestCapsuleService(capsuleRepo *mockCapsuleRepo, userRepo *mockUserRepo) TimeCapsuleService {
	return NewTimeCapsuleService(capsuleRepo, userRepo, &mockAudit{}, testLogger())
}

func capsuleUsers() *mockUserRepo {
	return &mockUserRepo{
		findByIDFn: func(id uint64) (*model.User, error) {
			if id == capsuleOwnerID {
				return &model.User{ID: id, Username: "alice", Nickname: "爱丽丝"}, nil
			}
			if id == capsuleRecipientID {
				return &model.User{ID: id, Username: "bob", Nickname: "鲍勃"}, nil
			}
			return nil, repository.ErrNotFound
		},
		findByUsernameFn: func(username string) (*model.User, error) {
			switch username {
			case "alice":
				return &model.User{ID: capsuleOwnerID, Username: "alice"}, nil
			case "bob":
				return &model.User{ID: capsuleRecipientID, Username: "bob"}, nil
			default:
				return nil, repository.ErrNotFound
			}
		},
	}
}

func TestTimeCapsuleService_CreateRecipient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		username    string
		wantErrCode int
	}{
		{name: "recipient not found stays on form", username: "ghost", wantErrCode: constants.CodeCapsuleRecipientNotFound},
		{name: "recipient cannot be self", username: "alice", wantErrCode: constants.CodeCapsuleRecipientSelf},
		{name: "valid recipient", username: "bob", wantErrCode: 0},
		{name: "no recipient allowed", username: "", wantErrCode: 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			capsuleRepo := &mockCapsuleRepo{}
			svc := newTestCapsuleService(capsuleRepo, capsuleUsers())
			capsule, err := svc.Create(capsuleOwnerID, dto.CreateCapsuleRequest{
				Title:             "给未来的我们",
				Content:           "要一起变勇敢",
				UnlockAt:          time.Now().Add(time.Hour),
				RecipientUsername: tt.username,
			}, "127.0.0.1", "req-1")
			if tt.wantErrCode != 0 {
				var appErr *util.AppError
				if !errors.As(err, &appErr) || appErr.Code != tt.wantErrCode {
					t.Fatalf("expected code %d, got %v", tt.wantErrCode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("create should succeed, got %v", err)
			}
			if tt.username == "bob" && (capsule.RecipientID == nil || *capsule.RecipientID != capsuleRecipientID) {
				t.Fatalf("expected recipient id %d, got %v", capsuleRecipientID, capsule.RecipientID)
			}
			if tt.username == "" && capsule.RecipientID != nil {
				t.Fatalf("expected nil recipient, got %v", capsule.RecipientID)
			}
		})
	}
}

// 解锁前收件人能看标题但看不到正文/图片/语音；到期后自动解锁。
func TestTimeCapsuleService_GetAsRecipient(t *testing.T) {
	t.Parallel()
	userRepo := capsuleUsers()
	capsule := &model.TimeCapsule{
		ID: 10, UserID: capsuleOwnerID, RecipientID: ptrU64(capsuleRecipientID),
		Title: "标题", Content: "秘密正文", ImageURLs: `["img.png"]`, AudioURL: "a.mp3",
		UnlockAt: time.Now().Add(time.Hour), Status: constants.CapsuleStatusLocked,
	}
	capsuleRepo := &mockCapsuleRepo{
		findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return capsule, nil },
	}
	svc := newTestCapsuleService(capsuleRepo, userRepo)

	got, names, err := svc.GetByID(capsuleRecipientID, 10)
	if err != nil {
		t.Fatalf("recipient should view capsule, got %v", err)
	}
	resp := dto.ToCapsuleResponse(got, capsuleRecipientID, *names)
	if resp.Content != "" || resp.AudioURL != "" || len(resp.ImageURLs) != 0 {
		t.Fatalf("locked content must be masked for recipient, got %+v", resp)
	}
	if resp.Title != "标题" {
		t.Fatalf("recipient should see title, got %q", resp.Title)
	}
	if resp.CanReply {
		t.Fatal("recipient cannot reply before unlock")
	}

	// 局外人无权查看
	if _, _, err := svc.GetByID(capsuleOutsiderID, 10); err == nil {
		t.Fatal("outsider must be forbidden")
	}

	// 到期后访问：自动解锁，收件人能看到全部内容并可以回信
	capsule.UnlockAt = time.Now().Add(-time.Minute)
	got2, names2, err := svc.GetByID(capsuleRecipientID, 10)
	if err != nil {
		t.Fatalf("recipient view after due, got %v", err)
	}
	if got2.Status != constants.CapsuleStatusUnlocked || got2.UnlockedAt == nil {
		t.Fatalf("capsule should be auto unlocked, status=%s unlocked_at=%v", got2.Status, got2.UnlockedAt)
	}
	resp2 := dto.ToCapsuleResponse(got2, capsuleRecipientID, *names2)
	if resp2.Content != "秘密正文" || resp2.AudioURL != "a.mp3" || len(resp2.ImageURLs) != 1 {
		t.Fatalf("unlocked content must be visible, got %+v", resp2)
	}
	if !resp2.CanReply {
		t.Fatal("recipient should be able to reply after unlock")
	}
}

// 回信只能由收件人在解锁后写一次，发出后不能改，主人可撤回。
func TestTimeCapsuleService_ReplyAndWithdraw(t *testing.T) {
	t.Parallel()
	userRepo := capsuleUsers()
	capsule := &model.TimeCapsule{
		ID: 11, UserID: capsuleOwnerID, RecipientID: ptrU64(capsuleRecipientID),
		Title: "标题", Content: "正文", UnlockAt: time.Now().Add(-time.Minute),
		Status: constants.CapsuleStatusUnlocked,
	}
	capsuleRepo := &mockCapsuleRepo{
		findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return capsule, nil },
	}
	svc := newTestCapsuleService(capsuleRepo, userRepo)

	// 主人不能给自己的胶囊回信
	if _, err := svc.Reply(capsuleOwnerID, 11, dto.CapsuleReplyRequest{Content: "回信"}, "", ""); err == nil {
		t.Fatal("owner must not reply")
	}
	// 局外人不能回信
	if _, err := svc.Reply(capsuleOutsiderID, 11, dto.CapsuleReplyRequest{Content: "回信"}, "", ""); err == nil {
		t.Fatal("outsider must not reply")
	}

	reply, err := svc.Reply(capsuleRecipientID, 11, dto.CapsuleReplyRequest{Content: "谢谢你的信"}, "", "")
	if err != nil {
		t.Fatalf("recipient reply should succeed, got %v", err)
	}
	if reply.ReplyAt == nil || reply.ReplyContent != "谢谢你的信" {
		t.Fatalf("unexpected reply state: %+v", reply)
	}

	// 再次回信应被拒绝（不能修改）
	if _, err := svc.Reply(capsuleRecipientID, 11, dto.CapsuleReplyRequest{Content: "改一下"}, "", ""); err == nil {
		t.Fatal("second reply must be rejected")
	}

	// 收件人不能撤回，只有主人可以
	if _, err := svc.WithdrawReply(capsuleRecipientID, 11, "", ""); err == nil {
		t.Fatal("recipient must not withdraw reply")
	}
	if _, err := svc.WithdrawReply(capsuleOwnerID, 11, "", ""); err != nil {
		t.Fatalf("owner withdraw should succeed, got %v", err)
	}
	if !capsule.ReplyWithdrawn || capsule.ReplyContent != "" {
		t.Fatalf("reply should be withdrawn and content cleared, got %+v", capsule)
	}
}

// 未解锁时收件人尝试回信应被拒绝。
func TestTimeCapsuleService_ReplyLocked(t *testing.T) {
	t.Parallel()
	capsule := &model.TimeCapsule{
		ID: 12, UserID: capsuleOwnerID, RecipientID: ptrU64(capsuleRecipientID),
		Title: "标题", Content: "正文", UnlockAt: time.Now().Add(time.Hour),
		Status: constants.CapsuleStatusLocked,
	}
	capsuleRepo := &mockCapsuleRepo{
		findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return capsule, nil },
	}
	svc := newTestCapsuleService(capsuleRepo, capsuleUsers())
	if _, err := svc.Reply(capsuleRecipientID, 12, dto.CapsuleReplyRequest{Content: "太早了"}, "", ""); err == nil {
		t.Fatal("reply before unlock must be rejected")
	}
}

func ptrU64(v uint64) *uint64 { return &v }
