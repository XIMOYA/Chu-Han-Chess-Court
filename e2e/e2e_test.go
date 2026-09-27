package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"xiangqi/internal/api"
	"xiangqi/internal/auth"
	"xiangqi/internal/hub"
	"xiangqi/internal/store"
)

func startServer(t *testing.T) (string, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "e2e.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	am := auth.New("e2e-test-secret-key-0123456789abcdef")
	hb := hub.NewHub(st, am)
	srv := api.NewServer(st, am, hb)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", srv.Register)
	mux.HandleFunc("POST /api/login", srv.Login)
	mux.HandleFunc("GET /api/me", srv.Me)
	mux.HandleFunc("GET /api/games/recent", srv.Recent)
	mux.HandleFunc("GET /ws", hb.ServeWS)

	ts := httptest.NewServer(mux)
	return ts.URL, func() {
		ts.Close()
		_ = st.Close()
	}
}

func register(t *testing.T, base, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": name, "password": "secret123"})
	res, err := http.Post(base+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Token
}

func dial(t *testing.T, base, token string) *websocket.Conn {
	t.Helper()
	u, _ := url.Parse(base)
	wsURL := "ws://" + u.Host + "/ws"
	if token != "" {
		wsURL += "?token=" + url.QueryEscape(token)
	}
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func readMsg(t *testing.T, c *websocket.Conn) hub.ServerMsg {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg hub.ServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func writeMsg(t *testing.T, c *websocket.Conn, msg any) {
	t.Helper()
	if err := c.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, c *websocket.Conn, pred func(hub.ServerMsg) bool) hub.ServerMsg {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		msg := readMsg(t, c)
		if pred(msg) {
			return msg
		}
	}
	t.Fatal("condition not met before deadline")
	return hub.ServerMsg{}
}

func TestFullGameFlow(t *testing.T) {
	base, shutdown := startServer(t)
	defer shutdown()

	tokenA := register(t, base, "棋手甲")
	tokenB := register(t, base, "棋手乙")

	// 甲建房（红方，包干 10 分钟）
	cA := dial(t, base, tokenA)
	defer cA.Close()
	writeMsg(t, cA, map[string]any{
		"type": "create_room", "timeMode": "budget",
		"timeSeconds": 600, "side": "red",
	})
	st := waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Code != ""
	})
	code := st.Code
	if st.Status != "waiting" {
		t.Fatalf("status = %s, want waiting", st.Status)
	}

	// 乙加入
	cB := dial(t, base, tokenB)
	defer cB.Close()
	writeMsg(t, cB, map[string]string{"type": "join_room", "code": code})
	playing := waitFor(t, cB, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Status == "playing"
	})
	if playing.You.Role != "black" {
		t.Fatalf("B role = %s, want black", playing.You.Role)
	}
	// 甲也应收到 playing
	waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Status == "playing"
	})

	// 甲：炮二平五 (7,7)->(7,4)
	writeMsg(t, cA, map[string]any{
		"type": "move",
		"from": map[string]int{"r": 7, "c": 7},
		"to":   map[string]int{"r": 7, "c": 4},
	})
	moved := waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && len(m.Moves) == 1
	})
	if moved.Moves[0].Notation != "炮二平五" {
		t.Fatalf("notation = %s, want 炮二平五", moved.Moves[0].Notation)
	}
	waitFor(t, cB, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Side == "black" && len(m.Moves) == 1
	})

	// 乙：马8进7 (0,7)->(2,6)
	writeMsg(t, cB, map[string]any{
		"type": "move",
		"from": map[string]int{"r": 0, "c": 7},
		"to":   map[string]int{"r": 2, "c": 6},
	})
	after := waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && len(m.Moves) == 2
	})
	if after.Moves[1].Notation != "马8进7" {
		t.Fatalf("notation = %s, want 马8进7", after.Moves[1].Notation)
	}

	// 甲申请悔棋，乙同意
	writeMsg(t, cA, map[string]string{"type": "undo_request"})
	req := waitFor(t, cB, func(m hub.ServerMsg) bool {
		return m.Type == "request" && m.Request != nil && m.Request.Kind == "undo"
	})
	if req.Request.From != "棋手甲" {
		t.Fatalf("request from = %s", req.Request.From)
	}
	writeMsg(t, cB, map[string]string{"type": "undo_accept"})
	undone := waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && len(m.Moves) == 0
	})
	if undone.Side != "red" {
		t.Fatal("after undo it should be red's turn")
	}

	// 重新走两步后甲认输
	writeMsg(t, cA, map[string]any{
		"type": "move",
		"from": map[string]int{"r": 7, "c": 7},
		"to":   map[string]int{"r": 7, "c": 4},
	})
	waitFor(t, cB, func(m hub.ServerMsg) bool { return len(m.Moves) == 1 })
	writeMsg(t, cB, map[string]any{
		"type": "move",
		"from": map[string]int{"r": 0, "c": 7},
		"to":   map[string]int{"r": 2, "c": 6},
	})
	waitFor(t, cA, func(m hub.ServerMsg) bool { return len(m.Moves) == 2 })

	writeMsg(t, cA, map[string]string{"type": "resign"})
	fin := waitFor(t, cA, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Status == "finished"
	})
	if fin.Winner != "black" || fin.Reason != "resign" {
		t.Fatalf("winner=%s reason=%s", fin.Winner, fin.Reason)
	}

	// 战绩校验
	me := func(token string) store.Stats {
		req, _ := http.NewRequest("GET", base+"/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out struct {
			Stats store.Stats `json:"stats"`
		}
		json.NewDecoder(res.Body).Decode(&out)
		return out.Stats
	}
	if sA := me(tokenA); sA.Losses != 1 || sA.Total != 1 {
		t.Fatalf("A stats = %+v", sA)
	}
	if sB := me(tokenB); sB.Wins != 1 || sB.Total != 1 {
		t.Fatalf("B stats = %+v", sB)
	}
}

