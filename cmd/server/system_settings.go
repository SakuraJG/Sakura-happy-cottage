package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SystemSettings struct {
	RegistrationEnabled       bool   `json:"registration_enabled"`
	EmailConfirmationRequired bool   `json:"email_confirmation_required"`
	PasswordRecoveryEnabled   bool   `json:"password_recovery_enabled"`
	PublicURL                 string `json:"public_url"`
	SMTPEnabled               bool   `json:"smtp_enabled"`
	SMTPHost                  string `json:"smtp_host"`
	SMTPPort                  int    `json:"smtp_port"`
	SMTPUsername              string `json:"smtp_username"`
	SMTPPassword              string `json:"-"`
	SMTPPasswordSet           bool   `json:"smtp_password_set"`
	SMTPFromName              string `json:"smtp_from_name"`
	SMTPFromEmail             string `json:"smtp_from_email"`
	SMTPEncryption            string `json:"smtp_encryption"`
}

type systemSettingsInput struct {
	RegistrationEnabled       bool   `json:"registration_enabled"`
	EmailConfirmationRequired bool   `json:"email_confirmation_required"`
	PasswordRecoveryEnabled   bool   `json:"password_recovery_enabled"`
	PublicURL                 string `json:"public_url"`
	SMTPEnabled               bool   `json:"smtp_enabled"`
	SMTPHost                  string `json:"smtp_host"`
	SMTPPort                  int    `json:"smtp_port"`
	SMTPUsername              string `json:"smtp_username"`
	SMTPPassword              string `json:"smtp_password"`
	SMTPFromName              string `json:"smtp_from_name"`
	SMTPFromEmail             string `json:"smtp_from_email"`
	SMTPEncryption            string `json:"smtp_encryption"`
}

func (a *App) ensureSystemSettings(ctx context.Context) error {
	publicURL := fmt.Sprintf("http://127.0.0.1:%d", a.config.Server.Port)
	_, err := a.db.Exec(ctx, `
		INSERT INTO system_settings(singleton, public_url)
		VALUES(TRUE, $1)
		ON CONFLICT (singleton) DO NOTHING`, publicURL)
	return err
}

func (a *App) loadSystemSettings(ctx context.Context) (SystemSettings, error) {
	var settings SystemSettings
	err := a.db.QueryRow(ctx, `
		SELECT registration_enabled, email_confirmation_required, password_recovery_enabled,
		       public_url, smtp_enabled, smtp_host, smtp_port, smtp_username, smtp_password,
		       smtp_from_name, smtp_from_email, smtp_encryption
		FROM system_settings WHERE singleton = TRUE`,
	).Scan(
		&settings.RegistrationEnabled,
		&settings.EmailConfirmationRequired,
		&settings.PasswordRecoveryEnabled,
		&settings.PublicURL,
		&settings.SMTPEnabled,
		&settings.SMTPHost,
		&settings.SMTPPort,
		&settings.SMTPUsername,
		&settings.SMTPPassword,
		&settings.SMTPFromName,
		&settings.SMTPFromEmail,
		&settings.SMTPEncryption,
	)
	settings.SMTPPasswordSet = settings.SMTPPassword != ""
	return settings, err
}

func (a *App) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	var users int64
	var memos int64
	if err := a.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM memos WHERE user_id IS NOT NULL").Scan(&memos); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":          users,
		"memos":          memos,
		"uptime_seconds": int64(time.Since(a.startedAt).Seconds()),
		"settings":       settings,
	})
}

func (a *App) handleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var input systemSettingsInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSystemSettings(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := a.db.Exec(r.Context(), `
		UPDATE system_settings SET
			registration_enabled = $1,
			email_confirmation_required = $2,
			password_recovery_enabled = $3,
			public_url = $4,
			smtp_enabled = $5,
			smtp_host = $6,
			smtp_port = $7,
			smtp_username = $8,
			smtp_password = CASE WHEN $9 = '' THEN smtp_password ELSE $9 END,
			smtp_from_name = $10,
			smtp_from_email = $11,
			smtp_encryption = $12,
			updated_at = NOW()
		WHERE singleton = TRUE`,
		input.RegistrationEnabled,
		input.EmailConfirmationRequired,
		input.PasswordRecoveryEnabled,
		input.PublicURL,
		input.SMTPEnabled,
		input.SMTPHost,
		input.SMTPPort,
		input.SMTPUsername,
		input.SMTPPassword,
		input.SMTPFromName,
		input.SMTPFromEmail,
		input.SMTPEncryption,
	)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "系统设置已保存", "settings": settings})
}

func validateSystemSettings(input *systemSettingsInput) error {
	input.PublicURL = strings.TrimRight(strings.TrimSpace(input.PublicURL), "/")
	parsed, err := url.Parse(input.PublicURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return errors.New("公开访问地址格式无效")
	}
	input.SMTPHost = strings.TrimSpace(input.SMTPHost)
	input.SMTPUsername = strings.TrimSpace(input.SMTPUsername)
	input.SMTPFromName = strings.TrimSpace(input.SMTPFromName)
	input.SMTPFromEmail = strings.TrimSpace(input.SMTPFromEmail)
	input.SMTPEncryption = strings.ToLower(strings.TrimSpace(input.SMTPEncryption))
	if input.SMTPPort < 1 || input.SMTPPort > 65535 {
		return errors.New("SMTP 端口必须在 1 到 65535 之间")
	}
	if input.SMTPEncryption != "none" && input.SMTPEncryption != "starttls" && input.SMTPEncryption != "tls" {
		return errors.New("SMTP 加密方式无效")
	}
	if input.SMTPEnabled {
		if input.SMTPHost == "" || input.SMTPFromEmail == "" {
			return errors.New("启用 SMTP 前需要填写服务器和发件邮箱")
		}
		fromEmail, err := normalizeEmail(input.SMTPFromEmail)
		if err != nil {
			return errors.New("发件邮箱格式无效")
		}
		input.SMTPFromEmail = fromEmail
	}
	if (input.EmailConfirmationRequired || input.PasswordRecoveryEnabled) && !input.SMTPEnabled {
		return errors.New("邮箱确认或找回密码开启时必须同时启用 SMTP")
	}
	return nil
}
