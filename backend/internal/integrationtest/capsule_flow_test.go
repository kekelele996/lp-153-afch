package integration_tmp_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/wishwall/wishwall/internal/constants"
	"github.com/wishwall/wishwall/internal/handler"
	"github.com/wishwall/wishwall/internal/middleware"
	"github.com/wishwall/wishwall/internal/model"
	"github.com/wishwall/wishwall/internal/repository"
	"github.com/wishwall/wishwall/internal/router"
	"github.com/wishwall/wishwall/internal/service"
	"github.com/wishwall/wishwall/internal/util"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.TimeCapsule{}, &model.CapsuleReply{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	logger := util.InitLogger("test")
	userRepo := repository.NewUserRepository(db)
	capsuleRepo := repository.NewTimeCapsuleRepository(db)
	replyRepo := repository.NewCapsuleReplyRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	auditSvc := service.NewAuditService(auditRepo, userRepo, logger)
	capsuleSvc := service.NewTimeCapsuleService(capsuleRepo, replyRepo, userRepo, auditSvc, logger)

	gin.SetMode(gin.TestMode)
	h := &router.Handlers{
		Capsule: handler.NewTimeCapsuleHandler(capsuleSvc),
	}
	engine := gin.New()
	auth := middleware.Auth("test-secret")
	rg := engine.Group("/api/v1")
	router.RegisterTimeCapsuleRoutes(rg, h.Capsule, auth)
	return engine, db
}

