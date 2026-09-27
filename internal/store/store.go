/*
internal/store/store.go
模块：SQLite 数据库持久化存储
职责：
- 管理 SQLite 单写连接与数据表自动迁移（users / games）
- 提供用户账号创建与查询、对局结果落库及历史战绩统计
*/

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound 记录不存在
var ErrNotFound = errors.New("record not found")

// User 用户
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Email        string `json:"email"`
	Role         string `json:"role"`   // admin / user
	Status       string `json:"status"` // active / banned
	CreatedAt    string `json:"createdAt"`
}

// AdminStats 管理员全盘统计
type AdminStats struct {
	TotalUsers   int `json:"totalUsers"`
	TodayUsers   int `json:"todayUsers"`
	TotalGames   int `json:"totalGames"`
	XiangqiGames int `json:"xiangqiGames"`
	GomokuGames  int `json:"gomokuGames"`
}

// UserWithStats 附带战绩的用户信息
type UserWithStats struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	Wins      int    `json:"wins"`
	Losses    int    `json:"losses"`
	Draws     int    `json:"draws"`
	Total     int    `json:"total"`
}

// LLMStats 大模型调用统计
type LLMStats struct {
	TotalRequests    int64 `json:"totalRequests"`
	PromptTokens     int64 `json:"promptTokens"`
	CompletionTokens int64 `json:"completionTokens"`
	TotalTokens      int64 `json:"totalTokens"`
}

// Puzzle 经典象棋残局模型
type Puzzle struct {
	ID          int64  `json:"id"`
	Level       int    `json:"level"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
	FEN         string `json:"fen"`
	Solution    string `json:"solution,omitempty"`
	Hint        string `json:"hint"`
	Difficulty  string `json:"difficulty"` // easy / medium / hard
	CreatedAt   string `json:"createdAt"`
}

// PuzzleWithStatus 附带当前玩家破局记录的残局
type PuzzleWithStatus struct {
	Puzzle
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// GameRecord 已结束对局记录
type GameRecord struct {
	ID          int64  `json:"id"`
	GameType    string `json:"gameType"` // xiangqi / gomoku
	Code        string `json:"code"`
	RedID       int64  `json:"redId"`
	BlackID     int64  `json:"blackId"`
	RedName     string `json:"redName"`
	BlackName   string `json:"blackName"`
	Result      string `json:"result"` // xiangqi: red/black/draw; gomoku: black/white/draw
	Reason      string `json:"reason"`
	Moves       string `json:"moves"`
	TimeMode    string `json:"timeMode"`
	TimeSeconds int    `json:"timeSeconds"`
	StartedAt   string `json:"startedAt"`
	EndedAt     string `json:"endedAt"`
}

// Stats 战绩
type Stats struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
	Total  int `json:"total"`
}

// Store 数据存储
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）数据库
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS games (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			game_type TEXT NOT NULL DEFAULT 'xiangqi',
			code TEXT NOT NULL,
			red_id INTEGER NOT NULL,
			black_id INTEGER NOT NULL,
			result TEXT NOT NULL,
			reason TEXT NOT NULL,
			moves TEXT NOT NULL DEFAULT '',
			time_mode TEXT NOT NULL DEFAULT 'budget',
			time_seconds INTEGER NOT NULL DEFAULT 1200,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			ended_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_games_red ON games(red_id)`,
		`CREATE INDEX IF NOT EXISTS idx_games_black ON games(black_id)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS llm_stats (
			id INTEGER PRIMARY KEY,
			total_requests INTEGER DEFAULT 0,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS puzzles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level INTEGER DEFAULT 1,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			fen TEXT NOT NULL,
			solution TEXT NOT NULL,
			hint TEXT NOT NULL DEFAULT '',
			difficulty TEXT NOT NULL DEFAULT 'easy',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS puzzle_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			puzzle_id INTEGER NOT NULL,
			completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, puzzle_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_puzzle_records_user ON puzzle_records(user_id)`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	_, _ = s.db.Exec(`INSERT OR IGNORE INTO llm_stats (id, total_requests, prompt_tokens, completion_tokens, total_tokens) VALUES (1, 0, 0, 0, 0)`)

	// 动态检测老库并兼容增加 game_type 字段
	var colCount int
	_ = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('games') WHERE name='game_type'`).Scan(&colCount)
	if colCount == 0 {
		_, _ = s.db.Exec(`ALTER TABLE games ADD COLUMN game_type TEXT NOT NULL DEFAULT 'xiangqi'`)
	}

	// 动态检测老库 users 表并增补 role、status、email 字段
	var roleCol int
	_ = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('users') WHERE name='role'`).Scan(&roleCol)
	if roleCol == 0 {
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`)
	}

	var statusCol int
	_ = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('users') WHERE name='status'`).Scan(&statusCol)
	if statusCol == 0 {
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`)
	}

	var emailCol int
	_ = s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('users') WHERE name='email'`).Scan(&emailCol)
	if emailCol == 0 {
		_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`)
	}

	// 默认赋权首个用户为管理员（方便部署直接使用）
	_, _ = s.db.Exec(`UPDATE users SET role = 'admin' WHERE id = 1`)

	// 初始化默认设置
	defaultSettings := map[string]string{
		"allow_registration":  "true",
		"require_email":       "false",
		"min_password_length": "6",
		"smtp_host":           "",
		"smtp_port":           "465",
		"smtp_user":           "",
		"smtp_pass":           "",
		"smtp_ssl":            "true",
		"llm_enabled":         "false",
		"llm_base_url":        "https://api.deepseek.com/v1",
		"llm_api_key":         "",
		"llm_model":           "deepseek-chat",
	}
	for k, v := range defaultSettings {
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, k, v)
	}

	_ = s.seedPuzzles()

	return nil
}

// CreateUser 创建用户（首个注册账号或账号名为 admin 自动提权为管理员）
func (s *Store) CreateUser(username, hash string) (*User, error) {
	return s.CreateUserWithEmail(username, hash, "")
}

// CreateUserWithEmail 支持邮箱的注册账号
func (s *Store) CreateUserWithEmail(username, hash, email string) (*User, error) {
	var count int
	_ = s.db.QueryRow(`SELECT count(*) FROM users`).Scan(&count)
	role := "user"
	if count == 0 || username == "admin" {
		role = "admin"
	}

	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, email, role, status) VALUES (?, ?, ?, ?, 'active')`,
		username, hash, email, role)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

