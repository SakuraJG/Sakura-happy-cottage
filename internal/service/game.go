package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/repository"
)

const (
	DrawingGuessGame = "drawing-guess"
	DouDizhuGame     = "dou-dizhu"
	DaVinciGame      = "da-vinci-code"
)

func validGameKey(gameKey string) bool {
	return gameKey == DrawingGuessGame || gameKey == DouDizhuGame || gameKey == DaVinciGame
}

type GameService struct{ repo *repository.GameRepository }

func NewGameService(repo *repository.GameRepository) *GameService { return &GameService{repo: repo} }

func (s *GameService) Stats(ctx context.Context, userID int64, gameKey string) (domain.GameStats, error) {
	gameKey = strings.TrimSpace(gameKey)
	if gameKey == "" {
		gameKey = DrawingGuessGame
	}
	if !validGameKey(gameKey) {
		return domain.GameStats{}, errors.New("小游戏不存在")
	}
	return s.repo.Stats(ctx, userID, gameKey)
}

func (s *GameService) Record(ctx context.Context, userID int64, match domain.GameMatch) (domain.GameStats, error) {
	match.ID = 0
	match.UserID = userID
	match.CreatedAt = time.Time{}
	match.GameKey = strings.TrimSpace(match.GameKey)
	match.RoomCode = strings.ToUpper(strings.TrimSpace(match.RoomCode))
	match.RoomName = strings.TrimSpace(match.RoomName)
	match.Word = strings.TrimSpace(match.Word)
	if err := validateGameMatch(match); err != nil {
		return domain.GameStats{}, err
	}
	if err := s.repo.CreateMatch(ctx, &match); err != nil {
		return domain.GameStats{}, err
	}
	return s.repo.Stats(ctx, userID, match.GameKey)
}

func (s *GameService) Session(ctx context.Context, userID int64, gameKey, roomCode string) (domain.GameSessionDetail, error) {
	gameKey = strings.TrimSpace(gameKey)
	if gameKey == "" {
		gameKey = DrawingGuessGame
	}
	roomCode = strings.ToUpper(strings.TrimSpace(roomCode))
	if !validGameKey(gameKey) || roomCode == "" || len(roomCode) > 12 {
		return domain.GameSessionDetail{}, errors.New("对局不存在")
	}
	return s.repo.Session(ctx, userID, gameKey, roomCode)
}

func validateGameMatch(match domain.GameMatch) error {
	if !validGameKey(match.GameKey) {
		return errors.New("小游戏不存在")
	}
	validRoles := map[string]bool{"drawer": true, "guesser": true, "landlord": true, "farmer": true, "codebreaker": true}
	if !validRoles[match.Role] {
		return errors.New("玩家身份无效")
	}
	if match.Result != "win" && match.Result != "complete" && match.Result != "lose" {
		return errors.New("对局结果无效")
	}
	if match.DurationSeconds < 1 || match.DurationSeconds > 7200 {
		return errors.New("游玩时长无效")
	}
	if match.Score < 0 || match.Score > 100000 {
		return errors.New("分数无效")
	}
	if match.RoundNumber < 1 || match.RoundNumber > 100 {
		return errors.New("回合编号无效")
	}
	if len(match.RoomCode) > 12 {
		return errors.New("房间邀请码无效")
	}
	if match.RoomName == "" || len([]rune(match.RoomName)) > 24 {
		return errors.New("房间名称无效")
	}
	if match.GameKey == DrawingGuessGame && match.Word == "" {
		return errors.New("题目无效")
	}
	if len([]rune(match.Word)) > 32 {
		return errors.New("对局摘要无效")
	}
	return nil
}