func seedUser(t *testing.T, db *gorm.DB, id uint64, username string) {
	t.Helper()
	hash, _ := util.HashPassword("secret123")
	if err := db.Create(&model.User{ID: id, Username: username, Email: username + "@example.com", PasswordHash: hash, Nickname: username, Role: "user", Status: "active"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func token(t *testing.T, userID uint64, username string) string {
	t.Helper()
	tok, err := util.GenerateToken("test-secret", userID, username, "user", 1)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok
}

func do(t *testing.T, engine *gin.Engine, method, path, tok string, body any) apiResp {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var resp apiResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %s %s: %v, body=%s", method, path, err, rec.Body.String())
	}
	if resp.Code != 0 {
		t.Fatalf("%s %s expected code 0, got %d: %s", method, path, resp.Code, resp.Message)
	}
	return resp
}

func expectCode(t *testing.T, engine *gin.Engine, method, path, tok string, body any, wantCode int) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var resp apiResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if resp.Code != wantCode {
		t.Fatalf("%s %s expected code %d, got %d: %s", method, path, wantCode, resp.Code, resp.Message)
	}
}

// TestCapsuleRecipientFlowHTTP 覆盖完整业务链路：
// 指定收件人（自己/不存在被拒）→ 收件人收到胶囊但解锁前打码 → 到期后双方可读 → 回信一次 → 主人撤回。
func TestCapsuleRecipientFlowHTTP(t *testing.T) {
	engine, db := setupEngine(t)
	seedUser(t, db, 1, "alice")
	seedUser(t, db, 2, "bob")
	seedUser(t, db, 3, "carol")
	alice, bob, carol := token(t, 1, "alice"), token(t, 2, "bob"), token(t, 3, "carol")

	// 1. 不能指定自己
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules", alice, map[string]any{
		"title": "标题足够长", "content": "正文也至少五个字哦", "unlock_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"recipient_username": "alice",
	}, constants.CodeCapsuleRecipientSelf)

	// 2. 账号不存在，留在填写页（错误提示返回）
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules", alice, map[string]any{
		"title": "标题足够长", "content": "正文也至少五个字哦", "unlock_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"recipient_username": "ghost",
	}, constants.CodeRecipientNotFound)

	// 3. 合法封存，指定 bob
	create := do(t, engine, http.MethodPost, "/api/v1/capsules", alice, map[string]any{
		"title": "标题足够长", "content": "正文也至少五个字哦", "image_urls": []string{"http://img/1.png"},
		"audio_url": "http://a/1.mp3", "unlock_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"recipient_username": "bob",
	})
	var capsule struct {
		ID uint64 `json:"id"`
	}
	_ = json.Unmarshal(create.Data, &capsule)
	if capsule.ID == 0 {
		t.Fatal("capsule id missing")
	}

	// 4. 「收到的」列表：bob 可见，carol 列表为空
	bobList := do(t, engine, http.MethodGet, "/api/v1/capsules/received", bob, nil)
	var received struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	_ = json.Unmarshal(bobList.Data, &received)
	if received.Total != 1 || received.Items[0]["title"] != "标题足够长" {
		t.Fatalf("recipient list wrong: %s", string(bobList.Data))
	}
	if received.Items[0]["content"] != "" {
		t.Fatalf("locked content leaked in recipient list: %v", received.Items[0]["content"])
	}

	// 5. carol 无权访问详情
	expectCode(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), carol, nil, constants.CodeForbidden)

	// 6. bob 解锁前看详情：标题在，正文/图片/语音打码
	detail := do(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), bob, nil)
	var locked map[string]any
	_ = json.Unmarshal(detail.Data, &locked)
	if locked["content"] != "" || locked["audio_url"] != "" {
		t.Fatalf("locked detail leaked: %v", locked)
	}
	if imgs, _ := locked["image_urls"].([]any); len(imgs) != 0 {
		t.Fatalf("locked images leaked: %v", imgs)
	}
	if locked["title"] != "标题足够长" {
		t.Fatalf("title missing: %v", locked["title"])
	}

	// 7. 解锁前不能回信
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", bob,
		map[string]any{"content": "提前回"}, constants.CodeReplyLocked)

	// 8. 让胶囊到期；bob 访问触发自动解锁
	if err := db.Model(&model.TimeCapsule{}).Where("id = ?", capsule.ID).Update("unlock_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("fast-forward: %v", err)
	}
	unlockedDetail := do(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), bob, nil)
	var unlocked map[string]any
	_ = json.Unmarshal(unlockedDetail.Data, &unlocked)
	if unlocked["status"] != "unlocked" || unlocked["content"] != "正文也至少五个字哦" {
		t.Fatalf("unlocked detail wrong: %v", unlocked)
	}

	// 9. carol 仍无权访问
	expectCode(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), carol, nil, constants.CodeForbidden)

	// 10. bob 回信一次
	do(t, engine, http.MethodPost, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", bob,
		map[string]any{"content": "收到啦"})

	// 11. 再回被拒；alice 不能代回
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", bob,
		map[string]any{"content": "再回"}, constants.CodeReplyAlreadySent)
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", alice,
		map[string]any{"content": "我替他回"}, constants.CodeForbidden)

	// 12. alice 详情可见回信原文；撤回后 bob 只见 withdrawn
	ownerDetail := do(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), alice, nil)
	var od map[string]any
	_ = json.Unmarshal(ownerDetail.Data, &od)
	replyObj, _ := od["reply"].(map[string]any)
	if replyObj["content"] != "收到啦" {
		t.Fatalf("owner should see reply: %v", od["reply"])
	}
	do(t, engine, http.MethodDelete, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", alice, nil)

	bobDetail := do(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), bob, nil)
	var bd map[string]any
	_ = json.Unmarshal(bobDetail.Data, &bd)
	bobReply, _ := bd["reply"].(map[string]any)
	if bobReply["content"] != "" || bobReply["status"] != "withdrawn" {
		t.Fatalf("recipient should see withdrawn only: %v", bobReply)
	}
	ownerAfter := do(t, engine, http.MethodGet, "/api/v1/capsules/"+itoa(capsule.ID), alice, nil)
	var oa map[string]any
	_ = json.Unmarshal(ownerAfter.Data, &oa)
	ownerReply, _ := oa["reply"].(map[string]any)
	if ownerReply["content"] != "收到啦" || ownerReply["status"] != "withdrawn" {
		t.Fatalf("owner must still see original reply after withdraw: %v", ownerReply)
	}

	// 13. 撤回后 bob 仍不能再发第二封
	expectCode(t, engine, http.MethodPost, "/api/v1/capsules/"+itoa(capsule.ID)+"/reply", bob,
		map[string]any{"content": "补一封"}, constants.CodeReplyAlreadySent)
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