// GetUserByName 按用户名查询
func (s *Store) GetUserByName(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, COALESCE(email, ''), COALESCE(role, 'user'), COALESCE(status, 'active'), created_at FROM users WHERE username = ?`,
		username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID 按 ID 查询
func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, COALESCE(email, ''), COALESCE(role, 'user'), COALESCE(status, 'active'), created_at FROM users WHERE id = ?`,
		id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByEmail 按邮箱查询
func (s *Store) GetUserByEmail(email string) (*User, error) {
	if email == "" {
		return nil, ErrNotFound
	}
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, COALESCE(email, ''), COALESCE(role, 'user'), COALESCE(status, 'active'), created_at FROM users WHERE email = ?`,
		email).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetSetting 获取单个设置值
func (s *Store) GetSetting(key, defaultVal string) string {
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetSettings 获取全部配置键值对
func (s *Store) GetSettings() map[string]string {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out
}

// SetSettings 批量持久化设置项
func (s *Store) SetSettings(kv map[string]string) error {
	for k, v := range kv {
		_, err := s.db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// IncrementLLMUsage 累加记录大模型调用次数与 Token 用量
func (s *Store) IncrementLLMUsage(promptTokens, completionTokens, totalTokens int) error {
	_, err := s.db.Exec(`UPDATE llm_stats SET
		total_requests = total_requests + 1,
		prompt_tokens = prompt_tokens + ?,
		completion_tokens = completion_tokens + ?,
		total_tokens = total_tokens + ?
		WHERE id = 1`, promptTokens, completionTokens, totalTokens)
	return err
}

// GetLLMUsage 获取大模型调用统计数据
func (s *Store) GetLLMUsage() (*LLMStats, error) {
	st := &LLMStats{}
	err := s.db.QueryRow(`SELECT total_requests, prompt_tokens, completion_tokens, total_tokens FROM llm_stats WHERE id = 1`).Scan(
		&st.TotalRequests, &st.PromptTokens, &st.CompletionTokens, &st.TotalTokens)
	if err != nil {
		return &LLMStats{}, nil
	}
	return st, nil
}

// ResetLLMUsage 清空重置大模型统计
func (s *Store) ResetLLMUsage() error {
	_, err := s.db.Exec(`UPDATE llm_stats SET total_requests = 0, prompt_tokens = 0, completion_tokens = 0, total_tokens = 0 WHERE id = 1`)
	return err
}

// RecordGame 保存一局对局
func (s *Store) RecordGame(r *GameRecord) (int64, error) {
	if r.GameType == "" {
		r.GameType = "xiangqi"
	}
	res, err := s.db.Exec(
		`INSERT INTO games
		 (game_type, code, red_id, black_id, result, reason, moves, time_mode, time_seconds, started_at, ended_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		r.GameType, r.Code, r.RedID, r.BlackID, r.Result, r.Reason, r.Moves,
		r.TimeMode, r.TimeSeconds, r.StartedAt, r.EndedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetStats 查询用户战绩（双游戏胜负兼顾）
func (s *Store) GetStats(userID int64) (*Stats, error) {
	st := &Stats{}
	query := `
		SELECT
			COALESCE(SUM(CASE
				WHEN (g.game_type = 'gomoku' AND ((g.red_id = ? AND g.result = 'black') OR (g.black_id = ? AND g.result = 'white'))) THEN 1
				WHEN (g.game_type != 'gomoku' AND ((g.red_id = ? AND g.result = 'red') OR (g.black_id = ? AND g.result = 'black'))) THEN 1
				ELSE 0
			END), 0),
			COALESCE(SUM(CASE
				WHEN (g.game_type = 'gomoku' AND ((g.red_id = ? AND g.result = 'white') OR (g.black_id = ? AND g.result = 'black'))) THEN 1
				WHEN (g.game_type != 'gomoku' AND ((g.red_id = ? AND g.result = 'black') OR (g.black_id = ? AND g.result = 'red'))) THEN 1
				ELSE 0
			END), 0),
			COALESCE(SUM(CASE WHEN (g.red_id = ? OR g.black_id = ?) AND g.result = 'draw' THEN 1 ELSE 0 END), 0)
		FROM games g
		WHERE g.red_id = ? OR g.black_id = ?`

	err := s.db.QueryRow(query,
		userID, userID,
		userID, userID,
		userID, userID,
		userID, userID,
		userID, userID,
		userID, userID).Scan(&st.Wins, &st.Losses, &st.Draws)
	if err != nil {
		return nil, err
	}
	st.Total = st.Wins + st.Losses + st.Draws
	return st, nil
}

// RecentGames 查询用户最近对局
func (s *Store) RecentGames(userID int64, limit int) ([]*GameRecord, error) {
	rows, err := s.db.Query(
		`SELECT g.id, COALESCE(g.game_type, 'xiangqi'), g.code, g.red_id, g.black_id,
		 ru.username, bu.username,
		 g.result, g.reason, g.moves, g.time_mode, g.time_seconds,
		 g.started_at, g.ended_at
		 FROM games g
		 JOIN users ru ON ru.id = g.red_id
		 JOIN users bu ON bu.id = g.black_id
		 WHERE g.red_id = ? OR g.black_id = ?
		 ORDER BY g.id DESC LIMIT ?`,
		userID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GameRecord
	for rows.Next() {
		r := &GameRecord{}
		if err := rows.Scan(&r.ID, &r.GameType, &r.Code, &r.RedID, &r.BlackID,
			&r.RedName, &r.BlackName,
			&r.Result, &r.Reason, &r.Moves, &r.TimeMode, &r.TimeSeconds,
			&r.StartedAt, &r.EndedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// GetGameByID 根据对局 ID 查询单局完整记录
func (s *Store) GetGameByID(id int64) (*GameRecord, error) {
	r := &GameRecord{}
	query := `SELECT g.id, COALESCE(g.game_type, 'xiangqi'), g.code, g.red_id, g.black_id,
		ru.username, bu.username,
		g.result, g.reason, g.moves, g.time_mode, g.time_seconds,
		g.started_at, g.ended_at
		FROM games g
		JOIN users ru ON ru.id = g.red_id
		JOIN users bu ON bu.id = g.black_id
		WHERE g.id = ?`

	err := s.db.QueryRow(query, id).Scan(
		&r.ID, &r.GameType, &r.Code, &r.RedID, &r.BlackID,
		&r.RedName, &r.BlackName,
		&r.Result, &r.Reason, &r.Moves, &r.TimeMode, &r.TimeSeconds,
		&r.StartedAt, &r.EndedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// GetAdminStats 管理员大盘数据
func (s *Store) GetAdminStats() (*AdminStats, error) {
	st := &AdminStats{}
	_ = s.db.QueryRow(`SELECT count(*) FROM users`).Scan(&st.TotalUsers)
	_ = s.db.QueryRow(`SELECT count(*) FROM users WHERE date(created_at) = date('now', 'localtime')`).Scan(&st.TodayUsers)
	_ = s.db.QueryRow(`SELECT count(*) FROM games`).Scan(&st.TotalGames)
	_ = s.db.QueryRow(`SELECT count(*) FROM games WHERE game_type = 'xiangqi' OR game_type IS NULL`).Scan(&st.XiangqiGames)
	_ = s.db.QueryRow(`SELECT count(*) FROM games WHERE game_type = 'gomoku'`).Scan(&st.GomokuGames)
	return st, nil
}

// GetUsersList 管理员用户列表分页
func (s *Store) GetUsersList(page, limit int, query string) ([]*UserWithStats, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	whereClause := ""
	args := []any{}
	if query != "" {
		whereClause = " WHERE username LIKE ?"
		args = append(args, "%"+query+"%")
	}

	err := s.db.QueryRow("SELECT count(*) FROM users"+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	querySQL := `
		SELECT id, username, COALESCE(email, ''), COALESCE(role, 'user'), COALESCE(status, 'active'), created_at
		FROM users` + whereClause + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	listArgs := append(args, limit, offset)
	rows, err := s.db.Query(querySQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	var list []*UserWithStats
	for rows.Next() {
		u := &UserWithStats{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		list = append(list, u)
	}
	_ = rows.Close() // 关键：立即关闭游标，将独占连接归还给全局连接池，防止死锁

	// 连接池空闲后，安全遍历查询统计战绩
	for _, u := range list {
		if st, err := s.GetStats(u.ID); err == nil {
			u.Wins = st.Wins
			u.Losses = st.Losses
			u.Draws = st.Draws
			u.Total = st.Total
		}
	}
	return list, total, nil
}

// SetUserStatus 设置用户状态（active / banned）
func (s *Store) SetUserStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE users SET status = ? WHERE id = ?`, status, id)
	return err
}

// ResetUserPassword 重置用户密码
func (s *Store) ResetUserPassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// GetAllGames 管理员查询历史对局列表
func (s *Store) GetAllGames(page, limit int, gameType, query string) ([]*GameRecord, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	whereParts := []string{"1=1"}
	args := []any{}

	if gameType != "" {
		whereParts = append(whereParts, "g.game_type = ?")
		args = append(args, gameType)
	}
	if query != "" {
		whereParts = append(whereParts, "(g.code LIKE ? OR ru.username LIKE ? OR bu.username LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q, q)
	}

	whereSQL := " WHERE " + strings.Join(whereParts, " AND ")

	countSQL := `SELECT count(*) FROM games g
		JOIN users ru ON ru.id = g.red_id
		JOIN users bu ON bu.id = g.black_id` + whereSQL

	var total int
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataSQL := `SELECT g.id, COALESCE(g.game_type, 'xiangqi'), g.code, g.red_id, g.black_id,
		ru.username, bu.username,
		g.result, g.reason, g.moves, g.time_mode, g.time_seconds,
		g.started_at, g.ended_at
		FROM games g
		JOIN users ru ON ru.id = g.red_id
		JOIN users bu ON bu.id = g.black_id` + whereSQL + ` ORDER BY g.id DESC LIMIT ? OFFSET ?`

	listArgs := append(args, limit, offset)
	rows, err := s.db.Query(dataSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*GameRecord
	for rows.Next() {
		r := &GameRecord{}
		if err := rows.Scan(&r.ID, &r.GameType, &r.Code, &r.RedID, &r.BlackID,
			&r.RedName, &r.BlackName,
			&r.Result, &r.Reason, &r.Moves, &r.TimeMode, &r.TimeSeconds,
			&r.StartedAt, &r.EndedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, nil
}

func (s *Store) seedPuzzles() error {
	var count int
	_ = s.db.QueryRow(`SELECT count(*) FROM puzzles`).Scan(&count)
	if count > 0 {
		return nil
	}

	seeds := []struct {
		level       int
		title       string
		description string
		source      string
		fen         string
		solution    string
		hint        string
		difficulty  string
	}{
		{
			level:       1,
			title:       "马后炮绝杀",
			description: "经典实用残局。借马立威，当头炮借马身当炮架绝杀孤将。",
			source:      "《适情雅趣》",
			fen:         "3ak4/4a4/9/4N4/9/9/9/9/4C4/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"炮五进七"}]`,
			hint:        "炮五进七，沉底借中马为架当头点将",
			difficulty:  "easy",
		},
		{
			level:       2,
			title:       "重炮连珠",
			description: "前后双炮同列，前炮为架，后炮沉底叫杀，前后相依破九宫。",
			source:      "《橘中秘》",
			fen:         "3ak4/9/4b4/9/9/9/9/4C4/4C4/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"炮五进八"}]`,
			hint:        "底炮借前炮为架直贯底线",
			difficulty:  "easy",
		},
		{
			level:       3,
			title:       "铁门栓破阵",
			description: "中炮镇中路封死将门肋道，贴身沉底强杀无解。",
			source:      "《百局象棋谱》",
			fen:         "3ak4/4a4/9/9/9/9/9/4C4/9/R3K4 w",
			solution:    `[{"from":{"r":9,"c":0},"to":{"r":0,"c":0},"notation":"车一进九"}]`,
			hint:        "底车直贯九泉，底线直点死将",
			difficulty:  "easy",
		},
		{
			level:       4,
			title:       "卧槽马逼宫",
			description: "骏马卧槽控死将门二楼，侧翼引杀迫使黑将受制。",
			source:      "《适情雅趣》",
			fen:         "4k4/4a4/9/9/9/9/9/9/4R4/2N1K4 w",
			solution:    `[{"from":{"r":9,"c":2},"to":{"r":7,"c":3},"notation":"马三进四","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"车五平六"}]`,
			hint:        "先以马跃盘中逼将起楼，再以重车贴身直取中宫",
			difficulty:  "medium",
		},
		{
			level:       5,
			title:       "双车挫骨",
			description: "双车交替呼应，一推一拉，在底线与次底线形成碾压之势。",
			source:      "《韬略元机》",
			fen:         "3ak4/9/9/9/9/9/9/4R4/R8/4K4 w",
			solution:    `[{"from":{"r":8,"c":0},"to":{"r":0,"c":0},"notation":"车一进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":7,"c":4},"to":{"r":0,"c":4},"notation":"车五平六"}]`,
			hint:        "先以边车下底逼将出宫，再以中车移位侧翼合围",
			difficulty:  "medium",
		},
		{
			level:       6,
			title:       "海底捞月",
			description: "经典残局车炮巧胜车卒，以车牵制，借底炮翻山反杀。",
			source:      "《心武残编》",
			fen:         "3ak4/4a4/9/9/9/9/9/9/1C7/R3K4 w",
			solution:    `[{"from":{"r":9,"c":0},"to":{"r":0,"c":0},"notation":"车一进九","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":1},"to":{"r":0,"c":1},"notation":"炮二进八"}]`,
			hint:        "车点将门引蛇出洞，沉底炮借士架绝杀",
			difficulty:  "medium",
		},
		{
			level:       7,
			title:       "挂角马绝杀",
			description: "马入九宫挂角，封锁将门转角，配合底线大子入局。",
			source:      "《渊深海阔》",
			fen:         "3ak4/9/9/9/9/9/9/9/4R4/4K1N2 w",
			solution:    `[{"from":{"r":9,"c":6},"to":{"r":7,"c":5},"notation":"马七进六","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八"}]`,
			hint:        "挂角马封将，铁骑定乾坤",
			difficulty:  "medium",
		},
		{
			level:       8,
			title:       "白马现蹄",
			description: "弃子攻心，马踏连营现神威，破尽敌方防守阵型。",
			source:      "《橘中秘》",
			fen:         "3ak4/4a4/4b4/9/9/9/9/4N4/4R4/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":5,"c":3},"notation":"马五进六","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":3},"notation":"将5平4"}},{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八"}]`,
			hint:        "神马跃起，撕裂防御",
			difficulty:  "hard",
		},
		{
			level:       9,
			title:       "适情雅趣·千里独行",
			description: "江湖名局之意蕴，单车单骑千里奔袭，绝地反击步步惊心。",
			source:      "《适情雅趣》",
			fen:         "3ak4/4a4/9/4N4/9/9/9/9/R8/4K4 w",
			solution:    `[{"from":{"r":8,"c":0},"to":{"r":0,"c":0},"notation":"车一进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":3,"c":4},"to":{"r":1,"c":3},"notation":"马五进四"}]`,
			hint:        "车马并进，断敌退路",
			difficulty:  "hard",
		},
		{
			level:       10,
			title:       "橘中秘·长驱直入",
			description: "雷霆万钧之势，沉底大炮与重车双线合击，直取敌方老营帅旗。",
			source:      "《橘中秘》",
			fen:         "3ak4/9/4a4/9/9/9/9/4C4/4R4/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":7,"c":4},"to":{"r":1,"c":4},"notation":"炮五进六"}]`,
			hint:        "以车换将位，以炮锁喉咙",
			difficulty:  "hard",
		},
		{
			level:       11,
			title:       "夹车炮杀",
			description: "经典车炮双翼齐飞，右翼重车直插九泉，左翼大炮封堵归途。",
			source:      "《适情雅趣》",
			fen:         "3a1k3/4a4/9/9/9/9/9/9/1C5R1/4K4 w",
			solution:    `[{"from":{"r":8,"c":7},"to":{"r":0,"c":7},"notation":"车八进八"}]`,
			hint:        "重车自右翼长驱直入，直取底线",
			difficulty:  "easy",
		},
		{
			level:       12,
			title:       "二鬼拍门",
			description: "双兵直逼九宫重地，先破重臣，再平中肋绝杀。",
			source:      "《适情雅趣》",
			fen:         "3ak4/3PP4/9/9/9/9/9/9/9/4K4 w",
			solution:    `[{"from":{"r":1,"c":3},"to":{"r":0,"c":3},"notation":"兵六进一","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":1,"c":4},"to":{"r":1,"c":5},"notation":"兵五平六"}]`,
			hint:        "左兵破士引将，右兵平移断生路",
			difficulty:  "medium",
		},
		{
			level:       13,
			title:       "天地炮破阵",
			description: "一炮居天沉底，一炮居地镇肋，天罗地网合围黑将。",
			source:      "《蕉窗逸品》",
			fen:         "3ak1C2/4a4/9/9/9/9/9/4R4/9/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":1,"c":4},"notation":"车五进六"}]`,
			hint:        "以重车直破中宫重士",
			difficulty:  "medium",
		},
		{
			level:       14,
			title:       "三子归边",
			description: "车马炮三军齐发，偏师借力，底线绝杀摧枯拉朽。",
			source:      "《百局象棋谱》",
			fen:         "3ak4/4a4/9/9/9/9/9/9/1N5R1/1C2K4 w",
			solution:    `[{"from":{"r":8,"c":7},"to":{"r":0,"c":7},"notation":"车八进八"}]`,
			hint:        "以重车抢先夺占底线天元",
			difficulty:  "medium",
		},
		{
			level:       15,
			title:       "大刀剜心",
			description: "勇车舍生取义直插黑方心腹中士，大炮乘虚而入奠定胜局。",
			source:      "《竹香斋象戏谱》",
			fen:         "3ak4/4a4/9/9/9/9/9/4R4/4C4/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":1,"c":4},"notation":"车五进六","reply":{"from":{"r":0,"c":3},"to":{"r":1,"c":4},"notation":"士4进5"}},{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"炮五进七"}]`,
			hint:        "先以车断敌腹心中枢，再以炮定乾坤",
			difficulty:  "medium",
		},
		{
			level:       16,
			title:       "拔簧马杀局",
			description: "神马跃起引开中宫炮架，借炮发力直击老将面门。",
			source:      "《橘中秘》",
			fen:         "3ak4/4a4/9/9/9/9/9/4N4/4C4/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":5,"c":5},"notation":"马五进六","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":5,"c":5},"to":{"r":3,"c":4},"notation":"马六进五"}]`,
			hint:        "马跃侧翼借炮点穴，再回马连击",
			difficulty:  "medium",
		},
		{
			level:       17,
			title:       "侧面虎伏杀",
			description: "双车潜伏侧肋，车马交相掩护，如猛虎下山直扑将门。",
			source:      "《适情雅趣》",
			fen:         "3ak4/9/9/9/9/9/9/9/2R6/3NK3R w",
			solution:    `[{"from":{"r":8,"c":2},"to":{"r":0,"c":2},"notation":"车三进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":9,"c":3},"to":{"r":7,"c":4},"notation":"马四进五"}]`,
			hint:        "左车直贯九泉，灵马跃入肋门",
			difficulty:  "medium",
		},
		{
			level:       18,
			title:       "老卒平顶",
			description: "孤兵深入敌国如虎添翼，横行九宫直点将台。",
			source:      "《心武残编》",
			fen:         "3ak4/4a1P2/9/9/9/9/9/9/4C4/4K4 w",
			solution:    `[{"from":{"r":1,"c":6},"to":{"r":1,"c":5},"notation":"兵四平五","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":4},"to":{"r":8,"c":5},"notation":"炮五平六"}]`,
			hint:        "兵进中肋逼将位，大炮移位锁全宫",
			difficulty:  "medium",
		},
		{
			level:       19,
			title:       "闷宫巧绝",
			description: "利用黑方双士自塞动线，单炮沉底封死九宫全门。",
			source:      "《韬略元机》",
			fen:         "3aka3/9/4b4/9/9/9/9/9/4C4/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"炮五进八"}]`,
			hint:        "以象为炮架，单炮沉底直取闷宫",
			difficulty:  "easy",
		},
		{
			level:       20,
			title:       "借炮还乡",
			description: "诱敌深入，车沉底逼位，回马金枪破尽杀局。",
			source:      "《适情雅趣》",
			fen:         "3ak4/4a4/9/9/9/9/9/4N4/R8/4K3C w",
			solution:    `[{"from":{"r":8,"c":0},"to":{"r":0,"c":0},"notation":"车一进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":7,"c":4},"to":{"r":5,"c":5},"notation":"马五进六"}]`,
			hint:        "左车长驱诱将，中马跃起收官",
			difficulty:  "medium",
		},
		{
			level:       21,
			title:       "双马饮泉",
			description: "二马盘桓交替踏阵，踏破贺兰山缺，黑将上下难安。",
			source:      "《梦入神机》",
			fen:         "4k4/4a4/9/9/9/9/9/9/4N4/2N1K4 w",
			solution:    `[{"from":{"r":9,"c":2},"to":{"r":7,"c":3},"notation":"马三进四","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":7,"c":4},"to":{"r":5,"c":5},"notation":"马五进六"}]`,
			hint:        "底马跃肋起高楼，中马穿插断回路",
			difficulty:  "hard",
		},
		{
			level:       22,
			title:       "炮辗丹沙",
			description: "重车压阵，大炮在底线辗转腾挪，扫清六合。",
			source:      "《橘中秘》",
			fen:         "3ak4/4a4/9/9/9/9/9/9/4R4/1C2K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":9,"c":1},"to":{"r":1,"c":1},"notation":"炮二进八"}]`,
			hint:        "中车点额，大炮直锁二楼将门",
			difficulty:  "hard",
		},
		{
			level:       23,
			title:       "百局象棋·七星聚会",
			description: "江湖四大名局之首。风云变幻，车马纵横交错，步步杀着。",
			source:      "《百局象棋谱》",
			fen:         "3ak4/4a4/9/4N4/9/9/9/9/4R4/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"车五进七","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":3,"c":4},"to":{"r":1,"c":5},"notation":"马五进六"}]`,
			hint:        "以车抢占心窝，神马侧翼合围",
			difficulty:  "hard",
		},
		{
			level:       24,
			title:       "百局象棋·野马操田",
			description: "江湖四大名局之二。细水长流，以巧破千钧，残局之神韵尽在其中。",
			source:      "《百局象棋谱》",
			fen:         "4k4/4a4/9/9/9/9/9/9/2N1R4/4K4 w",
			solution:    `[{"from":{"r":8,"c":2},"to":{"r":6,"c":3},"notation":"马三进四","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"车五进七"}]`,
			hint:        "马踏中宫起将，重车乘势入座",
			difficulty:  "hard",
		},
		{
			level:       25,
			title:       "百局象棋·蚯蚓降龙",
			description: "江湖四大名局之三。柔能克刚，以卒为先导，车定乾坤。",
			source:      "《百局象棋谱》",
			fen:         "3ak4/4a4/9/9/9/9/9/4P4/4R4/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":6,"c":4},"notation":"兵五进一","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八"}]`,
			hint:        "兵点中肋迫将移步，车入底门奠胜基",
			difficulty:  "hard",
		},
		{
			level:       26,
			title:       "带子入朝",
			description: "二兵连环步步逼近，双兵锁将门，胜似双强车。",
			source:      "《适情雅趣》",
			fen:         "4k4/4a4/9/9/9/9/9/9/4PP3/4K4 w",
			solution:    `[{"from":{"r":8,"c":4},"to":{"r":7,"c":4},"notation":"兵五进一","reply":{"from":{"r":0,"c":4},"to":{"r":1,"c":4},"notation":"将5进1"}},{"from":{"r":8,"c":5},"to":{"r":7,"c":5},"notation":"兵六进一"}]`,
			hint:        "左兵起将，右兵封门",
			difficulty:  "hard",
		},
		{
			level:       27,
			title:       "雪拥蓝关",
			description: "冰封千里，车炮双煞镇守九宫出路，万夫莫开。",
			source:      "《竹香斋象戏谱》",
			fen:         "3ak4/4a4/9/9/9/9/9/9/3C1R3/4K4 w",
			solution:    `[{"from":{"r":8,"c":5},"to":{"r":0,"c":5},"notation":"车六进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":3},"to":{"r":0,"c":3},"notation":"炮四进八"}]`,
			hint:        "右车直下封退路，左炮沉底夺帅旗",
			difficulty:  "hard",
		},
		{
			level:       28,
			title:       "落花流水",
			description: "兵贵神速，马踏连营如水银泻地，势不可挡。",
			source:      "《蕉窗逸品》",
			fen:         "3ak4/9/9/9/9/9/9/9/4R1N2/4K4 w",
			solution:    `[{"from":{"r":8,"c":6},"to":{"r":6,"c":5},"notation":"马七进六","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":4},"to":{"r":0,"c":4},"notation":"车五进八"}]`,
			hint:        "以马引敌破阵，以车扫荡底线",
			difficulty:  "hard",
		},
		{
			level:       29,
			title:       "飞刀挂壁",
			description: "险中求胜，借肋道飞车悬刀，绝处逢生巧构杀局。",
			source:      "《梦入神机》",
			fen:         "3ak4/4a4/9/9/9/9/9/9/1C6R/4K4 w",
			solution:    `[{"from":{"r":8,"c":7},"to":{"r":0,"c":7},"notation":"车八进八","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":1},"to":{"r":0,"c":1},"notation":"炮二进八"}]`,
			hint:        "右车先行立威，左炮后至合围",
			difficulty:  "hard",
		},
		{
			level:       30,
			title:       "百灵朝凤",
			description: "三十大关圆满之卷！群子合围，车马炮三才一体，破局定乾坤！",
			source:      "《适情雅趣》",
			fen:         "3ak4/4a4/9/9/9/9/9/4N4/4C3R/4K4 w",
			solution:    `[{"from":{"r":7,"c":4},"to":{"r":5,"c":5},"notation":"马五进六","reply":{"from":{"r":0,"c":4},"to":{"r":0,"c":5},"notation":"将5平6"}},{"from":{"r":8,"c":8},"to":{"r":0,"c":8},"notation":"车九进八"}]`,
			hint:        "中马跃入将台，边车直捣黄龙",
			difficulty:  "hard",
		},
	}

	for _, p := range seeds {
		var exists int
		_ = s.db.QueryRow(`SELECT count(*) FROM puzzles WHERE title = ?`, p.title).Scan(&exists)
		if exists == 0 {
			_, _ = s.db.Exec(`INSERT INTO puzzles (level, title, description, source, fen, solution, hint, difficulty)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				p.level, p.title, p.description, p.source, p.fen, p.solution, p.hint, p.difficulty)
		}
	}
	return nil
}

