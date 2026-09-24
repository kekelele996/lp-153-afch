package constants

// 胶囊回信状态机：active -> withdrawn（收件人发出后不可修改，仅胶囊主人可撤回）。
const (
	ReplyStatusActive    = "active"
	ReplyStatusWithdrawn = "withdrawn"
)

// ValidReplyStatuses 回信状态白名单。
var ValidReplyStatuses = []string{ReplyStatusActive, ReplyStatusWithdrawn}

// ReplyStatusText 回信状态文本。
func ReplyStatusText(s string) string {
	if s == ReplyStatusWithdrawn {
		return "已撤回"
	}
	return "可查看"
}
