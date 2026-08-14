package repository

import (
	"context"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/domain"
)

type MemoRepository struct{ db *gorm.DB }

func NewMemoRepository(db *gorm.DB) *MemoRepository { return &MemoRepository{db: db} }

func (r *MemoRepository) List(ctx context.Context, userID int64, status, search string) ([]domain.Memo, error) {
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if status == "open" || status == "done" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("(title ILIKE ? OR description ILIKE ?)", "%"+search+"%", "%"+search+"%")
	}
	var memos []domain.Memo
	err := query.Preload("Attachments").Order("CASE WHEN status = 'open' THEN 0 ELSE 1 END, updated_at DESC").Find(&memos).Error
	return memos, err
}

func (r *MemoRepository) Create(ctx context.Context, memo *domain.Memo) error {
	return r.db.WithContext(ctx).Create(memo).Error
}

func (r *MemoRepository) FindOwned(ctx context.Context, id, userID int64) (domain.Memo, error) {
	var memo domain.Memo
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Preload("Attachments").First(&memo).Error
	return memo, err
}

func (r *MemoRepository) Save(ctx context.Context, memo *domain.Memo) error {
	return r.db.WithContext(ctx).Omit("Attachments").Save(memo).Error
}

func (r *MemoRepository) Delete(ctx context.Context, id, userID int64) (bool, error) {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&domain.Memo{})
	return result.RowsAffected > 0, result.Error
}

func (r *MemoRepository) AddAttachments(ctx context.Context, attachments []domain.Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&attachments).Error
}

func (r *MemoRepository) AttachmentOwned(ctx context.Context, id, userID int64) (domain.Attachment, error) {
	var attachment domain.Attachment
	err := r.db.WithContext(ctx).Table("attachments AS a").
		Select("a.*").Joins("JOIN memos m ON m.id = a.memo_id").
		Where("a.id = ? AND m.user_id = ?", id, userID).Scan(&attachment).Error
	if err == nil && attachment.ID == 0 {
		err = gorm.ErrRecordNotFound
	}
	return attachment, err
}

func (r *MemoRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Memo{}).Where("user_id IS NOT NULL").Count(&count).Error
	return count, err
}
