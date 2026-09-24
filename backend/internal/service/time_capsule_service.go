package service

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/dto"
	"github.com/wishwall/wishwall/internal/model"
	"github.com/wishwall/wishwall/internal/repository"
	"github.com/wishwall/wishwall/internal/util"
)

// TimeCapsuleService 时光胶囊服务：封存/查看/解锁/删除，支持指定收件人与一次性回信。
type TimeCapsuleService interface {
	Create(userID uint64, req dto.CreateCapsuleRequest, ip, requestID string) (*model.TimeCapsule, error)
	ListMine(userID uint64, q dto.PageQuery) (*dto.PageResult, error)
	ListReceived(userID uint64, q dto.PageQuery) (*dto.PageResult, error)
	GetByID(userID, capsuleID uint64) (*model.TimeCapsule, *dto.CapsuleResponseNames, error)
	Delete(userID, capsuleID uint64, ip, requestID string) error
	Reply(userID, capsuleID uint64, req dto.CapsuleReplyRequest, ip, requestID string) (*model.TimeCapsule, error)
	WithdrawReply(userID, capsuleID uint64, ip, requestID string) (*model.TimeCapsule, error)
	UnlockDue() ([]model.TimeCapsule, error)
}

type timeCapsuleService struct {
	capsule repository.TimeCapsuleRepository
	users   repository.UserRepository
	audit   AuditService
	logger  *slog.Logger
}

// NewTimeCapsuleService 构造胶囊服务。
func NewTimeCapsuleService(capsule repository.TimeCapsuleRepository, users repository.UserRepository, audit AuditService, logger *slog.Logger) TimeCapsuleService {
	return &timeCapsuleService{capsule: capsule, users: users, audit: audit, logger: logger}
}

func (s *timeCapsuleService) Create(userID uint64, req dto.CreateCapsuleRequest, ip, requestID string) (*model.TimeCapsule, error) {
	if req.UnlockAt.Before(time.Now()) {
		return nil, util.NewAppError(constants.CodeBadRequest, "解锁时间必须晚于当前时间", errors.New("unlock_at in past"))
	}
	var recipientID *uint64
	if name := strings.TrimSpace(req.RecipientUsername); name != "" {
		recipient, err := s.users.FindByUsername(name)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, util.NewAppError(constants.CodeCapsuleRecipientNotFound, constants.MsgCapsuleRecipientNotFound, err)
			}
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		if recipient.ID == userID {
			return nil, util.NewAppError(constants.CodeCapsuleRecipientSelf, constants.MsgCapsuleRecipientSelf, errors.New("recipient is owner"))
		}
		recipientID = &recipient.ID
	}
	capsule := &model.TimeCapsule{
		UserID:      userID,
		RecipientID: recipientID,
		Title:       req.Title,
		Content:     req.Content,
		ImageURLs:   util.EncodeStringArray(req.ImageURLs),
		AudioURL:    req.AudioURL,
		UnlockAt:    req.UnlockAt,
		Status:      constants.CapsuleStatusLocked,
	}
	if err := s.capsule.Create(capsule); err != nil {
		s.logger.Error(constants.LogCapsuleCreateFail, "user_id", userID, "error", err)
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleCreated, "capsule_id", capsule.ID, "user_id", userID, "recipient_id", recipientID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "create_capsule", EntityType: "time_capsule", EntityID: u64str(capsule.ID),
		Detail: "封存时光胶囊", IP: ip, RequestID: requestID,
	})
	return capsule, nil
}

