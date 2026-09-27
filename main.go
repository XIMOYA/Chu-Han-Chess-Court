/*
main.go
模块：应用程序主入口
职责：
- 解析命令行参数并初始化 SQLite 数据存储与密钥
- 组装核心领域组件（Auth、Hub、Server）并注册 HTTP/WS 路由
- 嵌入前端构建产物（web/dist）并提供 SPA 静态回退服务
*/

package main

import (
	"crypto/rand"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"xiangqi/internal/api"
	"xiangqi/internal/auth"
	"xiangqi/internal/hub"
	"xiangqi/internal/store"
)

//go:embed all:web/dist
var distFS embed.FS

func main() {
	addr := flag.String("addr", ":32555", "HTTP 监听地址")
	dbPath := flag.String("db", "data/xiangqi.db", "SQLite 数据库路径")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()

	secret, err := loadOrCreateSecret(filepath.Join(filepath.Dir(*dbPath), "secret.key"))
	if err != nil {
		log.Fatalf("密钥初始化失败: %v", err)
	}
	am := auth.New(secret)
	hb := hub.NewHub(st, am)
	srv := api.NewServer(st, am, hb)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", srv.GetPublicConfig)
	mux.HandleFunc("POST /api/send-code", srv.SendVerificationCode)
	mux.HandleFunc("POST /api/register", srv.Register)
	mux.HandleFunc("POST /api/login", srv.Login)
	mux.HandleFunc("GET /api/me", srv.Me)
	mux.HandleFunc("GET /api/games/recent", srv.Recent)
	mux.HandleFunc("GET /api/games/{id}", srv.GetGameDetail)
	mux.HandleFunc("GET /ws", hb.ServeWS)

	// 管理员后台专区接口
	mux.HandleFunc("GET /api/admin/stats", srv.AdminStats)
	mux.HandleFunc("GET /api/admin/rooms", srv.AdminRooms)
	mux.HandleFunc("POST /api/admin/rooms/close", srv.AdminCloseRoom)
	mux.HandleFunc("GET /api/admin/users", srv.AdminUsers)
	mux.HandleFunc("POST /api/admin/users/status", srv.AdminUserStatus)
	mux.HandleFunc("POST /api/admin/users/reset-pwd", srv.AdminResetPassword)
	mux.HandleFunc("GET /api/admin/games", srv.AdminGames)
	mux.HandleFunc("GET /api/admin/settings", srv.GetAdminSettings)
	mux.HandleFunc("POST /api/admin/settings", srv.SaveAdminSettings)
	mux.HandleFunc("POST /api/admin/test-email", srv.TestAdminEmail)
	mux.HandleFunc("POST /api/admin/llm/models", srv.AdminLLMModels)
	mux.HandleFunc("POST /api/admin/llm/test", srv.AdminLLMTest)
	mux.HandleFunc("GET /api/admin/llm/stats", srv.AdminLLMStats)
	mux.HandleFunc("POST /api/admin/llm/reset-stats", srv.AdminLLMResetStats)

	// 公开大厅与极速匹配接口
	mux.HandleFunc("GET /api/rooms/public", srv.GetPublicRooms)
	mux.HandleFunc("POST /api/match", srv.QuickMatch)

	// 经典残局与每日一题接口
	mux.HandleFunc("GET /api/puzzles/daily", srv.GetDailyPuzzle)
	mux.HandleFunc("GET /api/puzzles", srv.GetPuzzles)
	mux.HandleFunc("GET /api/puzzles/{id}", srv.GetPuzzleDetail)
	mux.HandleFunc("POST /api/puzzles/{id}/move", srv.SubmitPuzzleMove)
	mux.HandleFunc("GET /api/admin/puzzles", srv.AdminPuzzles)
	mux.HandleFunc("POST /api/admin/puzzles", srv.AdminSavePuzzle)
	mux.HandleFunc("POST /api/admin/puzzles/delete", srv.AdminDeletePuzzle)

	sub, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatalf("嵌入资源错误: %v", err)
	}
	mux.Handle("GET /", spaHandler(sub))

	log.Printf("象棋对弈服务已启动，访问 http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func loadOrCreateSecret(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return string(b), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return "", err
	}
	return string(buf), nil
}

func spaHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(root, p); err != nil {
			// 不存在的路径回退到 index.html（前端路由）
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
