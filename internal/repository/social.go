package repository

import (
	"context"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sakura-happy-cottage/internal/domain"
)

type SocialRepository struct{ db *gorm.DB }

func NewSocialRepository(db *gorm.DB) *SocialRepository { return &SocialRepository{db: db} }

const socialUserSelect = `
SELECT u.id AS uid, u.display_name AS username, u.bio, u.created_at,
       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = ? AND f.followed_id = u.id) AS following,
       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = u.id AND f.followed_id = ?) AS followed_by,
       EXISTS(SELECT 1 FROM follows f1 JOIN follows f2 ON f2.follower_id = f1.followed_id AND f2.followed_id = f1.follower_id WHERE f1.follower_id = ? AND f1.followed_id = u.id) AS friend,
       u.id = ? AS is_self,
       (SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.id) AS following_count,
       (SELECT COUNT(*) FROM follows f WHERE f.followed_id = u.id) AS follower_count,
       (SELECT COUNT(*) FROM follows f1 JOIN follows f2 ON f2.follower_id = f1.followed_id AND f2.followed_id = f1.follower_id WHERE f1.follower_id = u.id) AS friend_count
FROM users u`

func (r *SocialRepository) query(ctx context.Context, sql string, args ...any) ([]domain.SocialUser, error) {
	var users []domain.SocialUser
	err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&users).Error
	return users, err
}

func (r *SocialRepository) Search(ctx context.Context, currentID int64, query string) ([]domain.SocialUser, error) {
	uid, _ := strconv.ParseInt(query, 10, 64)
	sql := socialUserSelect + `
WHERE POSITION(LOWER(?) IN LOWER(u.display_name)) > 0 OR (? > 0 AND u.id = ?)
ORDER BY CASE WHEN u.id = ? THEN 0 ELSE 1 END, u.display_name, u.id LIMIT 20`
	args := []any{currentID, currentID, currentID, currentID, query, uid, uid, uid}
	return r.query(ctx, sql, args...)
}

func (r *SocialRepository) Get(ctx context.Context, currentID, userID int64) (domain.SocialUser, error) {
	users, err := r.query(ctx, socialUserSelect+" WHERE u.id = ?", currentID, currentID, currentID, currentID, userID)
	if err != nil {
		return domain.SocialUser{}, err
	}
	if len(users) == 0 {
		return domain.SocialUser{}, gorm.ErrRecordNotFound
	}
	return users[0], nil
}

func (r *SocialRepository) Follow(ctx context.Context, currentID, userID int64) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.Follow{FollowerID: currentID, FollowedID: userID}).Error
}

func (r *SocialRepository) Unfollow(ctx context.Context, currentID, userID int64) error {
	return r.db.WithContext(ctx).Where("follower_id = ? AND followed_id = ?", currentID, userID).Delete(&domain.Follow{}).Error
}

func (r *SocialRepository) Following(ctx context.Context, currentID int64) ([]domain.SocialUser, error) {
	sql := socialUserSelect + " JOIN follows mine ON mine.followed_id = u.id AND mine.follower_id = ? ORDER BY mine.created_at DESC"
	return r.query(ctx, sql, currentID, currentID, currentID, currentID, currentID)
}

func (r *SocialRepository) Friends(ctx context.Context, currentID int64) ([]domain.SocialUser, error) {
	sql := socialUserSelect + " JOIN follows mine ON mine.followed_id = u.id AND mine.follower_id = ? JOIN follows theirs ON theirs.follower_id = u.id AND theirs.followed_id = ? ORDER BY GREATEST(mine.created_at, theirs.created_at) DESC"
	return r.query(ctx, sql, currentID, currentID, currentID, currentID, currentID, currentID)
}
