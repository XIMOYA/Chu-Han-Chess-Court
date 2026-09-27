/*
internal/api/server.go
模块：REST API 服务端点
职责：
- 处理用户账号注册、密码加密存储与登录鉴权
- 提供当前用户信息、历史战绩与最近对局记录查询
- 施加请求体大小限制与基础输入合法性校验
*/

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"xiangqi/internal/auth"
	"xiangqi/internal/email"
	"xiangqi/internal/game"
	"xiangqi/internal/hub"
	"xiangqi/internal/llm"
	"xiangqi/internal/puzzle"
	"xiangqi/internal/store"
)

// Server HTTP 接口
type Server struct {
	store *store.Store
	auth  *auth.Manager
	hub   *hub.Hub
	cm    *email.CodeManager
}

// NewServer 创建接口服务
func NewServer(st *store.Store, am *auth.Manager, hb *hub.Hub) *Server {
	return &Server{
		store: st,
		auth:  am,
		hub:   hb,
		cm:    email.NewCodeManager(),
	}
}

var (
	usernameRe = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_]{3,20}$`)
	emailRe    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

type authReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Code     string `json:"code"`
}

type userResp struct {
	ID       int64        `json:"id"`
	Username string       `json:"username"`
	Email    string       `json:"email,omitempty"`
	Role     string       `json:"role"`
	Status   string       `json:"status"`
	Token    string       `json:"token,omitempty"`
	Stats    *store.Stats `json:"stats,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) validate(a authReq, minPwdLen int) (string, bool) {
	u := strings.TrimSpace(a.Username)
	if !usernameRe.MatchString(u) {
		return "用户名需为 3-20 位中文、字母、数字或下划线", false
	}
	if len(a.Password) < minPwdLen || len(a.Password) > 32 {
		return fmt.Sprintf("密码长度需为 %d-32 位", minPwdLen), false
	}
	return u, true
}

// GetPublicConfig 获取免登录公开系统配置（注册准入策略）
func (s *Server) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	allowReg := s.store.GetSetting("allow_registration", "true") == "true"
	reqEmail := s.store.GetSetting("require_email", "false") == "true"
	minPwdLen, _ := strconv.Atoi(s.store.GetSetting("min_password_length", "6"))
	if minPwdLen <= 0 {
		minPwdLen = 6
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"allowRegistration": allowReg,
		"requireEmail":      reqEmail,
		"minPasswordLength": minPwdLen,
	})
}

// SendVerificationCode 发送邮箱注册验证码
func (s *Server) SendVerificationCode(w http.ResponseWriter, r *http.Request) {
	if s.store.GetSetting("allow_registration", "true") == "false" {
		writeErr(w, http.StatusForbidden, "棋苑当前暂未开放注册")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}

	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRe.MatchString(emailAddr) {
		writeErr(w, http.StatusBadRequest, "邮箱格式不正确")
		return
	}

	// 查重：邮箱是否已被注册
	if _, err := s.store.GetUserByEmail(emailAddr); err == nil {
		writeErr(w, http.StatusConflict, "该邮箱已被绑定，请更换或直接登录")
		return
	}

	// 获取 SMTP 配置
	settings := s.store.GetSettings()
	port, _ := strconv.Atoi(settings["smtp_port"])
	if port <= 0 {
		port = 465
	}
	cfg := email.SMTPConfig{
		Host: settings["smtp_host"],
		Port: port,
		User: settings["smtp_user"],
		Pass: settings["smtp_pass"],
		SSL:  settings["smtp_ssl"] == "true",
	}

	if cfg.Host == "" || cfg.User == "" || cfg.Pass == "" {
		writeErr(w, http.StatusInternalServerError, "邮件服务尚未配置完成，请联系督抚")
		return
	}

	code, err := s.cm.Generate(emailAddr)
	if err != nil {
		writeErr(w, http.StatusTooManyRequests, err.Error())
		return
	}

	html := email.BuildCodeEmailHTML(code)
	if err := email.SendMail(cfg, emailAddr, "【楚汉棋苑】弈者入苑验证码", html); err != nil {
		writeErr(w, http.StatusInternalServerError, "邮件发送失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "验证码已成功投递，请查收"})
}

// Register 注册
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	if s.store.GetSetting("allow_registration", "true") == "false" {
		writeErr(w, http.StatusForbidden, "棋苑当前暂未开放新棋友注册")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误或内容过大")
		return
	}

	minPwdLen, _ := strconv.Atoi(s.store.GetSetting("min_password_length", "6"))
	if minPwdLen <= 0 {
		minPwdLen = 6
	}

	u, ok := s.validate(req, minPwdLen)
	if !ok {
		writeErr(w, http.StatusBadRequest, u)
		return
	}

	requireEmail := s.store.GetSetting("require_email", "false") == "true"
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))

	if requireEmail {
		if !emailRe.MatchString(emailAddr) {
			writeErr(w, http.StatusBadRequest, "请输入有效的电子邮箱")
			return
		}
		if !s.cm.Verify(emailAddr, req.Code) {
			writeErr(w, http.StatusBadRequest, "验证码无效或已过期，请重新获取")
			return
		}
		if _, err := s.store.GetUserByEmail(emailAddr); err == nil {
			writeErr(w, http.StatusConflict, "该邮箱已被其他账号绑定")
			return
		}
	}

	if _, err := s.store.GetUserByName(u); err == nil {
		writeErr(w, http.StatusConflict, "用户名已被占用")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	user, err := s.store.CreateUserWithEmail(u, hash, emailAddr)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "注册失败")
		return
	}
	token, _ := s.auth.Token(user.ID, user.Username, user.Role)
	stats, _ := s.store.GetStats(user.ID)
	writeJSON(w, http.StatusOK, userResp{
		ID: user.ID, Username: user.Username, Email: user.Email, Role: user.Role, Status: user.Status, Token: token, Stats: stats,
	})
}

