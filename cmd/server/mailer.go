package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"html"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

func (a *App) sendVerificationEmail(settings SystemSettings, to, token string) error {
	link := authLink(settings.PublicURL, "verify_email", token)
	body := fmt.Sprintf(`<div style="font-family:sans-serif;line-height:1.7;color:#202826"><h2>确认绑定邮箱</h2><p>请点击下面的链接完成邮箱绑定。链接将在 %d 小时后失效。</p><p><a href="%s">确认绑定邮箱</a></p><p style="color:#7b8581">如果不是你本人操作，可以忽略这封邮件。</p></div>`, a.config.Auth.EmailVerifyTTLHours, html.EscapeString(link))
	return sendMail(settings, to, "Sakura的快乐小屋 邮箱确认", body)
}

func (a *App) sendPasswordResetEmail(settings SystemSettings, to, token string) error {
	link := authLink(settings.PublicURL, "reset_password", token)
	body := fmt.Sprintf(`<div style="font-family:sans-serif;line-height:1.7;color:#202826"><h2>重置 Sakura的快乐小屋 密码</h2><p>请点击下面的链接设置新密码。链接将在 %d 分钟后失效。</p><p><a href="%s">设置新密码</a></p><p style="color:#7b8581">如果不是你本人操作，可以忽略这封邮件。</p></div>`, a.config.Auth.PasswordResetTTLMinutes, html.EscapeString(link))
	return sendMail(settings, to, "Sakura的快乐小屋 密码重置", body)
}

func sendMail(settings SystemSettings, to, subject, body string) error {
	address := net.JoinHostPort(settings.SMTPHost, fmt.Sprintf("%d", settings.SMTPPort))
	serverName := settings.SMTPHost
	var connection net.Conn
	var err error
	if settings.SMTPEncryption == "tls" {
		connection, err = tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName})
	} else {
		connection, err = net.DialTimeout("tcp", address, 15*time.Second)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))

	client, err := smtp.NewClient(connection, serverName)
	if err != nil {
		return err
	}
	defer client.Close()
	if settings.SMTPEncryption == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP 服务器不支持 STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}); err != nil {
			return err
		}
	}
	if settings.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, serverName)); err != nil {
			return err
		}
	}
	if err := client.Mail(settings.SMTPFromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	buffer := bufio.NewWriter(writer)
	fromName := strings.TrimSpace(settings.SMTPFromName)
	from := settings.SMTPFromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", fromName), settings.SMTPFromEmail)
	}
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", from, to, mime.QEncoding.Encode("UTF-8", subject), body)
	if _, err := buffer.WriteString(message); err != nil {
		return err
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func authLink(publicURL, parameter, token string) string {
	return fmt.Sprintf("%s/?%s=%s", strings.TrimRight(publicURL, "/"), parameter, token)
}
