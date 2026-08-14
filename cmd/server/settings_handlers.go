package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sakura-happy-cottage/internal/domain"
)

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

type systemSettingsView struct {
	RegistrationEnabled       bool   `json:"registration_enabled"`
	EmailConfirmationRequired bool   `json:"email_confirmation_required"`
	PasswordRecoveryEnabled   bool   `json:"password_recovery_enabled"`
	PublicURL                 string `json:"public_url"`
	SMTPEnabled               bool   `json:"smtp_enabled"`
	SMTPHost                  string `json:"smtp_host"`
	SMTPPort                  int    `json:"smtp_port"`
	SMTPUsername              string `json:"smtp_username"`
	SMTPPasswordSet           bool   `json:"smtp_password_set"`
	SMTPFromName              string `json:"smtp_from_name"`
	SMTPFromEmail             string `json:"smtp_from_email"`
	SMTPEncryption            string `json:"smtp_encryption"`
}

func settingsView(settings domain.SystemSettings) systemSettingsView {
	return systemSettingsView{
		RegistrationEnabled: settings.RegistrationEnabled, EmailConfirmationRequired: settings.EmailConfirmationRequired,
		PasswordRecoveryEnabled: settings.PasswordRecoveryEnabled, PublicURL: settings.PublicURL, SMTPEnabled: settings.SMTPEnabled,
		SMTPHost: settings.SMTPHost, SMTPPort: settings.SMTPPort, SMTPUsername: settings.SMTPUsername,
		SMTPPasswordSet: settings.SMTPPassword != "", SMTPFromName: settings.SMTPFromName, SMTPFromEmail: settings.SMTPFromEmail,
		SMTPEncryption: settings.SMTPEncryption,
	}
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.settings.Get(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"max_upload_bytes": a.cfg.Storage.MaxUploadBytes, "smtp_enabled": settings.SMTPEnabled, "registration_enabled": settings.RegistrationEnabled, "email_confirmation_required": settings.EmailConfirmationRequired, "password_recovery_enabled": settings.PasswordRecoveryEnabled})
}
func (a *App) handleSystemInfo(w http.ResponseWriter, r *http.Request, id identity) {
	settings, err := a.settings.Get(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	users, err := a.users.Count(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	var memos int64
	if err := a.db.WithContext(r.Context()).Model(&domain.Memo{}).Count(&memos).Error; err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "memos": memos, "uptime_seconds": int64(time.Since(a.started).Seconds()), "settings": settingsView(settings)})
}
func (a *App) handleUpdateSystem(w http.ResponseWriter, r *http.Request, id identity) {
	var input systemSettingsInput
	if err := decodeJSON(r, &input); err != nil {
		handleServiceError(w, err)
		return
	}
	if err := validateSystemSettings(&input); err != nil {
		handleServiceError(w, err)
		return
	}
	settings := domain.SystemSettings{Singleton: true, RegistrationEnabled: input.RegistrationEnabled, EmailConfirmationRequired: input.EmailConfirmationRequired, PasswordRecoveryEnabled: input.PasswordRecoveryEnabled, PublicURL: input.PublicURL, SMTPEnabled: input.SMTPEnabled, SMTPHost: input.SMTPHost, SMTPPort: input.SMTPPort, SMTPUsername: input.SMTPUsername, SMTPPassword: input.SMTPPassword, SMTPFromName: input.SMTPFromName, SMTPFromEmail: input.SMTPFromEmail, SMTPEncryption: input.SMTPEncryption}
	if err := a.settings.Save(r.Context(), &settings, input.SMTPPassword != ""); err != nil {
		writeInternalError(w, err)
		return
	}
	saved, err := a.settings.Get(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "系统设置已保存", "settings": settingsView(saved)})
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
	if input.SMTPEnabled && (input.SMTPHost == "" || input.SMTPFromEmail == "") {
		return errors.New("启用 SMTP 前需要填写服务器和发件邮箱")
	}
	if (input.EmailConfirmationRequired || input.PasswordRecoveryEnabled) && !input.SMTPEnabled {
		return errors.New("邮箱确认或找回密码开启时必须同时启用 SMTP")
	}
	return nil
}
