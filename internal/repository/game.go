package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sakura-happy-cottage/internal/domain"
)

type GameRepository struct{ db *gorm.DB }

func NewGameRepository(db *gorm.DB) *GameRepository { return &GameRepository{db: db} }

func (r *GameRepository) CreateMatch(ctx context.Context, match *domain.GameMatch) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "room_code"}, {Name: "round_number"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{
			clause.Neq{Column: clause.Column{Name: "room_code"}, Value: ""},
		}},
		DoNothing: true,
	}).Create(match).Error
}

func (r *GameRepository) Stats(ctx context.Context, userID int64, gameKey string) (domain.GameStats, error) {
	var aggregate struct {
		TotalGames   int64
		TotalSeconds int64
		Wins         int64
		BestScore    int
	}
	err := r.db.WithContext(ctx).Raw(`
		WITH sessions AS (
			SELECT room_code, SUM(duration_seconds) AS total_seconds,
				SUM(score) AS total_score, COUNT(*) FILTER (WHERE result = 'win') AS wins
			FROM game_matches
			WHERE user_id = ? AND game_key = ?
			GROUP BY room_code
		)
		SELECT COUNT(*) AS total_games,
			COALESCE(SUM(total_seconds), 0) AS total_seconds,
			COUNT(*) FILTER (WHERE wins > 0) AS wins,
			COALESCE(MAX(total_score), 0) AS best_score
		FROM sessions`, userID, gameKey).Scan(&aggregate).Error
	if err != nil {
		return domain.GameStats{}, err
	}
	stats := domain.GameStats{
		TotalGames: aggregate.TotalGames, TotalSeconds: aggregate.TotalSeconds,
		Wins: aggregate.Wins, BestScore: aggregate.BestScore,
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT game_key, room_code, MAX(room_name) AS room_name,
			COUNT(*) AS rounds_played, SUM(duration_seconds) AS total_seconds,
			COUNT(*) FILTER (WHERE result = 'win') AS wins,
			SUM(score) AS total_score, MIN(created_at) AS started_at, MAX(created_at) AS ended_at
		FROM game_matches
		WHERE user_id = ? AND game_key = ?
		GROUP BY game_key, room_code
		ORDER BY MAX(created_at) DESC
		LIMIT 12`, userID, gameKey).Scan(&stats.Recent).Error; err != nil {
		return domain.GameStats{}, err
	}
	if stats.Recent == nil {
		stats.Recent = []domain.GameSessionSummary{}
	}
	return stats, nil
}

func (r *GameRepository) Session(ctx context.Context, userID int64, gameKey, roomCode string) (domain.GameSessionDetail, error) {
	var rounds []domain.GameMatch
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND game_key = ? AND room_code = ?", userID, gameKey, roomCode).
		Order("round_number ASC, created_at ASC").Find(&rounds).Error; err != nil {
		return domain.GameSessionDetail{}, err
	}
	if len(rounds) == 0 {
		return domain.GameSessionDetail{}, gorm.ErrRecordNotFound
	}
	summary := domain.GameSessionSummary{
		GameKey: gameKey, RoomCode: roomCode, RoomName: rounds[0].RoomName,
		StartedAt: rounds[0].CreatedAt, EndedAt: rounds[0].CreatedAt,
	}
	for _, round := range rounds {
		summary.RoundsPlayed++
		summary.TotalSeconds += int64(round.DurationSeconds)
		summary.TotalScore += round.Score
		if round.Result == "win" {
			summary.Wins++
		}
		if round.RoomName != "" {
			summary.RoomName = round.RoomName
		}
		if round.CreatedAt.Before(summary.StartedAt) {
			summary.StartedAt = round.CreatedAt
		}
		if round.CreatedAt.After(summary.EndedAt) {
			summary.EndedAt = round.CreatedAt
		}
	}
	if summary.RoomName == "" {
		summary.RoomName = map[string]string{"drawing-guess": "你画我猜房间", "dou-dizhu": "斗地主房间", "da-vinci-code": "达芬奇密码房间"}[gameKey]
	}
	return domain.GameSessionDetail{Summary: summary, Rounds: rounds}, nil
}
