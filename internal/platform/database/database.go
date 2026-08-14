package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sakura-happy-cottage/internal/config"
)

func Open(ctx context.Context, cfg config.Config) (*gorm.DB, error) {
	address := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Database.User, cfg.Database.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Database.Host, cfg.Database.Port),
		Path:   cfg.Database.Name,
	}
	query := address.Query()
	query.Set("sslmode", cfg.Database.SSLMode)
	address.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(address.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	for attempt := 1; attempt <= 30; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = sqlDB.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect to postgres: %w", err)
}

func Migrate(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			display_name TEXT NOT NULL,
			bio TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
			must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
			email TEXT,
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT",
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'",
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE",
		"UPDATE users SET display_name = username WHERE display_name IS NULL OR BTRIM(display_name) = ''",
		"ALTER TABLE users ALTER COLUMN display_name SET NOT NULL",
		`CREATE TABLE IF NOT EXISTS auth_tokens (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash BYTEA NOT NULL UNIQUE,
			purpose TEXT NOT NULL CHECK (purpose IN ('email_verify', 'password_reset')),
			pending_email TEXT,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS follows (
			follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			followed_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (follower_id, followed_id),
			CHECK (follower_id <> followed_id)
		)`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			registration_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			email_confirmation_required BOOLEAN NOT NULL DEFAULT FALSE,
			password_recovery_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			public_url TEXT NOT NULL,
			smtp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			smtp_host TEXT NOT NULL DEFAULT '',
			smtp_port INTEGER NOT NULL DEFAULT 587,
			smtp_username TEXT NOT NULL DEFAULT '',
			smtp_password TEXT NOT NULL DEFAULT '',
			smtp_from_name TEXT NOT NULL DEFAULT 'Sakura的快乐小屋',
			smtp_from_email TEXT NOT NULL DEFAULT '',
			smtp_encryption TEXT NOT NULL DEFAULT 'starttls' CHECK (smtp_encryption IN ('none', 'starttls', 'tls')),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS memos (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		"ALTER TABLE memos ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE",
		`CREATE TABLE IF NOT EXISTS attachments (
			id BIGSERIAL PRIMARY KEY,
			memo_id BIGINT NOT NULL REFERENCES memos(id) ON DELETE CASCADE,
			original_name TEXT NOT NULL,
			stored_name TEXT NOT NULL UNIQUE,
			content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			size BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users(lower(username))",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users(lower(email)) WHERE email IS NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_purpose ON auth_tokens(user_id, purpose)",
		"CREATE INDEX IF NOT EXISTS idx_follows_followed_id ON follows(followed_id)",
		"CREATE INDEX IF NOT EXISTS idx_memos_user_status_updated ON memos(user_id, status, updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_attachments_memo_id ON attachments(memo_id)",
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
