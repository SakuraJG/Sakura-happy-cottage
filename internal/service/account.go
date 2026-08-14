package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/platform/mailer"
)

func (s *AuthService) BindEmail(ctx context.Context, rateKey string, userID int64, email, password string, sender mailer.Sender) (bool, error) {
	if err := s.allow(ctx, "bind-email:"+rateKey, 5, 15*time.Minute); err != nil {
		return false, err
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return false, err
	}
	if settings.EmailConfirmationRequired && !settings.SMTPEnabled {
		return false, ErrMailUnavailable
	}
	email, err = NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return false, ErrInvalidCredentials
	}
	exists, err := s.users.EmailExistsForOther(ctx, email, userID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, ErrEmailInUse
	}
	if !settings.EmailConfirmationRequired {
		return true, s.users.UpdateEmail(ctx, userID, email, true)
	}
	raw, err := s.createToken(ctx, userID, "email_verify", email, time.Duration(s.cfg.Auth.EmailVerifyTTLHours)*time.Hour)
	if err != nil {
		return false, err
	}
	link := fmt.Sprintf("%s/?verify_email=%s", strings.TrimRight(settings.PublicURL, "/"), raw)
	body := fmt.Sprintf(`<div style="font-family:sans-serif;line-height:1.7;color:#202826"><h2>确认绑定邮箱</h2><p>请点击链接完成邮箱绑定，该链接将在 %d 小时后失效。</p><p><a href="%s">确认绑定邮箱</a></p></div>`, s.cfg.Auth.EmailVerifyTTLHours, html.EscapeString(link))
	if err := sender.Send(settings, email, "Sakura的快乐小屋 邮箱确认", body); err != nil {
		_ = s.tokens.DeleteByHash(ctx, TokenHash(raw))
		return false, err
	}
	return false, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return ErrInvalidToken
	}
	if err := s.tokens.VerifyEmail(ctx, TokenHash(token)); errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInvalidToken
	} else {
		return err
	}
}

func (s *AuthService) ForgotPassword(ctx context.Context, rateKey, email string, sender mailer.Sender) error {
	if err := s.allow(ctx, "forgot-password:"+rateKey, 5, 15*time.Minute); err != nil {
		return err
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	if !settings.PasswordRecoveryEnabled || !settings.SMTPEnabled {
		return ErrMailUnavailable
	}
	email, err = NormalizeEmail(email)
	if err != nil {
		return err
	}
	user, err := s.users.FindVerifiedByEmail(ctx, email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	raw, err := s.createToken(ctx, user.ID, "password_reset", "", time.Duration(s.cfg.Auth.PasswordResetTTLMinutes)*time.Minute)
	if err != nil {
		return err
	}
	link := fmt.Sprintf("%s/?reset_password=%s", strings.TrimRight(settings.PublicURL, "/"), raw)
	body := fmt.Sprintf(`<div style="font-family:sans-serif;line-height:1.7;color:#202826"><h2>重置密码</h2><p>请点击链接设置新密码，该链接将在 %d 分钟后失效。</p><p><a href="%s">设置新密码</a></p></div>`, s.cfg.Auth.PasswordResetTTLMinutes, html.EscapeString(link))
	if err := sender.Send(settings, email, "Sakura的快乐小屋 密码重置", body); err != nil {
		log.Printf("send password reset email: %v", err)
	}
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, password string) error {
	if token == "" {
		return ErrInvalidToken
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	userID, err := s.tokens.ResetPassword(ctx, TokenHash(token), string(hash))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	return s.redis.DeleteUserSessions(ctx, userID)
}

func (s *AuthService) createToken(ctx context.Context, userID int64, purpose, pendingEmail string, ttl time.Duration) (string, error) {
	raw, err := RandomToken()
	if err != nil {
		return "", err
	}
	var email *string
	if pendingEmail != "" {
		email = &pendingEmail
	}
	token := domain.AuthToken{UserID: userID, TokenHash: TokenHash(raw), Purpose: purpose, PendingEmail: email, ExpiresAt: time.Now().Add(ttl)}
	return raw, s.tokens.Replace(ctx, &token)
}

var (
	ErrMailUnavailable = errors.New("邮件服务尚未配置")
	ErrEmailInUse      = errors.New("该邮箱已被其他账号使用")
	ErrInvalidToken    = errors.New("链接无效或已过期")
)