func (s *timeCapsuleService) ListMine(userID uint64, q dto.PageQuery) (*dto.PageResult, error) {
	page, size := q.Normalize()
	total, err := s.capsule.CountByUserID(userID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	items, err := s.capsule.ListByUserID(userID, (page-1)*size, size)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	resps, err := s.toResponses(items, userID)
	if err != nil {
		return nil, err
	}
	return &dto.PageResult{Items: resps, Total: total, Page: page, PageSize: size}, nil
}

// ListReceived 我作为收件人收到的胶囊。
func (s *timeCapsuleService) ListReceived(userID uint64, q dto.PageQuery) (*dto.PageResult, error) {
	page, size := q.Normalize()
	total, err := s.capsule.CountByRecipientID(userID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	items, err := s.capsule.ListByRecipientID(userID, (page-1)*size, size)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	resps, err := s.toResponses(items, userID)
	if err != nil {
		return nil, err
	}
	return &dto.PageResult{Items: resps, Total: total, Page: page, PageSize: size}, nil
}

// toResponses 批量补全主人/收件人用户名后构造返回结构。
func (s *timeCapsuleService) toResponses(items []model.TimeCapsule, viewerID uint64) ([]dto.CapsuleResponse, error) {
	userCache := map[uint64]*model.User{}
	resolve := func(id uint64) (*model.User, error) {
		if u, ok := userCache[id]; ok {
			return u, nil
		}
		u, err := s.users.FindByID(id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return &model.User{ID: id, Username: "unknown", Nickname: "已注销用户"}, nil
			}
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		userCache[id] = u
		return u, nil
	}
	resps := make([]dto.CapsuleResponse, 0, len(items))
	for i := range items {
		c := &items[i]
		names, err := s.resolveNames(c, resolve)
		if err != nil {
			return nil, err
		}
		resps = append(resps, dto.ToCapsuleResponse(c, viewerID, names))
	}
	return resps, nil
}

func (s *timeCapsuleService) resolveNames(c *model.TimeCapsule, resolve func(uint64) (*model.User, error)) (dto.CapsuleResponseNames, error) {
	var names dto.CapsuleResponseNames
	owner, err := resolve(c.UserID)
	if err != nil {
		return names, err
	}
	names.OwnerUsername = owner.Username
	names.OwnerNickname = owner.Nickname
	if c.RecipientID != nil {
		recipient, err := resolve(*c.RecipientID)
		if err != nil {
			return names, err
		}
		names.RecipientUsername = recipient.Username
		names.RecipientNickname = recipient.Nickname
	}
	return names, nil
}

// GetByID 查看胶囊：主人与收件人可见；解锁前内容打码，到期自动解锁。
func (s *timeCapsuleService) GetByID(userID, capsuleID uint64) (*model.TimeCapsule, *dto.CapsuleResponseNames, error) {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return nil, nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if !s.canView(capsule, userID) {
		return nil, nil, util.NewAppError(constants.CodeForbidden, constants.MsgNeedLogin+"：无权查看该胶囊", errors.New("capsule visibility forbidden"))
	}
	s.autoUnlock(capsule, userID)
	names, err := s.resolveNames(capsule, func(id uint64) (*model.User, error) {
		u, e := s.users.FindByID(id)
		if e != nil && errors.Is(e, repository.ErrNotFound) {
			return &model.User{ID: id, Username: "unknown", Nickname: "已注销用户"}, nil
		}
		return u, e
	})
	if err != nil {
		return nil, nil, err
	}
	return capsule, &names, nil
}

func (s *timeCapsuleService) canView(capsule *model.TimeCapsule, userID uint64) bool {
	if capsule.UserID == userID {
		return true
	}
	return capsule.RecipientID != nil && *capsule.RecipientID == userID
}

// autoUnlock 到期胶囊在双方任一方访问时自动解锁并持久化。
func (s *timeCapsuleService) autoUnlock(capsule *model.TimeCapsule, userID uint64) {
	if capsule.Status == constants.CapsuleStatusLocked && !capsule.UnlockAt.After(time.Now()) {
		now := time.Now()
		capsule.Status = constants.CapsuleStatusUnlocked
		capsule.UnlockedAt = &now
		if err := s.capsule.Update(capsule); err != nil {
			s.logger.Warn("capsule auto unlock persist failed", "capsule_id", capsule.ID, "error", err)
			return
		}
		s.logger.Info(constants.LogCapsuleUnlocked, "capsule_id", capsule.ID, "user_id", userID)
	}
}

func (s *timeCapsuleService) Delete(userID, capsuleID uint64, ip, requestID string) error {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if capsule.UserID != userID {
		return util.NewAppError(constants.CodeForbidden, constants.MsgNeedLogin+"：无权删除该胶囊", errors.New("capsule owner mismatch"))
	}
	if err := s.capsule.Delete(capsuleID); err != nil {
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleDeleted, "capsule_id", capsuleID, "user_id", userID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "delete_capsule", EntityType: "time_capsule", EntityID: u64str(capsuleID),
		Detail: "删除时光胶囊", IP: ip, RequestID: requestID,
	})
	return nil
}

// Reply 收件人在胶囊解锁后回信，仅允许一次，发出后不可修改。
func (s *timeCapsuleService) Reply(userID, capsuleID uint64, req dto.CapsuleReplyRequest, ip, requestID string) (*model.TimeCapsule, error) {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if capsule.RecipientID == nil || *capsule.RecipientID != userID {
		return nil, util.NewAppError(constants.CodeCapsuleReplyForbidden, constants.MsgCapsuleReplyForbidden, errors.New("reply forbidden"))
	}
	if capsule.UserID == userID {
		return nil, util.NewAppError(constants.CodeCapsuleReplyForbidden, constants.MsgCapsuleReplyForbidden, errors.New("owner cannot reply"))
	}
	s.autoUnlock(capsule, userID)
	if capsule.Status == constants.CapsuleStatusLocked {
		return nil, util.NewAppError(constants.CodeCapsuleLocked, constants.MsgCapsuleLocked, errors.New("capsule locked"))
	}
	if capsule.ReplyAt != nil {
		return nil, util.NewAppError(constants.CodeCapsuleReplyExists, constants.MsgCapsuleReplyExists, errors.New("reply already sent"))
	}
	now := time.Now()
	capsule.ReplyContent = strings.TrimSpace(req.Content)
	capsule.ReplyAt = &now
	capsule.ReplyWithdrawn = false
	if err := s.capsule.Update(capsule); err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleReplied, "capsule_id", capsuleID, "user_id", userID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "reply_capsule", EntityType: "time_capsule", EntityID: u64str(capsuleID),
		Detail: "时光胶囊回信", IP: ip, RequestID: requestID,
	})
	return capsule, nil
}

