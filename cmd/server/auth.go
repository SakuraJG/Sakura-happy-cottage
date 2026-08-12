package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const identityContextKey contextKey = "identity"

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

type User struct {
	ID                 int64     `json:"id"`
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
	Email              string    `json:"email,omitempty"`
	EmailVerified      bool      `json:"email_verified"`
	CreatedAt          time.Time `json:"created_at"`
}

type sessionIdentity struct {
	User      User
	TokenHash []byte
}

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{attempts: make(map[string][]time.Time)}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-window)
	items := l.attempts[key]
	kept := items[:0]
	for _, item := range items {
		if item.After(cutoff) {
			kept = append(kept, item)
		}
	}
	if len(kept) >= limit {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, time.Now())
	return true
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(a.config.Auth.CookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		hash := tokenHash(cookie.Value)
		var identity sessionIdentity
		err = a.db.QueryRow(r.Context(), `
			SELECT u.id, u.username, u.role, u.must_change_password, COALESCE(u.email, ''), u.email_verified, u.created_at
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.expires_at > NOW()`, hash,
		).Scan(&identity.User.ID, &identity.User.Username, &identity.User.Role, &identity.User.MustChangePassword, &identity.User.Email, &identity.User.EmailVerified, &identity.User.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			a.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		if err != nil {
			writeInternalError(w, err)
			return
		}
		identity.TokenHash = hash
		_, _ = a.db.Exec(r.Context(), "UPDATE sessions SET last_seen_at = NOW() WHERE token_hash = $1", hash)
		ctx := context.WithValue(r.Context(), identityContextKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentIdentity(r).User.Role != "admin" {
			writeError(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func currentIdentity(r *http.Request) sessionIdentity {
	identity, _ := r.Context().Value(identityContextKey).(sessionIdentity)
	return identity
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if !settings.RegistrationEnabled {
		writeError(w, http.StatusForbidden, "系统当前未开放注册")
		return
	}
	if !a.allowAuthAttempt(r, "register", 8, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if !usernamePattern.MatchString(input.Username) {
		writeError(w, http.StatusBadRequest, "用户名需为 3-32 位字母、数字或下划线")
		return
	}
	if err := validatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), a.config.Auth.BcryptCost)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	var user User
	err = a.db.QueryRow(r.Context(), `
		INSERT INTO users(username, password_hash)
		VALUES($1, $2)
		RETURNING id, username, role, must_change_password, COALESCE(email, ''), email_verified, created_at`,
		input.Username, string(hash),
	).Scan(&user.ID, &user.Username, &user.Role, &user.MustChangePassword, &user.Email, &user.EmailVerified, &user.CreatedAt)
	if uniqueViolation(err) {
		writeError(w, http.StatusConflict, "用户名已存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := a.db.Exec(r.Context(), `
		UPDATE memos SET user_id = $1
		WHERE user_id IS NULL AND (SELECT COUNT(*) FROM users) = 1`, user.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.startSession(r.Context(), w, user.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r, "login", 10, 10*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
		return
	}
	var input struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	account := strings.TrimSpace(input.Account)
	var user User
	var passwordHash string
	err := a.db.QueryRow(r.Context(), `
		SELECT id, username, role, must_change_password, COALESCE(email, ''), email_verified, created_at, password_hash
		FROM users
		WHERE lower(username) = lower($1)
		   OR (email_verified = TRUE AND lower(email) = lower($1))`, account,
	).Scan(&user.ID, &user.Username, &user.Role, &user.MustChangePassword, &user.Email, &user.EmailVerified, &user.CreatedAt, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	if err := a.startSession(r.Context(), w, user.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": currentIdentity(r).User})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r)
	_, err := a.db.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash = $1", identity.TokenHash)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validatePassword(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	identity := currentIdentity(r)
	var currentHash string
	if err := a.db.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id = $1", identity.User.ID).Scan(&currentHash); err != nil {
		writeInternalError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "当前密码错误")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), a.config.Auth.BcryptCost)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = NOW() WHERE id = $2", string(newHash), identity.User.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2", identity.User.ID, identity.TokenHash); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已修改"})
}

func (a *App) handleBindEmail(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if settings.EmailConfirmationRequired && !settings.SMTPEnabled {
		writeError(w, http.StatusServiceUnavailable, "邮件服务尚未配置")
		return
	}
	if !a.allowAuthAttempt(r, "bind-email", 5, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "邮件发送过于频繁，请稍后再试")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	identity := currentIdentity(r)
	var passwordHash string
	if err := a.db.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id = $1", identity.User.ID).Scan(&passwordHash); err != nil {
		writeInternalError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "当前密码错误")
		return
	}
	var exists bool
	if err := a.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = lower($1) AND id <> $2)", email, identity.User.ID).Scan(&exists); err != nil {
		writeInternalError(w, err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "该邮箱已被其他账号使用")
		return
	}
	if !settings.EmailConfirmationRequired {
		_, err := a.db.Exec(r.Context(), "UPDATE users SET email = $1, email_verified = TRUE, updated_at = NOW() WHERE id = $2", email, identity.User.ID)
		if uniqueViolation(err) {
			writeError(w, http.StatusConflict, "该邮箱已被其他账号使用")
			return
		}
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "邮箱已绑定", "email": email, "email_verified": true})
		return
	}
	rawToken, err := a.createAuthToken(r.Context(), identity.User.ID, "email_verify", email, time.Duration(a.config.Auth.EmailVerifyTTLHours)*time.Hour)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.sendVerificationEmail(settings, email, rawToken); err != nil {
		_, _ = a.db.Exec(r.Context(), "DELETE FROM auth_tokens WHERE token_hash = $1", tokenHash(rawToken))
		log.Printf("send verification email: %v", err)
		writeError(w, http.StatusBadGateway, "确认邮件发送失败，请检查 SMTP 配置")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "确认邮件已发送"})
}

func (a *App) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Token == "" {
		writeError(w, http.StatusBadRequest, "确认链接无效")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var userID int64
	var email string
	err = tx.QueryRow(r.Context(), `
		SELECT user_id, pending_email
		FROM auth_tokens
		WHERE token_hash = $1 AND purpose = 'email_verify' AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE`, tokenHash(input.Token),
	).Scan(&userID, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "确认链接无效或已过期")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), "UPDATE users SET email = $1, email_verified = TRUE, updated_at = NOW() WHERE id = $2", email, userID)
	if uniqueViolation(err) {
		writeError(w, http.StatusConflict, "该邮箱已被其他账号使用")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "UPDATE auth_tokens SET used_at = NOW() WHERE token_hash = $1", tokenHash(input.Token)); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "邮箱绑定成功"})
}

func (a *App) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if !settings.PasswordRecoveryEnabled || !settings.SMTPEnabled {
		writeError(w, http.StatusServiceUnavailable, "邮件服务尚未配置")
		return
	}
	if !a.allowAuthAttempt(r, "forgot-password", 5, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var userID int64
	err = a.db.QueryRow(r.Context(), "SELECT id FROM users WHERE lower(email) = lower($1) AND email_verified = TRUE", email).Scan(&userID)
	if err == nil {
		rawToken, tokenErr := a.createAuthToken(r.Context(), userID, "password_reset", "", time.Duration(a.config.Auth.PasswordResetTTLMinutes)*time.Minute)
		if tokenErr != nil {
			log.Printf("create password reset token: %v", tokenErr)
		} else if sendErr := a.sendPasswordResetEmail(settings, email, rawToken); sendErr != nil {
			log.Printf("send password reset email: %v", sendErr)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "如果该邮箱已绑定，重置邮件将很快发送"})
}

func (a *App) ensureBootstrapAdmin(ctx context.Context) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var adminID int64
	var role string
	err = tx.QueryRow(ctx, "SELECT id, role FROM users WHERE lower(username) = lower($1)", a.config.Admin.Username).Scan(&adminID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		if a.config.Admin.Password == "" {
			return errors.New("bootstrap admin does not exist; set SAKURA_HOME_ADMIN_PASSWORD for the first startup")
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(a.config.Admin.Password), a.config.Auth.BcryptCost)
		if hashErr != nil {
			return hashErr
		}
		err = tx.QueryRow(ctx, `
		INSERT INTO users(username, password_hash, role, must_change_password)
		VALUES($1, $2, 'admin', TRUE)
		RETURNING id`, a.config.Admin.Username, string(hash)).Scan(&adminID)
	} else if err == nil && role != "admin" {
		return fmt.Errorf("bootstrap username %q belongs to a non-admin account; refusing automatic privilege escalation", a.config.Admin.Username)
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE memos SET user_id = $1 WHERE user_id IS NULL", adminID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Token == "" {
		writeError(w, http.StatusBadRequest, "重置链接无效")
		return
	}
	if err := validatePassword(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), a.config.Auth.BcryptCost)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var userID int64
	err = tx.QueryRow(r.Context(), `
		SELECT user_id FROM auth_tokens
		WHERE token_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE`, tokenHash(input.Token),
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "重置链接无效或已过期")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = NOW() WHERE id = $2", string(passwordHash), userID); err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "UPDATE auth_tokens SET used_at = NOW() WHERE token_hash = $1", tokenHash(input.Token)); err != nil {
		writeInternalError(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已重置，请重新登录"})
}

func (a *App) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	rawToken, err := randomToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(time.Duration(a.config.Auth.SessionTTLHours) * time.Hour)
	if _, err := a.db.Exec(ctx, `
		INSERT INTO sessions(user_id, token_hash, expires_at)
		VALUES($1, $2, $3)`, userID, tokenHash(rawToken), expiresAt); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     a.config.Auth.CookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
	return nil
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.config.Auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
}

func (a *App) createAuthToken(ctx context.Context, userID int64, purpose, pendingEmail string, ttl time.Duration) (string, error) {
	rawToken, err := randomToken()
	if err != nil {
		return "", err
	}
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "DELETE FROM auth_tokens WHERE user_id = $1 AND purpose = $2", userID, purpose); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_tokens(user_id, token_hash, purpose, pending_email, expires_at)
		VALUES($1, $2, $3, NULLIF($4, ''), $5)`,
		userID, tokenHash(rawToken), purpose, pendingEmail, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return rawToken, nil
}

func (a *App) allowAuthAttempt(r *http.Request, action string, limit int, window time.Duration) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return a.limiter.allow(action+":"+host, limit, window)
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func validatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return errors.New("密码至少需要 8 个字符")
	}
	if len(password) > 72 {
		return errors.New("密码不能超过 72 个字节")
	}
	return nil
}

func normalizeEmail(value string) (string, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address == "" || len(address.Address) > 254 {
		return "", errors.New("邮箱地址格式无效")
	}
	return strings.ToLower(address.Address), nil
}

func decodeJSON(r *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("JSON 内容无效")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON 内容无效")
	}
	return nil
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
