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

func u64ptr(v uint64) *uint64 { return &v }

func newTestCapsuleService(capsule repository.TimeCapsuleRepository, reply repository.CapsuleReplyRepository, user repository.UserRepository) TimeCapsuleService {
	return NewTimeCapsuleService(capsule, reply, user, &mockAudit{}, testLogger())
}

// TestCreateCapsuleRecipient 封存时指定收件人：自己被拒、账号不存在被拒、合法收件人写入成功。
func TestCreateCapsuleRecipient(t *testing.T) {
	owner := &model.User{ID: 1, Username: "alice"}
	bob := &model.User{ID: 2, Username: "bob"}
	users := &mockUserRepo{
		findByIDFn: func(id uint64) (*model.User, error) { return owner, nil },
		findByUsernameFn: func(name string) (*model.User, error) {
			if name == "bob" {
				return bob, nil
			}
			return nil, repository.ErrNotFound
		},
	}
	capsuleRepo := &mockCapsuleRepo{createFn: func(c *model.TimeCapsule) error { c.ID = 10; return nil }}
	svc := newTestCapsuleService(capsuleRepo, &mockReplyRepo{}, users)

	future := time.Now().Add(24 * time.Hour)
	baseReq := dto.CreateCapsuleRequest{
		Title: "给未来的我们", Content: "这是一封长一点的信，至少五个字",
		UnlockAt: future,
	}

	t.Run("不能指定自己", func(t *testing.T) {
		req := baseReq
		req.RecipientUsername = "alice"
		_, err := svc.Create(1, req, "127.0.0.1", "req-1")
		if err == nil {
			t.Fatal("expected error when recipient is self")
		}
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeCapsuleRecipientSelf {
			t.Fatalf("expected CodeCapsuleRecipientSelf, got %v", err)
		}
	})

	t.Run("账号不存在", func(t *testing.T) {
		req := baseReq
		req.RecipientUsername = "ghost"
		_, err := svc.Create(1, req, "127.0.0.1", "req-2")
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeRecipientNotFound {
			t.Fatalf("expected CodeRecipientNotFound, got %v", err)
		}
	})

	t.Run("合法收件人写入", func(t *testing.T) {
		req := baseReq
		req.RecipientUsername = " bob "
		capsule, err := svc.Create(1, req, "127.0.0.1", "req-3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.RecipientID == nil || *capsule.RecipientID != 2 || capsule.RecipientUsername != "bob" {
			t.Fatalf("recipient not persisted: %+v", capsule)
		}
	})

	t.Run("不指定收件人保持为空", func(t *testing.T) {
		capsule, err := svc.Create(1, baseReq, "127.0.0.1", "req-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.RecipientID != nil || capsule.RecipientUsername != "" {
			t.Fatalf("recipient should be empty: %+v", capsule)
		}
	})
}

// TestRecipientViewLocked 收件人解锁前只能看标题：GetByID 可访问，但 DTO 打码正文/图片/语音。
func TestRecipientViewLocked(t *testing.T) {
	locked := &model.TimeCapsule{
		ID: 10, UserID: 1, Title: "给鲍勃的信", Content: "秘密正文",
		ImageURLs: `["http://img/1.png"]`, AudioURL: "http://audio/1.mp3",
		UnlockAt: time.Now().Add(24 * time.Hour), Status: constants.CapsuleStatusLocked,
		RecipientID: u64ptr(2), RecipientUsername: "bob",
	}
	capsuleRepo := &mockCapsuleRepo{findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return locked, nil }}
	replyRepo := &mockReplyRepo{findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) { return nil, repository.ErrNotFound }}
	svc := newTestCapsuleService(capsuleRepo, replyRepo, &mockUserRepo{})

	view, err := svc.GetByID(2, 10)
	if err != nil {
		t.Fatalf("recipient should view locked capsule: %v", err)
	}
	resp := dto.ToCapsuleResponse(dto.CapsuleResponseInput{Capsule: view.Capsule, ViewerIsOwner: false})
	if resp.Content != "" || resp.AudioURL != "" || len(resp.ImageURLs) != 0 {
		t.Fatalf("locked content must be masked for recipient: %+v", resp)
	}
	if resp.Title != "给鲍勃的信" || resp.Role != "recipient" {
		t.Fatalf("title/role wrong: %+v", resp)
	}

	// 陌生人无权访问
	if _, err := svc.GetByID(99, 10); err == nil {
		t.Fatal("stranger must not view capsule")
	}
}