// WithdrawReply 胶囊主人撤回收件人的回信；撤回后双方都看不到回信正文。
func (s *timeCapsuleService) WithdrawReply(userID, capsuleID uint64, ip, requestID string) (*model.TimeCapsule, error) {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if capsule.UserID != userID {
		return nil, util.NewAppError(constants.CodeForbidden, "只有胶囊主人才能撤回回信", errors.New("withdraw reply forbidden"))
	}
	if capsule.ReplyAt == nil {
		return nil, util.NewAppError(constants.CodeCapsuleReplyMissing, constants.MsgCapsuleReplyMissing, errors.New("no reply"))
	}
	if capsule.ReplyWithdrawn {
		return nil, util.NewAppError(constants.CodeCapsuleReplyMissing, "回信已经撤回", errors.New("reply already withdrawn"))
	}
	capsule.ReplyContent = ""
	capsule.ReplyWithdrawn = true
	if err := s.capsule.Update(capsule); err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleReplyWithdrawn, "capsule_id", capsuleID, "user_id", userID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "withdraw_capsule_reply", EntityType: "time_capsule", EntityID: u64str(capsuleID),
		Detail: "撤回时光胶囊回信", IP: ip, RequestID: requestID,
	})
	return capsule, nil
}

// UnlockDue 解锁所有到期胶囊（启动时与定时任务调用）。
func (s *timeCapsuleService) UnlockDue() ([]model.TimeCapsule, error) {
	items, err := s.capsule.UnlockDue(time.Now())
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	now := time.Now()
	for i := range items {
		items[i].Status = constants.CapsuleStatusUnlocked
		items[i].UnlockedAt = &now
		if err := s.capsule.Update(&items[i]); err != nil {
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		s.logger.Info(constants.LogCapsuleUnlocked, "capsule_id", items[i].ID)
	}
	return items, nil
}
