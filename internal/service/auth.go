package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"sakura-happy-cottage/internal/config"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/platform/cache"
	"sakura-happy-cottage/internal/repository"
)

type AuthService struct {
	users    *repository.UserRepository
	tokens   *repository.TokenRepository
	settings *repository.SettingsRepository
	redis    *cache.Redis
	cfg      config.Config
}

func NewAuthService(users *repository.UserRepository, tokens *repository.TokenRepository, settings *repository.SettingsRepository, redisClient *cache.Redis, cfg config.Config) *AuthService {
	return &AuthService{users: users, tokens: tokens, settings: settings, redis: redisClient, cfg: cfg}
}

func (s *AuthService) Register(ctx context.Context, rateKey, account, username, password string) (domain.User, string, error) {
	if err := s.allow(ctx, "register:"+rateKey, 8, 15*time.Minute); err != nil {
		return domain.User{}, "", err
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return domain.User{}, "", err
	}
	if !settings.RegistrationEnabled {
		return domain.User{}, "", ErrRegistrationClosed
	}
	account = strings.TrimSpace(account)
	username = strings.TrimSpace(username)
	if !accountPattern.MatchString(account) {
		return domain.User{}, "", errors.New("账户名需为 3-32 位字母、数字或下划线")
	}
	if err := ValidateDisplayName(username); err != nil {
		return domain.User{}, "", err
	}
	if err := ValidatePassword(password); err != nil {
		return domain.User{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.Auth.BcryptCost)
	if err != nil {
		return domain.User{}, "", err
	}
	user := domain.User{Account: account, Username: username, PasswordHash: string(hash), Role: "user"}
	if err := s.users.Create(ctx, &user); err != nil {
		return domain.User{}, "", err
	}
	raw, err := s.startSession(ctx, user.ID)
	return user, raw, err
}

func (s *AuthService) Login(ctx context.Context, rateKey, account, password string) (domain.User, string, error) {
	if err := s.allow(ctx, "login:"+rateKey, 10, 10*time.Minute); err != nil {
		return domain.User{}, "", err
	}
	user, err := s.users.FindByLogin(ctx, strings.TrimSpace(account))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return domain.User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return domain.User{}, "", ErrInvalidCredentials
	}
	raw, err := s.startSession(ctx, user.ID)
	return user, raw, err
}

func (s *AuthService) Current(ctx context.Context, raw string) (domain.User, []byte, error) {
	hash := TokenHash(raw)
	userID, err := s.redis.SessionUserID(ctx, hash)
	if errors.Is(err, cache.ErrSessionNotFound) {
		return domain.User{}, nil, gorm.ErrRecordNotFound
	}
	if err != nil {
		return domain.User{}, nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}
	_ = s.redis.TouchSession(ctx, hash, userID, time.Duration(s.cfg.Auth.SessionTTLHours)*time.Hour)
	return user, hash, nil
}

func (s *AuthService) Logout(ctx context.Context, raw string, userID int64) error {
	return s.redis.DeleteSession(ctx, TokenHash(raw), userID)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, current, next string, currentHash []byte) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), s.cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return err
	}
	return s.redis.DeleteOtherSessions(ctx, userID, currentHash)
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, username, bio string) (domain.User, error) {
	username = strings.TrimSpace(username)
	bio = strings.TrimSpace(bio)
	if err := ValidateDisplayName(username); err != nil {
		return domain.User{}, err
	}
	if utf8.RuneCountInString(bio) > 160 {
		return domain.User{}, errors.New("个人简介不能超过 160 个字符")
	}
	return s.users.UpdateProfile(ctx, userID, username, bio)
}

func (s *AuthService) startSession(ctx context.Context, userID int64) (string, error) {
	raw, err := RandomToken()
	if err != nil {
		return "", err
	}
	return raw, s.redis.PutSession(ctx, TokenHash(raw), userID, time.Duration(s.cfg.Auth.SessionTTLHours)*time.Hour)
}
func (s *AuthService) allow(ctx context.Context, action string, limit int, window time.Duration) error {
	ok, err := s.redis.Allow(ctx, action, limit, window)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	return nil
}

func RandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
func TokenHash(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }
func ValidatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return errors.New("密码至少需要 8 个字符")
	}
	if len(password) > 72 {
		return errors.New("密码不能超过 72 个字节")
	}
	return nil
}
func ValidateDisplayName(value string) error {
	length := utf8.RuneCountInString(value)
	if length < 2 || length > 32 {
		return errors.New("用户名需为 2-32 个字符")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return errors.New("用户名不能包含控制字符")
		}
	}
	return nil
}
func NormalizeEmail(value string) (string, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address == "" || len(address.Address) > 254 {
		return "", errors.New("邮箱地址格式无效")
	}
	return strings.ToLower(address.Address), nil
}

var accountPattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)
var (
	ErrRegistrationClosed = errors.New("系统当前未开放注册")
	ErrInvalidCredentials = errors.New("账号或密码错误")
	ErrRateLimited        = errors.New("操作过于频繁，请稍后再试")
)
