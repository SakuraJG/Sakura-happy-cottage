package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sakura-happy-cottage/internal/domain"
)

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Ensure(ctx context.Context, publicURL string) error {
	settings := domain.SystemSettings{
		Singleton: true, RegistrationEnabled: true, PublicURL: publicURL,
		SMTPPort: 587, SMTPFromName: "Sakura的快乐小屋", SMTPEncryption: "starttls",
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error
}

func (r *SettingsRepository) Get(ctx context.Context) (domain.SystemSettings, error) {
	var settings domain.SystemSettings
	err := r.db.WithContext(ctx).Where("singleton = TRUE").First(&settings).Error
	return settings, err
}

func (r *SettingsRepository) Save(ctx context.Context, settings *domain.SystemSettings, replacePassword bool) error {
	values := map[string]any{
		"registration_enabled":        settings.RegistrationEnabled,
		"email_confirmation_required": settings.EmailConfirmationRequired,
		"password_recovery_enabled":   settings.PasswordRecoveryEnabled,
		"public_url":                  settings.PublicURL,
		"smtp_enabled":                settings.SMTPEnabled,
		"smtp_host":                   settings.SMTPHost,
		"smtp_port":                   settings.SMTPPort,
		"smtp_username":               settings.SMTPUsername,
		"smtp_from_name":              settings.SMTPFromName,
		"smtp_from_email":             settings.SMTPFromEmail,
		"smtp_encryption":             settings.SMTPEncryption,
	}
	if replacePassword {
		values["smtp_password"] = settings.SMTPPassword
	}
	return r.db.WithContext(ctx).Model(&domain.SystemSettings{}).Where("singleton = TRUE").Updates(values).Error
}