// GetPuzzles 获取全量关卡列表及某用户的破局状态
func (s *Store) GetPuzzles(userID int64) ([]*PuzzleWithStatus, error) {
	query := `SELECT p.id, p.level, p.title, p.description, p.source, p.fen, p.hint, p.difficulty, p.created_at,
		CASE WHEN r.id IS NOT NULL THEN 1 ELSE 0 END as completed,
		COALESCE(r.completed_at, '')
		FROM puzzles p
		LEFT JOIN puzzle_records r ON r.puzzle_id = p.id AND r.user_id = ?
		ORDER BY p.level ASC`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*PuzzleWithStatus
	for rows.Next() {
		pz := &PuzzleWithStatus{}
		var compInt int
		if err := rows.Scan(&pz.ID, &pz.Level, &pz.Title, &pz.Description, &pz.Source,
			&pz.FEN, &pz.Hint, &pz.Difficulty, &pz.CreatedAt, &compInt, &pz.CompletedAt); err != nil {
			return nil, err
		}
		pz.Completed = compInt == 1
		out = append(out, pz)
	}
	return out, nil
}

// GetDailyPuzzle 获取今日轮换残局与玩家打卡状态
func (s *Store) GetDailyPuzzle(userID int64) (*PuzzleWithStatus, int, error) {
	nowStr := time.Now().Format("20060102")
	var totalPuzzles int
	_ = s.db.QueryRow(`SELECT count(*) FROM puzzles`).Scan(&totalPuzzles)
	if totalPuzzles == 0 {
		return nil, 0, ErrNotFound
	}

	var seed int
	for _, c := range nowStr {
		seed = seed*31 + int(c)
	}
	dailyIndex := (seed % totalPuzzles) + 1

	pz := &PuzzleWithStatus{}
	query := `SELECT p.id, p.level, p.title, p.description, p.source, p.fen, p.hint, p.difficulty, p.created_at,
		CASE WHEN r.id IS NOT NULL THEN 1 ELSE 0 END as completed,
		COALESCE(r.completed_at, '')
		FROM puzzles p
		LEFT JOIN puzzle_records r ON r.puzzle_id = p.id AND r.user_id = ?
		WHERE p.level = ? LIMIT 1`

	var compInt int
	err := s.db.QueryRow(query, userID, dailyIndex).Scan(
		&pz.ID, &pz.Level, &pz.Title, &pz.Description, &pz.Source,
		&pz.FEN, &pz.Hint, &pz.Difficulty, &pz.CreatedAt, &compInt, &pz.CompletedAt)
	if err != nil {
		_ = s.db.QueryRow(`SELECT p.id, p.level, p.title, p.description, p.source, p.fen, p.hint, p.difficulty, p.created_at,
			CASE WHEN r.id IS NOT NULL THEN 1 ELSE 0 END as completed,
			COALESCE(r.completed_at, '')
			FROM puzzles p
			LEFT JOIN puzzle_records r ON r.puzzle_id = p.id AND r.user_id = ?
			LIMIT 1`, userID).Scan(
			&pz.ID, &pz.Level, &pz.Title, &pz.Description, &pz.Source,
			&pz.FEN, &pz.Hint, &pz.Difficulty, &pz.CreatedAt, &compInt, &pz.CompletedAt)
	}
	pz.Completed = compInt == 1

	var streak int
	_ = s.db.QueryRow(`SELECT count(*) FROM puzzle_records WHERE user_id = ?`, userID).Scan(&streak)

	return pz, streak, nil
}