func TestSpectatorCannotMove(t *testing.T) {
	base, shutdown := startServer(t)
	defer shutdown()

	tokenA := register(t, base, "擂主红")
	cA := dial(t, base, tokenA)
	defer cA.Close()
	writeMsg(t, cA, map[string]any{
		"type": "create_room", "timeMode": "budget",
		"timeSeconds": 600, "side": "red",
	})
	st := waitFor(t, cA, func(m hub.ServerMsg) bool { return m.Code != "" })

	// 游客加入观战
	cGuest := dial(t, base, "")
	defer cGuest.Close()
	writeMsg(t, cGuest, map[string]string{"type": "join_room", "code": st.Code})
	joined := waitFor(t, cGuest, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.You != nil
	})
	if joined.You.Role != "spectator" {
		t.Fatalf("guest role = %s, want spectator", joined.You.Role)
	}

	// 游客尝试走棋，应收到 error
	writeMsg(t, cGuest, map[string]any{
		"type": "move",
		"from": map[string]int{"r": 7, "c": 7},
		"to":   map[string]int{"r": 7, "c": 4},
	})
	_ = cGuest.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotError := false
	for {
		_, data, err := cGuest.ReadMessage()
		if err != nil {
			break
		}
		var m hub.ServerMsg
		json.Unmarshal(data, &m)
		if m.Type == "error" {
			gotError = true
			break
		}
	}
	if !gotError {
		t.Fatal("spectator move should be rejected with error")
	}
}

func TestPerMoveTimeout(t *testing.T) {
	base, shutdown := startServer(t)
	defer shutdown()

	tokenA := register(t, base, "慢棋手")
	tokenB := register(t, base, "快棋手")

	cA := dial(t, base, tokenA)
	defer cA.Close()
	writeMsg(t, cA, map[string]any{
		"type": "create_room", "timeMode": "per_move",
		"timeSeconds": 1, "side": "red",
	})
	st := waitFor(t, cA, func(m hub.ServerMsg) bool { return m.Code != "" })

	cB := dial(t, base, tokenB)
	defer cB.Close()
	writeMsg(t, cB, map[string]string{"type": "join_room", "code": st.Code})
	waitFor(t, cB, func(m hub.ServerMsg) bool {
		return m.Status == "playing"
	})

	// 甲（红）不走，等待超时
	fin := waitFor(t, cB, func(m hub.ServerMsg) bool {
		return m.Type == "room_state" && m.Status == "finished"
	})
	if fin.Reason != "timeout" || fin.Winner != "black" {
		t.Fatalf("winner=%s reason=%s, want black/timeout", fin.Winner, fin.Reason)
	}
}
