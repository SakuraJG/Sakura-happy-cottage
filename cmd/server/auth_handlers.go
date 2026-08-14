package main

import (
	"net/http"
	"sakura-happy-cottage/internal/domain"
	"sakura-happy-cottage/internal/platform/mailer"
)

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Account  string `json:"account"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	user, raw, err := a.auth.Register(r.Context(), clientKey(r), in.Account, in.Username, in.Password)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	a.setCookie(w, raw)
	writeJSON(w, http.StatusCreated, map[string]any{"user": domain.NewUserView(user)})
}
func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	user, raw, err := a.auth.Login(r.Context(), clientKey(r), in.Account, in.Password)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	a.setCookie(w, raw)
	writeJSON(w, http.StatusOK, map[string]any{"user": domain.NewUserView(user)})
}
func (a *App) handleMe(w http.ResponseWriter, r *http.Request, id identity) {
	writeJSON(w, http.StatusOK, map[string]any{"user": domain.NewUserView(id.User)})
}
func (a *App) handleLogout(w http.ResponseWriter, r *http.Request, id identity) {
	if err := a.auth.Logout(r.Context(), id.RawToken, id.User.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	a.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request, id identity) {
	var in struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	if err := a.auth.ChangePassword(r.Context(), id.User.ID, in.Current, in.Next, id.TokenHash); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已修改"})
}
func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request, id identity) {
	var in struct {
		Username string `json:"username"`
		Bio      string `json:"bio"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	user, err := a.auth.UpdateProfile(r.Context(), id.User.ID, in.Username, in.Bio)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "个人资料已保存", "user": domain.NewUserView(user)})
}
func (a *App) handleBindEmail(w http.ResponseWriter, r *http.Request, id identity) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	verified, err := a.auth.BindEmail(r.Context(), clientKey(r), id.User.ID, in.Email, in.Password, mailer.SMTP{})
	if err != nil {
		handleServiceError(w, err)
		return
	}
	message := "确认邮件已发送"
	if verified {
		message = "邮箱已绑定"
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": message, "email_verified": verified})
}
func (a *App) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	if err := a.auth.VerifyEmail(r.Context(), in.Token); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "邮箱绑定成功"})
}
func (a *App) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	if err := a.auth.ForgotPassword(r.Context(), clientKey(r), in.Email, mailer.SMTP{}); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "如果该邮箱已绑定，重置邮件将很快发送"})
}
func (a *App) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"new_password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		handleServiceError(w, err)
		return
	}
	if err := a.auth.ResetPassword(r.Context(), in.Token, in.Password); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已重置，请重新登录"})
}
