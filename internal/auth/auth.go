/*
internal/auth/auth.go
模块：用户身份认证与令牌签发
职责：
- 基于 HMAC-SHA256 实现轻量 JWT 令牌签发与校验（支持时效验证）
- 基于 bcrypt 实现密码高安全哈希与密码比对
*/

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Claims JWT 载荷
type Claims struct {
	UID      int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
}

// Manager 令牌管理
type Manager struct {
	secret []byte
}

// New 创建管理器
func New(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

var enc = base64.RawURLEncoding

func (m *Manager) sign(data string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(data))
	return enc.EncodeToString(mac.Sum(nil))
}

// Token 为用户签发 30 天有效令牌
func (m *Manager) Token(uid int64, username, role string) (string, error) {
	if role == "" {
		role = "user"
	}
	header := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(Claims{
		UID: uid, Username: username, Role: role,
		Exp: time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	data := header + "." + enc.EncodeToString(payloadBytes)
	return data + "." + m.sign(data), nil
}

// Parse 校验并解析令牌
func (m *Manager) Parse(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	data := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(m.sign(data)), []byte(parts[2])) {
		return nil, errors.New("invalid signature")
	}
	raw, err := enc.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("malformed payload")
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errors.New("malformed claims")
	}
	if time.Now().Unix() > c.Exp {
		return nil, errors.New("token expired")
	}
	return &c, nil
}

// HashPassword 生成 bcrypt 哈希
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验密码
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
