package service

import (
	"errors"
	"log/slog"
	"time"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/dto"
	"github.com/wishwall/wishwall/internal/model"
	"github.com/wishwall/wishwall/internal/repository"
	"github.com/wishwall/wishwall/internal/util"
)

// CapsuleView 胶囊详情聚合：胶囊本体 + 可能存在的回信 + 主人资料（收件人视角展示）。
type CapsuleView struct {
	Capsule *model.TimeCapsule
	Reply   *model.CapsuleReply
	Owner   *model.User
}

// TimeCapsuleService 时光胶囊服务：封存（可指定收件人）/查看（双方视角）/回信/撤回/删除/自动解锁。
type TimeCapsuleService interface {
	Create(userID uint64, req dto.CreateCapsuleRequest, ip, requestID string) (*model.TimeCapsule, error)
	ListMine(userID uint64, q dto.PageQuery) (*dto.PageResult, error)
	ListReceived(userID uint64, q dto.PageQuery) (*dto.PageResult, error)
	GetByID(userID, capsuleID uint64) (*CapsuleView, error)
	SendReply(userID, capsuleID uint64, req dto.CapsuleReplyRequest, ip, requestID string) (*model.CapsuleReply, error)
	WithdrawReply(userID, capsuleID uint64, ip, requestID string) error
	Delete(userID, capsuleID uint64, ip, requestID string) error
	UnlockDue() ([]model.TimeCapsule, error)
}

type timeCapsuleService struct {
	capsule repository.TimeCapsuleRepository
	reply   repository.CapsuleReplyRepository
	user    repository.UserRepository
	audit   AuditService
	logger  *slog.Logger
}

// NewTimeCapsuleService 构造胶囊服务。
func NewTimeCapsuleService(capsule repository.TimeCapsuleRepository, reply repository.CapsuleReplyRepository, user repository.UserRepository, audit AuditService, logger *slog.Logger) TimeCapsuleService {
	return &timeCapsuleService{capsule: capsule, reply: reply, user: user, audit: audit, logger: logger}
}

func (s *timeCapsuleService) Create(userID uint64, req dto.CreateCapsuleRequest, ip, requestID string) (*model.TimeCapsule, error) {
	if req.UnlockAt.Before(time.Now()) {
		return nil, util.NewAppError(constants.CodeBadRequest, "解锁时间必须晚于当前时间", errors.New("unlock_at in past"))
	}
	capsule := &model.TimeCapsule{
		UserID:    userID,
		Title:     req.Title,
		Content:   req.Content,
		ImageURLs: util.EncodeStringArray(req.ImageURLs),
		AudioURL:  req.AudioURL,
		UnlockAt:  req.UnlockAt,
		Status:    constants.CapsuleStatusLocked,
	}
	// 指定收件人：账号必须存在，且不能是自己
	if recipientName := req.NormalizedRecipient(); recipientName != "" {
		me, err := s.user.FindByID(userID)
		if err != nil {
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		if recipientName == me.Username {
			return nil, util.NewAppError(constants.CodeCapsuleRecipientSelf, constants.MsgCapsuleRecipientSelf, errors.New("capsule recipient is self"))
		}
		recipient, err := s.user.FindByUsername(recipientName)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, util.NewAppError(constants.CodeRecipientNotFound, constants.MsgRecipientNotFound, err)
			}
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		capsule.RecipientID = &recipient.ID
		capsule.RecipientUsername = recipient.Username
	}
	if err := s.capsule.Create(capsule); err != nil {
		s.logger.Error(constants.LogCapsuleCreateFail, "user_id", userID, "error", err)
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleCreated, "capsule_id", capsule.ID, "user_id", userID, "recipient_id", capsule.RecipientID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "create_capsule", EntityType: "time_capsule", EntityID: u64str(capsule.ID),
		Detail: s.createAuditDetail(capsule), IP: ip, RequestID: requestID,
	})
	return capsule, nil
}

