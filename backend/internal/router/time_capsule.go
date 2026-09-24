package router

import (
	"github.com/gin-gonic/gin"

	"github.com/wishwall/wishwall/internal/handler"
)

// RegisterTimeCapsuleRoutes 注册时光胶囊路由（封存/我封存的/收到的/详情/回信/撤回/删除）。
func RegisterTimeCapsuleRoutes(rg *gin.RouterGroup, h *handler.TimeCapsuleHandler, auth gin.HandlerFunc) {
	rg.POST("/capsules", auth, h.Create)
	rg.GET("/capsules/mine", auth, h.ListMine)
	rg.GET("/capsules/received", auth, h.ListReceived)
	rg.GET("/capsules/:id", auth, h.GetByID)
	rg.POST("/capsules/:id/reply", auth, h.SendReply)
	rg.DELETE("/capsules/:id/reply", auth, h.WithdrawReply)
	rg.DELETE("/capsules/:id", auth, h.Delete)
}