// TestReplyRules 回信规则：解锁后可回一次；重复回信被拒；未解锁不能回；非收件人不能回。
func TestReplyRules(t *testing.T) {
	unlocked := &model.TimeCapsule{
		ID: 10, UserID: 1, Title: "给鲍勃的信", Content: "正文",
		UnlockAt: time.Now().Add(-time.Hour), Status: constants.CapsuleStatusUnlocked,
		RecipientID: u64ptr(2), RecipientUsername: "bob",
	}
	capsuleRepo := &mockCapsuleRepo{
		findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return unlocked, nil },
		updateFn:   func(*model.TimeCapsule) error { return nil },
	}

	t.Run("解锁后首次回信成功", func(t *testing.T) {
		var saved *model.CapsuleReply
		replyRepo := &mockReplyRepo{
			findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) { return nil, repository.ErrNotFound },
			createFn:        func(r *model.CapsuleReply) error { r.ID = 1; saved = r; return nil },
		}
		svc := newTestCapsuleService(capsuleRepo, replyRepo, &mockUserRepo{})
		reply, err := svc.SendReply(2, 10, dto.CapsuleReplyRequest{Content: "收到啦"}, "127.0.0.1", "r1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reply.Status != constants.ReplyStatusActive || saved.UserID != 2 {
			t.Fatalf("reply not persisted correctly: %+v", reply)
		}
	})

	t.Run("重复回信被拒（含已撤回）", func(t *testing.T) {
		replyRepo := &mockReplyRepo{
			findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) {
				return &model.CapsuleReply{CapsuleID: 10, Status: constants.ReplyStatusWithdrawn}, nil
			},
		}
		svc := newTestCapsuleService(capsuleRepo, replyRepo, &mockUserRepo{})
		_, err := svc.SendReply(2, 10, dto.CapsuleReplyRequest{Content: "再回一次"}, "127.0.0.1", "r2")
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeReplyAlreadySent {
			t.Fatalf("expected CodeReplyAlreadySent, got %v", err)
		}
	})

	t.Run("未解锁不能回信（访问触发自动解锁判定）", func(t *testing.T) {
		locked := &model.TimeCapsule{
			ID: 11, UserID: 1, UnlockAt: time.Now().Add(24 * time.Hour),
			Status: constants.CapsuleStatusLocked, RecipientID: u64ptr(2),
		}
		localCapsule := &mockCapsuleRepo{
			findByIDFn: func(id uint64) (*model.TimeCapsule, error) { return locked, nil },
			updateFn:   func(*model.TimeCapsule) error { return nil },
		}
		replyRepo := &mockReplyRepo{findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) { return nil, repository.ErrNotFound }}
		svc := newTestCapsuleService(localCapsule, replyRepo, &mockUserRepo{})
		_, err := svc.SendReply(2, 11, dto.CapsuleReplyRequest{Content: "提前回"}, "127.0.0.1", "r3")
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Code != constants.CodeReplyLocked {
			t.Fatalf("expected CodeReplyLocked, got %v", err)
		}
	})

	t.Run("主人不能代替收件人回信", func(t *testing.T) {
		replyRepo := &mockReplyRepo{findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) { return nil, repository.ErrNotFound }}
		svc := newTestCapsuleService(capsuleRepo, replyRepo, &mockUserRepo{})
		if _, err := svc.SendReply(1, 10, dto.CapsuleReplyRequest{Content: "自导自演"}, "127.0.0.1", "r4"); err == nil {
			t.Fatal("owner must not send reply")
		}
	})
}

// TestReplyWithdraw 主人撤回：非主人被拒；撤回后主人可见原文、收件人只见 withdrawn。
func TestReplyWithdraw(t *testing.T) {
	capsule := &model.TimeCapsule{ID: 10, UserID: 1, RecipientID: u64ptr(2)}
	capsuleRepo := &mockCapsuleRepo{findByIDFn: func(uint64) (*model.TimeCapsule, error) { return capsule, nil }}
	reply := &model.CapsuleReply{ID: 1, CapsuleID: 10, UserID: 2, Content: "给主人的回信", Status: constants.ReplyStatusActive}
	replyRepo := &mockReplyRepo{
		findByCapsuleFn: func(uint64) (*model.CapsuleReply, error) { return reply, nil },
		updateFn:        func(r *model.CapsuleReply) error { reply = r; return nil },
	}
	svc := newTestCapsuleService(capsuleRepo, replyRepo, &mockUserRepo{})

	if err := svc.WithdrawReply(2, 10, "127.0.0.1", "w1"); err == nil {
		t.Fatal("recipient must not withdraw reply")
	}
	if err := svc.WithdrawReply(1, 10, "127.0.0.1", "w2"); err != nil {
		t.Fatalf("owner withdraw failed: %v", err)
	}
	if reply.Status != constants.ReplyStatusWithdrawn || reply.WithdrawnAt == nil {
		t.Fatalf("reply not withdrawn: %+v", reply)
	}

	// 主人视角仍可见正文
	ownerResp := dto.ToCapsuleReplyResponse(reply, false)
	if ownerResp.Content != "给主人的回信" {
		t.Fatalf("owner should keep reply content: %+v", ownerResp)
	}
	// 收件人视角正文打码
	recipientResp := dto.ToCapsuleReplyResponse(reply, true)
	if recipientResp.Content != "" || recipientResp.Status != constants.ReplyStatusWithdrawn {
		t.Fatalf("recipient must see withdrawn only: %+v", recipientResp)
	}

	// 重复撤回被拒
	if err := svc.WithdrawReply(1, 10, "127.0.0.1", "w3"); err == nil {
		t.Fatal("double withdraw must fail")
	}
}
