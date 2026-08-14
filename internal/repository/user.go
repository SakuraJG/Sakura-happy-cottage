package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sakura-happy-cottage/internal/domain"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	return user, err
}

func (r *UserRepository) FindByAccount(ctx context.Context, account string) (domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("LOWER(username) = ?", strings.ToLower(account)).First(&user).Error
	return user, err
}

func (r *UserRepository) FindByLogin(ctx context.Context, account string) (domain.User, error) {
	var user domain.User
	normalized := strings.ToLower(account)
	err := r.db.WithContext(ctx).
		Where("LOWER(username) = ? OR (email_verified = TRUE AND LOWER(email) = ?)", normalized, normalized).
		First(&user).Error
	return user, err
}

func (r *UserRepository) FindVerifiedByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("LOWER(email) = ? AND email_verified = TRUE", strings.ToLower(email)).First(&user).Error
	return user, err
}

func (r *UserRepository) Save(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepository) EmailExistsForOther(ctx context.Context, email string, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("LOWER(email) = ? AND id <> ?", strings.ToLower(email), userID).Count(&count).Error
	return count > 0, err
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id int64, username, bio string) (domain.User, error) {
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).
		Updates(map[string]any{"display_name": username, "bio": bio, "updated_at": time.Now()}).Error
	if err != nil {
		return domain.User{}, err
	}
	return r.FindByID(ctx, id)
}

func (r *UserRepository) UpdateEmail(ctx context.Context, id int64, email string, verified bool) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).
		Updates(map[string]any{"email": email, "email_verified": verified, "updated_at": time.Now()}).Error
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, hash string) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash, "must_change_password": false, "updated_at": time.Now()}).Error
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepository) EnsureBootstrapAdmin(ctx context.Context, account, passwordHash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user domain.User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(username) = ?", strings.ToLower(account)).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if passwordHash == "" {
				return ErrBootstrapPasswordRequired
			}
			user = domain.User{Account: account, Username: account, PasswordHash: passwordHash, Role: "admin", MustChangePassword: true}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if user.Role != "admin" {
			return ErrBootstrapAccountConflict
		}
		return tx.Model(&domain.Memo{}).Where("user_id IS NULL").Update("user_id", user.ID).Error
	})
}

var (
	ErrBootstrapPasswordRequired = errors.New("bootstrap admin password required")
	ErrBootstrapAccountConflict  = errors.New("bootstrap account belongs to a non-admin user")
)
