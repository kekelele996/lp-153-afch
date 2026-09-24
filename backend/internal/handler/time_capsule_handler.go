package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/dto"
	"github.com/wishwall/wishwall/internal/middleware"
	"github.com/wishwall/wishwall/internal/service"
)

// TimeCapsuleHandler 时光胶囊处理器。
type TimeCapsuleHandler struct {
	capsule service.TimeCapsuleService
}

// NewTimeCapsuleHandler 构造胶囊处理器。
func NewTimeCapsuleHandler(capsule service.TimeCapsuleService) *TimeCapsuleHandler {
	return &TimeCapsuleHandler{capsule: capsule}
}

// Create POST /api/v1/capsules
func (h *TimeCapsuleHandler) Create(c *gin.Context) {
	var req dto.CreateCapsuleRequest
	if !bindJSON(c, &req) {
		return
	}
	userID := middleware.CurrentUserID(c)
	capsule, err := h.capsule.Create(userID, req, c.ClientIP(), middleware.GetRequestID(c))
	if err != nil {
		// service 已按「时光胶囊-收件人用户名」拼接错误信息（账号不存在/不能选自己），handler 再次包装透传。
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": constants.MsgCapsuleCreated, "data": dto.ToCapsuleResponse(dto.CapsuleResponseInput{Capsule: capsule, ViewerIsOwner: true})})
}

// ListMine GET /api/v1/capsules/mine —— 「我封存的」
func (h *TimeCapsuleHandler) ListMine(c *gin.Context) {
	var q dto.PageQuery
	if !bindQuery(c, &q) {
		return
	}
	userID := middleware.CurrentUserID(c)
	result, err := h.capsule.ListMine(userID, q)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": result})
}

// ListReceived GET /api/v1/capsules/received —— 「收到的」
func (h *TimeCapsuleHandler) ListReceived(c *gin.Context) {
	var q dto.PageQuery
	if !bindQuery(c, &q) {
		return
	}
	userID := middleware.CurrentUserID(c)
	result, err := h.capsule.ListReceived(userID, q)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": result})
}

// GetByID GET /api/v1/capsules/:id
func (h *TimeCapsuleHandler) GetByID(c *gin.Context) {
	capsuleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		responseError(c, 400, constants.CodeBadRequest, "胶囊 id 参数非法")
		return
	}
	userID := middleware.CurrentUserID(c)
	view, err := h.capsule.GetByID(userID, capsuleID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": h.toResponse(userID, view)})
}

// SendReply POST /api/v1/capsules/:id/reply —— 收件人回信一次
func (h *TimeCapsuleHandler) SendReply(c *gin.Context) {
	capsuleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		responseError(c, 400, constants.CodeBadRequest, "胶囊 id 参数非法")
		return
	}
	var req dto.CapsuleReplyRequest
	if !bindJSON(c, &req) {
		return
	}
	userID := middleware.CurrentUserID(c)
	reply, err := h.capsule.SendReply(userID, capsuleID, req, c.ClientIP(), middleware.GetRequestID(c))
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": constants.MsgReplySent, "data": dto.ToCapsuleReplyResponse(reply, false)})
}

// WithdrawReply DELETE /api/v1/capsules/:id/reply —— 主人撤回回信
func (h *TimeCapsuleHandler) WithdrawReply(c *gin.Context) {
	capsuleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		responseError(c, 400, constants.CodeBadRequest, "胶囊 id 参数非法")
		return
	}
	userID := middleware.CurrentUserID(c)
	if err := h.capsule.WithdrawReply(userID, capsuleID, c.ClientIP(), middleware.GetRequestID(c)); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": constants.MsgReplyWithdrawn, "data": nil})
}

// Delete DELETE /api/v1/capsules/:id
func (h *TimeCapsuleHandler) Delete(c *gin.Context) {
	capsuleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		responseError(c, 400, constants.CodeBadRequest, "胶囊 id 参数非法")
		return
	}
	userID := middleware.CurrentUserID(c)
	if err := h.capsule.Delete(userID, capsuleID, c.ClientIP(), middleware.GetRequestID(c)); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": constants.MsgDeleteSuccess, "data": nil})
}

// toResponse 详情按当前用户视角（主人/收件人）组装打码后的返回结构。
func (h *TimeCapsuleHandler) toResponse(userID uint64, view *service.CapsuleView) dto.CapsuleResponse {
	in := dto.CapsuleResponseInput{
		Capsule:       view.Capsule,
		ViewerIsOwner: view.Capsule.UserID == userID,
		Reply:         view.Reply,
	}
	if view.Owner != nil {
		in.OwnerUsername = view.Owner.Username
		in.OwnerNickname = view.Owner.Nickname
	}
	return dto.ToCapsuleResponse(in)
}