// Login 登录
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误或内容过大")
		return
	}
	u := strings.TrimSpace(req.Username)
	user, err := s.store.GetUserByName(u)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if user.Status == "banned" {
		writeErr(w, http.StatusForbidden, "该账号已被管理员封禁，无法登录")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	token, _ := s.auth.Token(user.ID, user.Username, user.Role)
	stats, _ := s.store.GetStats(user.ID)
	writeJSON(w, http.StatusOK, userResp{
		ID: user.ID, Username: user.Username, Email: user.Email, Role: user.Role, Status: user.Status, Token: token, Stats: stats,
	})
}

func (s *Server) authenticate(r *http.Request) (*auth.Claims, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, false
	}
	claims, err := s.auth.Parse(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return nil, false
	}
	return claims, true
}

func (s *Server) adminAuth(r *http.Request) (*auth.Claims, bool) {
	claims, ok := s.authenticate(r)
	if !ok {
		return nil, false
	}
	if claims.Role == "admin" {
		return claims, true
	}
	// 兼容未刷新 Token 或刚提权场景：穿透查询数据库
	u, err := s.store.GetUserByID(claims.UID)
	if err == nil && u.Role == "admin" {
		claims.Role = "admin"
		return claims, true
	}
	return nil, false
}

// Me 当前用户信息与战绩
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticate(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	user, err := s.store.GetUserByID(claims.UID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	stats, _ := s.store.GetStats(user.ID)
	writeJSON(w, http.StatusOK, userResp{
		ID: user.ID, Username: user.Username, Role: user.Role, Status: user.Status, Stats: stats,
	})
}

// Recent 最近对局
func (s *Server) Recent(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticate(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	games, err := s.store.RecentGames(claims.UID, 20)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	out := make([]*store.GameRecord, 0, len(games))
	out = append(out, games...)
	writeJSON(w, http.StatusOK, map[string]any{"games": out})
}

// GetGameDetail 查询指定已完结对局详情（公开查询，供复盘系统使用）
func (s *Server) GetGameDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的对局ID")
		return
	}
	game, err := s.store.GetGameByID(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "对局记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"game": game})
}

// AdminStats 管理员大盘数据
func (s *Server) AdminStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	dbStats, err := s.store.GetAdminStats()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取统计失败")
		return
	}
	liveRooms, liveConns := s.hub.GetLiveStats()
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":     dbStats,
		"liveRooms": liveRooms,
		"liveConns": liveConns,
	})
}