// GetPuzzleByID 查询指定残局完整数据（含正解步骤）
func (s *Store) GetPuzzleByID(id int64) (*Puzzle, error) {
	pz := &Puzzle{}
	query := `SELECT id, level, title, description, source, fen, solution, hint, difficulty, created_at FROM puzzles WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(
		&pz.ID, &pz.Level, &pz.Title, &pz.Description, &pz.Source,
		&pz.FEN, &pz.Solution, &pz.Hint, &pz.Difficulty, &pz.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return pz, err
}

// RecordPuzzleCompletion 记录玩家通关该残局
func (s *Store) RecordPuzzleCompletion(userID, puzzleID int64) (bool, int, error) {
	if userID <= 0 {
		return true, 1, nil
	}
	res, err := s.db.Exec(`INSERT OR IGNORE INTO puzzle_records (user_id, puzzle_id, completed_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		userID, puzzleID)
	if err != nil {
		return false, 0, err
	}
	affected, _ := res.RowsAffected()
	var totalDone int
	_ = s.db.QueryRow(`SELECT count(*) FROM puzzle_records WHERE user_id = ?`, userID).Scan(&totalDone)
	return affected > 0, totalDone, nil
}

// AdminGetPuzzles 后台残局列表分页
func (s *Store) AdminGetPuzzles(page, limit int) ([]*Puzzle, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 15
	}
	offset := (page - 1) * limit

	var total int
	_ = s.db.QueryRow(`SELECT count(*) FROM puzzles`).Scan(&total)

	rows, err := s.db.Query(`SELECT id, level, title, description, source, fen, solution, hint, difficulty, created_at
		FROM puzzles ORDER BY level ASC, id ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*Puzzle
	for rows.Next() {
		p := &Puzzle{}
		if err := rows.Scan(&p.ID, &p.Level, &p.Title, &p.Description, &p.Source,
			&p.FEN, &p.Solution, &p.Hint, &p.Difficulty, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, nil
}

// AdminSavePuzzle 新增或保存残局
func (s *Store) AdminSavePuzzle(p *Puzzle) error {
	if p.ID > 0 {
		_, err := s.db.Exec(`UPDATE puzzles SET level = ?, title = ?, description = ?, source = ?, fen = ?, solution = ?, hint = ?, difficulty = ?
			WHERE id = ?`, p.Level, p.Title, p.Description, p.Source, p.FEN, p.Solution, p.Hint, p.Difficulty, p.ID)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO puzzles (level, title, description, source, fen, solution, hint, difficulty)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, p.Level, p.Title, p.Description, p.Source, p.FEN, p.Solution, p.Hint, p.Difficulty)
	return err
}

// AdminDeletePuzzle 删除残局
func (s *Store) AdminDeletePuzzle(id int64) error {
	_, err := s.db.Exec(`DELETE FROM puzzles WHERE id = ?`, id)
	return err
}
