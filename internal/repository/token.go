package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sakura-happy-cottage/internal/domain"
)

type TokenRepository struct{ db *gorm.DB }

func NewTokenRepository(db *gorm.DB) *TokenRepository { return &TokenRepository{db: db} }

func (r *TokenRepository) Replace(ctx context.Context, token *domain.AuthToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND purpose = ?", token.UserID, token.Purpose).Delete(&domain.AuthToken{}).Error; err != nil {
			return err
		}
		return tx.Create(token).Error
	})
}

func (r *TokenRepository) DeleteByHash(ctx context.Context, hash []byte) error {
	return r.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&domain.AuthToken{}).Error
}

func (r *TokenRepository) VerifyEmail(ctx context.Context, hash []byte) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token domain.AuthToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", hash, "email_verify", time.Now()).
			First(&token).Error
		if err != nil {
			return err
		}
		if token.PendingEmail == nil {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&domain.User{}).Where("id = ?", token.UserID).
			Updates(map[string]any{"email": *token.PendingEmail, "email_verified": true, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Model(&token).Update("used_at", time.Now()).Error
	})
}

func (r *TokenRepository) ResetPassword(ctx context.Context, hash []byte, passwordHash string) (int64, error) {
	var userID int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token domain.AuthToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", hash, "password_reset", time.Now()).
			First(&token).Error
		if err != nil {
			return err
		}
		userID = token.UserID
		if err := tx.Model(&domain.User{}).Where("id = ?", userID).
			Updates(map[string]any{"password_hash": passwordHash, "must_change_password": false, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Model(&token).Update("used_at", time.Now()).Error
	})
	return userID, err
}