func (s *timeCapsuleService) createAuditDetail(c *model.TimeCapsule) string {
	if c.RecipientID != nil {
		return "封存时光胶囊，收件人：" + c.RecipientUsername
	}
	return "封存时光胶囊"
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
	resps, err := s.buildResponses(items, true)
	if err != nil {
		return nil, err
	}
	return &dto.PageResult{Items: resps, Total: total, Page: page, PageSize: size}, nil
}

// ListReceived 「收到的」胶囊：当前用户作为收件人，未解锁也可见标题与剩余时间。
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
	resps, err := s.buildResponses(items, false)
	if err != nil {
		return nil, err
	}
	return &dto.PageResult{Items: resps, Total: total, Page: page, PageSize: size}, nil
}

// buildResponses 批量组装列表返回：补全回信状态与（收件人视角的）主人昵称。
func (s *timeCapsuleService) buildResponses(items []model.TimeCapsule, viewerIsOwner bool) ([]dto.CapsuleResponse, error) {
	ids := make([]uint64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	replies, err := s.reply.ListByCapsuleIDs(ids)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	replyMap := make(map[uint64]model.CapsuleReply, len(replies))
	for i := range replies {
		replyMap[replies[i].CapsuleID] = replies[i]
	}
	// 收件人视角：批量查询主人资料
	ownerMap := map[uint64]model.User{}
	if !viewerIsOwner && len(items) > 0 {
		ownerIDs := make([]uint64, 0, len(items))
		seen := map[uint64]bool{}
		for i := range items {
			if !seen[items[i].UserID] {
				seen[items[i].UserID] = true
				ownerIDs = append(ownerIDs, items[i].UserID)
			}
		}
		users, err := s.user.FindByIDs(ownerIDs)
		if err != nil {
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		for _, u := range users {
			ownerMap[u.ID] = u
		}
	}
	resps := make([]dto.CapsuleResponse, 0, len(items))
	for i := range items {
		c := &items[i]
		var reply *model.CapsuleReply
		if r, ok := replyMap[c.ID]; ok {
			reply = &r
		}
		in := dto.CapsuleResponseInput{Capsule: c, ViewerIsOwner: viewerIsOwner, Reply: reply}
		if u, ok := ownerMap[c.UserID]; ok {
			in.OwnerUsername = u.Username
			in.OwnerNickname = u.Nickname
		}
		resps = append(resps, dto.ToCapsuleResponse(in))
	}
	return resps, nil
}

// GetByID 查看胶囊：主人或指定收件人可访问；访问时顺带完成到期自动解锁。
func (s *timeCapsuleService) GetByID(userID, capsuleID uint64) (*CapsuleView, error) {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if !s.canView(capsule, userID) {
		return nil, util.NewAppError(constants.CodeForbidden, constants.MsgNeedLogin+"：无权查看该胶囊", errors.New("capsule visibility forbidden"))
	}
	s.autoUnlock(capsule, userID)
	view := &CapsuleView{Capsule: capsule}
	if capsule.UserID != userID {
		owner, err := s.user.FindByID(capsule.UserID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
		}
		view.Owner = owner
	}
	reply, err := s.reply.FindByCapsuleID(capsuleID)
	if err == nil {
		view.Reply = reply
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	return view, nil
}

// canView 仅主人与指定收件人可见。
func (s *timeCapsuleService) canView(capsule *model.TimeCapsule, userID uint64) bool {
	if capsule.UserID == userID {
		return true
	}
	return capsule.RecipientID != nil && *capsule.RecipientID == userID
}

// autoUnlock 到期自动解锁（主人或收件人首次访问时触发，与定时任务等价）。
func (s *timeCapsuleService) autoUnlock(capsule *model.TimeCapsule, userID uint64) {
	if capsule.Status != constants.CapsuleStatusLocked {
		return
	}
	now := time.Now()
	if capsule.UnlockAt.After(now) {
		return
	}
	capsule.Status = constants.CapsuleStatusUnlocked
	capsule.UnlockedAt = &now
	if err := s.capsule.Update(capsule); err != nil {
		s.logger.Warn("auto unlock capsule failed", "capsule_id", capsule.ID, "error", err)
		return
	}
	s.logger.Info(constants.LogCapsuleUnlocked, "capsule_id", capsule.ID, "trigger_user_id", userID)
}

// SendReply 收件人回信：胶囊须已解锁，且每封胶囊只能回复一次（发出后不可改）。
func (s *timeCapsuleService) SendReply(userID, capsuleID uint64, req dto.CapsuleReplyRequest, ip, requestID string) (*model.CapsuleReply, error) {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if capsule.RecipientID == nil || *capsule.RecipientID != userID {
		return nil, util.NewAppError(constants.CodeForbidden, "只有该胶囊指定的收件人才能回信", errors.New("capsule reply not recipient"))
	}
	s.autoUnlock(capsule, userID)
	if capsule.Status == constants.CapsuleStatusLocked {
		return nil, util.NewAppError(constants.CodeReplyLocked, constants.MsgCapsuleLocked+"，解锁后才能回信", errors.New("capsule still locked"))
	}
	if existing, err := s.reply.FindByCapsuleID(capsuleID); err == nil && existing != nil {
		// 无论回信当前是有效还是已被主人撤回，收件人均不能再发第二封
		return nil, util.NewAppError(constants.CodeReplyAlreadySent, constants.MsgReplyAlreadySent, errors.New("capsule reply already exists"))
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	reply := &model.CapsuleReply{
		CapsuleID: capsuleID,
		UserID:    userID,
		Content:   req.Content,
		Status:    constants.ReplyStatusActive,
	}
	if err := s.reply.Create(reply); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, util.NewAppError(constants.CodeReplyAlreadySent, constants.MsgReplyAlreadySent, err)
		}
		s.logger.Error(constants.LogCapsuleReplyFail, "capsule_id", capsuleID, "user_id", userID, "error", err)
		return nil, util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleReplySent, "capsule_id", capsuleID, "user_id", userID, "reply_id", reply.ID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "reply_capsule", EntityType: "time_capsule", EntityID: u64str(capsuleID),
		Detail: "回复时光胶囊", IP: ip, RequestID: requestID,
	})
	return reply, nil
}

// WithdrawReply 主人撤回收件人的回信：撤回后收件人不可见，主人仍保留原文与撤回记录。
func (s *timeCapsuleService) WithdrawReply(userID, capsuleID uint64, ip, requestID string) error {
	capsule, err := s.capsule.FindByID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeCapsuleNotFound, "时光胶囊不存在", err)
		}
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if capsule.UserID != userID {
		return util.NewAppError(constants.CodeReplyNotOwner, constants.MsgReplyNotOwner, errors.New("capsule reply withdraw not owner"))
	}
	reply, err := s.reply.FindByCapsuleID(capsuleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeReplyNotFound, constants.MsgReplyNotFound, err)
		}
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	if reply.Status == constants.ReplyStatusWithdrawn {
		return util.NewAppError(constants.CodeReplyNotFound, "回信已撤回，不能重复操作", errors.New("capsule reply already withdrawn"))
	}
	now := time.Now()
	reply.Status = constants.ReplyStatusWithdrawn
	reply.WithdrawnAt = &now
	if err := s.reply.Update(reply); err != nil {
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
	}
	s.logger.Info(constants.LogCapsuleReplyWithdrawn, "capsule_id", capsuleID, "reply_id", reply.ID, "user_id", userID)
	_ = s.audit.Record(&model.AuditLog{
		UserID: userID, Action: "withdraw_capsule_reply", EntityType: "time_capsule", EntityID: u64str(capsuleID),
		Detail: "撤回胶囊回信", IP: ip, RequestID: requestID,
	})
	return nil
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
	// 级联清理回信（回信独立成表，不依赖数据库外键级联）
	if err := s.reply.DeleteByCapsuleID(capsuleID); err != nil {
		return util.NewAppError(constants.CodeInternalError, constants.MsgInternalError, err)
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