// AdminRooms 管理员查询活跃房间
func (s *Server) AdminRooms(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	rooms := s.hub.GetActiveRooms()
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

// AdminCloseRoom 管理员强制解散指定房间
func (s *Server) AdminCloseRoom(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeErr(w, http.StatusBadRequest, "房间号不能为空")
		return
	}
	if ok := s.hub.CloseRoom(req.Code); !ok {
		writeErr(w, http.StatusNotFound, "房间不存在或已关闭")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "房间已强制解散"})
}

// AdminUsers 管理员用户列表
func (s *Server) AdminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))

	list, total, err := s.store.GetUsersList(page, limit, query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取用户列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": list,
		"total": total,
	})
}

// AdminUserStatus 修改用户封禁状态
func (s *Server) AdminUserStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Status != "active" && req.Status != "banned") {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.store.SetUserStatus(req.ID, req.Status); err != nil {
		writeErr(w, http.StatusInternalServerError, "修改状态失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// AdminResetPassword 重置用户密码
func (s *Server) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		ID       int64  `json:"id"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "新密码长度需至少 6 位")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "加密密码失败")
		return
	}
	if err := s.store.ResetUserPassword(req.ID, hash); err != nil {
		writeErr(w, http.StatusInternalServerError, "重置密码失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码重置成功"})
}

// AdminGames 历史对局归档查询
func (s *Server) AdminGames(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	gameType := strings.TrimSpace(r.URL.Query().Get("gameType"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))

	list, total, err := s.store.GetAllGames(page, limit, gameType, query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取历史对局失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"games": list,
		"total": total,
	})
}

// GetAdminSettings 获取系统全量配置（含 SMTP 密钥）
func (s *Server) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	settings := s.store.GetSettings()
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

// SaveAdminSettings 保存系统全量配置
func (s *Server) SaveAdminSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := s.store.SetSettings(req.Settings); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "系统通规已成功保存更新"})
}

// TestAdminEmail 测试 SMTP 邮件隧道联通
func (s *Server) TestAdminEmail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.To == "" {
		writeErr(w, http.StatusBadRequest, "接收邮箱地址不能为空")
		return
	}
	if !emailRe.MatchString(req.To) {
		writeErr(w, http.StatusBadRequest, "测试接收邮箱格式不正确")
		return
	}

	settings := s.store.GetSettings()
	port, _ := strconv.Atoi(settings["smtp_port"])
	if port <= 0 {
		port = 465
	}
	cfg := email.SMTPConfig{
		Host: settings["smtp_host"],
		Port: port,
		User: settings["smtp_user"],
		Pass: settings["smtp_pass"],
		SSL:  settings["smtp_ssl"] == "true",
	}

	html := email.BuildTestEmailHTML(cfg.User)
	if err := email.SendMail(cfg, req.To, "【楚汉棋苑】邮件隧道联通测试", html); err != nil {
		writeErr(w, http.StatusBadRequest, "发信测试失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "测试邮件已成功投递！请检查收件箱"})
}

// AdminLLMModels 拉取大模型接口支持的模型标识
func (s *Server) AdminLLMModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	baseURL := req.BaseURL
	apiKey := req.APIKey
	if baseURL == "" {
		baseURL = s.store.GetSetting("llm_base_url", "")
	}
	if apiKey == "" {
		apiKey = s.store.GetSetting("llm_api_key", "")
	}

	models, err := llm.GetModels(baseURL, apiKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// AdminLLMTest 测试大模型联通性与对话响应
func (s *Server) AdminLLMTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
		Model   string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	baseURL := req.BaseURL
	apiKey := req.APIKey
	model := req.Model
	if baseURL == "" {
		baseURL = s.store.GetSetting("llm_base_url", "")
	}
	if apiKey == "" {
		apiKey = s.store.GetSetting("llm_api_key", "")
	}
	if model == "" {
		model = s.store.GetSetting("llm_model", "deepseek-chat")
	}

	reply, latency, pTokens, cTokens, tTokens, err := llm.TestChat(baseURL, apiKey, model)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "大模型测试失败: "+err.Error())
		return
	}

	// 成功调用时记录用量与统计
	_ = s.store.IncrementLLMUsage(pTokens, cTokens, tTokens)

	writeJSON(w, http.StatusOK, map[string]any{
		"reply":            reply,
		"latencyMs":        latency,
		"promptTokens":     pTokens,
		"completionTokens": cTokens,
		"totalTokens":      tTokens,
	})
}

// AdminLLMStats 获取大模型请求与 Token 统计
func (s *Server) AdminLLMStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	stats, err := s.store.GetLLMUsage()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取统计失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

// AdminLLMResetStats 清空大模型用量统计
func (s *Server) AdminLLMResetStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	if err := s.store.ResetLLMUsage(); err != nil {
		writeErr(w, http.StatusInternalServerError, "重置统计失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "统计数据已成功清零"})
}

// GetDailyPuzzle 获取今日每日一题
func (s *Server) GetDailyPuzzle(w http.ResponseWriter, r *http.Request) {
	var uid int64
	if claims, ok := s.authenticate(r); ok {
		uid = claims.UID
	}
	pz, streak, err := s.store.GetDailyPuzzle(uid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "暂无每日一题")
		return
	}
	pz.Solution = "" // 隐藏正解走法防作弊
	writeJSON(w, http.StatusOK, map[string]any{
		"puzzle": pz,
		"streak": streak,
	})
}

// GetPuzzles 获取残局关卡列表
func (s *Server) GetPuzzles(w http.ResponseWriter, r *http.Request) {
	var uid int64
	if claims, ok := s.authenticate(r); ok {
		uid = claims.UID
	}
	list, err := s.store.GetPuzzles(uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取残局列表失败")
		return
	}
	for _, p := range list {
		p.Solution = "" // 隐藏正解走法
	}
	writeJSON(w, http.StatusOK, map[string]any{"puzzles": list})
}

// GetPuzzleDetail 获取单个残局详情
func (s *Server) GetPuzzleDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的残局ID")
		return
	}
	pz, err := s.store.GetPuzzleByID(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "残局不存在")
		return
	}
	pz.Solution = "" // 隐藏正解走法
	writeJSON(w, http.StatusOK, map[string]any{"puzzle": pz})
}

// SubmitPuzzleMove 提交玩家走步进行推演校验
func (s *Server) SubmitPuzzleMove(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的残局ID")
		return
	}
	pz, err := s.store.GetPuzzleByID(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "残局不存在")
		return
	}

	var req struct {
		From      game.Pos `json:"from"`
		To        game.Pos `json:"to"`
		StepIndex int      `json:"stepIndex"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	valid, reply, finished, msg := puzzle.CheckMove(pz.Solution, req.StepIndex, req.From, req.To)

	streak := 0
	if finished {
		var uid int64
		if claims, ok := s.authenticate(r); ok {
			uid = claims.UID
		}
		if uid > 0 {
			_, streak, _ = s.store.RecordPuzzleCompletion(uid, id)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":    valid,
		"reply":    reply,
		"finished": finished,
		"message":  msg,
		"streak":   streak,
	})
}

// AdminPuzzles 后台查询残局列表
func (s *Server) AdminPuzzles(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, total, err := s.store.AdminGetPuzzles(page, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "获取残局列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"puzzles": list,
		"total":   total,
	})
}

// AdminSavePuzzle 后台保存残局
func (s *Server) AdminSavePuzzle(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var pz store.Puzzle
	if err := json.NewDecoder(r.Body).Decode(&pz); err != nil || pz.Title == "" || pz.FEN == "" {
		writeErr(w, http.StatusBadRequest, "标题与初始 FEN 不能为空")
		return
	}
	if err := s.store.AdminSavePuzzle(&pz); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存残局失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "残局已保存更新"})
}

// AdminDeletePuzzle 后台删除残局
func (s *Server) AdminDeletePuzzle(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminAuth(r); !ok {
		writeErr(w, http.StatusForbidden, "无权访问管理员后台")
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		writeErr(w, http.StatusBadRequest, "残局ID无效")
		return
	}
	if err := s.store.AdminDeletePuzzle(req.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "删除残局失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "残局已成功删除"})
}

// GetPublicRooms 获取当前公开候弈中的擂台列表
func (s *Server) GetPublicRooms(w http.ResponseWriter, r *http.Request) {
	rooms := s.hub.GetPublicWaitingRooms()
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

// QuickMatch 极速随缘匹配：智能分配现有擂台或返回未撮合
func (s *Server) QuickMatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GameType string `json:"gameType"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	code, ok := s.hub.FindMatch(req.GameType)
	writeJSON(w, http.StatusOK, map[string]any{
		"matched": ok,
		"code":    code,
	})
}
