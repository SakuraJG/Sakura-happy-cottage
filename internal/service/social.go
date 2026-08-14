package service

import (
	"context"
	"errors"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/repository"
	"strings"
	"unicode/utf8"
)

type SocialService struct{ repo *repository.SocialRepository }

func NewSocialService(repo *repository.SocialRepository) *SocialService {
	return &SocialService{repo: repo}
}
func (s *SocialService) Search(ctx context.Context, current int64, query string) ([]domain.SocialUser, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.SocialUser{}, nil
	}
	if utf8.RuneCountInString(query) > 50 {
		return nil, errors.New("搜索内容不能超过 50 个字符")
	}
	return s.repo.Search(ctx, current, query)
}
func (s *SocialService) Get(ctx context.Context, current, target int64) (domain.SocialUser, error) {
	return s.repo.Get(ctx, current, target)
}
func (s *SocialService) Follow(ctx context.Context, current, target int64) (domain.SocialUser, error) {
	if current == target {
		return domain.SocialUser{}, errors.New("不能关注自己")
	}
	if _, err := s.repo.Get(ctx, current, target); err != nil {
		return domain.SocialUser{}, err
	}
	if err := s.repo.Follow(ctx, current, target); err != nil {
		return domain.SocialUser{}, err
	}
	return s.repo.Get(ctx, current, target)
}
func (s *SocialService) Unfollow(ctx context.Context, current, target int64) (domain.SocialUser, error) {
	if err := s.repo.Unfollow(ctx, current, target); err != nil {
		return domain.SocialUser{}, err
	}
	return s.repo.Get(ctx, current, target)
}
func (s *SocialService) Following(ctx context.Context, current int64) ([]domain.SocialUser, error) {
	return s.repo.Following(ctx, current)
}
func (s *SocialService) Friends(ctx context.Context, current int64) ([]domain.SocialUser, error) {
	return s.repo.Friends(ctx, current)
}
