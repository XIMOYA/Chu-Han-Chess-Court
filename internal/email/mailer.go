/*
internal/email/mailer.go
模块：SMTP 邮件发送与验证码安全管理
职责：
- 基于 Go 原生 net/smtp 与 crypto/tls 实现 SSL/TLS 邮件隧道发信
- 维护注册验证码生成、60秒限流防刷、10分钟时效管理与一次性核销
- 提供典雅古风样式的 HTML 验证码与系统测试邮件模板
*/

package email

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// SMTPConfig 邮件隧道配置
type SMTPConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
	SSL  bool   `json:"ssl"`
}

type codeItem struct {
	code       string
	expiresAt  time.Time
	lastSentAt time.Time
}

// CodeManager 验证码生命周期管理
type CodeManager struct {
	mu    sync.Mutex
	codes map[string]*codeItem
}

// NewCodeManager 创建验证码管理器
func NewCodeManager() *CodeManager {
	cm := &CodeManager{
		codes: make(map[string]*codeItem),
	}
	go cm.cleanupLoop()
	return cm
}

func (cm *CodeManager) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		cm.mu.Lock()
		for k, v := range cm.codes {
			if now.After(v.expiresAt) {
				delete(cm.codes, k)
			}
		}
		cm.mu.Unlock()
	}
}

// Generate 生成 6 位随机验证码并记录限流
func (cm *CodeManager) Generate(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("邮箱地址不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	if item, ok := cm.codes[email]; ok {
		if now.Sub(item.lastSentAt) < 60*time.Second {
			remaining := 60 - int(now.Sub(item.lastSentAt).Seconds())
			return "", fmt.Errorf("发送过于频繁，请 %d 秒后再试", remaining)
		}
	}

	code := fmt.Sprintf("%06d", rand.Intn(900000)+100000)
	cm.codes[email] = &codeItem{
		code:       code,
		expiresAt:  now.Add(10 * time.Minute),
		lastSentAt: now,
	}
	return code, nil
}

// Verify 核销验证码（一次性）
func (cm *CodeManager) Verify(email, code string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return false
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	item, ok := cm.codes[email]
	if !ok {
		return false
	}
	if time.Now().After(item.expiresAt) {
		delete(cm.codes, email)
		return false
	}
	if item.code == code {
		delete(cm.codes, email)
		return true
	}
	return false
}

// SendMail 发送 HTML 邮件
func SendMail(cfg SMTPConfig, to, subject, htmlBody string) error {
	if cfg.Host == "" || cfg.Port == 0 || cfg.User == "" || cfg.Pass == "" {
		return errors.New("SMTP 邮件隧道配置不完整，请联系管理员")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)

	// UTF-8 标题编码
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	fromHeader := fmt.Sprintf("=?UTF-8?B?%s?= <%s>", base64.StdEncoding.EncodeToString([]byte("楚汉棋苑")), cfg.User)

	headers := []string{
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + encodedSubject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	message := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody)

	// 端口 465 或显式指定 SSL
	if cfg.SSL || cfg.Port == 465 {
		tlsConfig := &tls.Config{
			ServerName:         cfg.Host,
			InsecureSkipVerify: false,
		}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("建立 TLS 隧道失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Close()

		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 鉴权失败（请检查密码或授权码）: %w", err)
		}
		if err = client.Mail(cfg.User); err != nil {
			return fmt.Errorf("指定发件人失败: %w", err)
		}
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("指定收件人失败: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("准备写入信体失败: %w", err)
		}
		if _, err = w.Write(message); err != nil {
			return fmt.Errorf("写入邮件内容失败: %w", err)
		}
		return w.Close()
	}

	// 默认端口（如 587/25）常规连接与 STARTTLS 支持
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: cfg.Host}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("启动 STARTTLS 隧道失败: %w", err)
		}
	}

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 鉴权失败（请检查密码或授权码）: %w", err)
	}
	if err = client.Mail(cfg.User); err != nil {
		return fmt.Errorf("指定发件人失败: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("指定收件人失败: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("准备写入信体失败: %w", err)
	}
	if _, err = w.Write(message); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	return w.Close()
}

// BuildCodeEmailHTML 生成典雅古风验证码 HTML 模板
func BuildCodeEmailHTML(code string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:24px;background:#24160a;font-family:'Noto Serif SC',serif;color:#432a12;">
  <div style="max-width:540px;margin:0 auto;background:#f3e4c4;border:2px solid #8a5a28;border-radius:10px;padding:32px;box-shadow:0 10px 30px rgba(0,0,0,0.4);">
    <div style="text-align:center;border-bottom:1px solid #c9a24b;padding-bottom:16px;margin-bottom:20px;">
      <h1 style="margin:0;color:#7a2415;font-size:26px;letter-spacing:4px;">楚 汉 棋 苑</h1>
      <p style="margin:6px 0 0;color:#8a6a3c;font-size:13px;letter-spacing:2px;">楚河汉界 · 在线对弈平台</p>
    </div>
    <p style="font-size:15px;line-height:1.8;color:#5a3c1e;">尊敬的棋友：</p>
    <p style="font-size:15px;line-height:1.8;color:#5a3c1e;">您正在注册楚汉棋苑弈者账号。请使用下方的入苑验证码完成登记验证：</p>
    <div style="text-align:center;margin:28px 0;">
      <span style="display:inline-block;background:#7a2415;color:#f5e4c0;font-size:32px;font-weight:bold;letter-spacing:8px;padding:12px 32px;border-radius:8px;box-shadow:0 4px 12px rgba(122,36,21,0.35);">
        %s
      </span>
    </div>
    <p style="font-size:13px;color:#8a6a3c;line-height:1.6;">注：验证码在 <b>10 分钟</b> 内有效。如非您本人操作，请忽略此函。</p>
    <div style="margin-top:28px;padding-top:16px;border-top:1px dashed #c9a24b;text-align:center;font-size:12px;color:#9a7a4a;">
      对局如对战，落子不悔 · 楚汉棋苑敬启
    </div>
  </div>
</body>
</html>`, code)
}

// BuildTestEmailHTML 生成 SMTP 测试联通邮件模板
func BuildTestEmailHTML(user string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:24px;background:#24160a;font-family:'Noto Serif SC',serif;color:#432a12;">
  <div style="max-width:540px;margin:0 auto;background:#f3e4c4;border:2px solid #8a5a28;border-radius:10px;padding:32px;">
    <h2 style="color:#7a2415;margin-top:0;text-align:center;">楚汉棋苑 · 邮件隧道联通测试</h2>
    <p style="color:#5a3c1e;font-size:15px;line-height:1.8;">恭喜督抚！</p>
    <p style="color:#5a3c1e;font-size:15px;line-height:1.8;">当您看到此信时，代表楚汉棋苑的 <b>SMTP 邮件隧道已成功建立安全握手</b>。发件账号为：<b>%s</b>。</p>
    <p style="color:#5a3c1e;font-size:14px;">棋苑通规中的各项邮件验证服务现已可正常对外投递。</p>
    <p style="color:#8a6a3c;font-size:12px;margin-top:24px;">发送时间：%s</p>
  </div>
</body>
</html>`, user, time.Now().Format("2006-01-02 15:04:05"))
}
