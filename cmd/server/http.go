package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/service"
)

type identityKey struct{}

type identity struct {
	User      domain.User
	TokenHash []byte
	RawToken  string
}

type authHandler func(http.ResponseWriter, *http.Request, identity)

func (a *App) withAuth(next authHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(a.cfg.Auth.CookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		user, hash, err := a.auth.Current(r.Context(), cookie.Value)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			a.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		if err != nil {
			writeInternalError(w, err)
			return
		}
		id := identity{User: user, TokenHash: hash, RawToken: cookie.Value}
		next(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, id)), id)
	}
}

func (a *App) withAdmin(next authHandler) http.HandlerFunc {
	return a.withAuth(func(w http.ResponseWriter, r *http.Request, id identity) {
		if id.User.Role != "admin" {
			writeError(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		next(w, r, id)
	})
}

func (a *App) setCookie(w http.ResponseWriter, raw string) {
	ttl := time.Duration(a.cfg.Auth.SessionTTLHours) * time.Hour
	http.SetCookie(w, &http.Cookie{Name: a.cfg.Auth.CookieName, Value: raw, Path: "/", HttpOnly: true, Secure: a.cfg.Auth.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(ttl), MaxAge: int(ttl.Seconds())})
}

func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: a.cfg.Auth.CookieName, Path: "/", HttpOnly: true, Secure: a.cfg.Auth.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("JSON 内容无效")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON 内容无效")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("request failed: %v", err)
	writeError(w, http.StatusInternalServerError, "服务器处理失败")
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, service.ErrRegistrationClosed):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrMailUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrEmailInUse):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInvalidToken):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "资源不存在")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func pathID(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("ID 无效")
	}
	return value, nil
}
