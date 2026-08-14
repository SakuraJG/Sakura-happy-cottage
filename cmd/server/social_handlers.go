package main

import (
	"net/http"
	"sakura-happy-cottage/internal/domain"
)

func (a *App) handleSearchUsers(w http.ResponseWriter, r *http.Request, id identity) {
	users, err := a.social.Search(r.Context(), id.User.ID, r.URL.Query().Get("q"))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
func (a *App) handleUserProfile(w http.ResponseWriter, r *http.Request, id identity) {
	uid, err := pathID(r, "uid")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	user, err := a.social.Get(r.Context(), id.User.ID, uid)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}
func (a *App) handleFollow(w http.ResponseWriter, r *http.Request, id identity) {
	uid, err := pathID(r, "uid")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	user, err := a.social.Follow(r.Context(), id.User.ID, uid)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	message := "已关注"
	if user.Friend {
		message = "已互相关注，成为好友"
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": message, "user": user})
}
func (a *App) handleUnfollow(w http.ResponseWriter, r *http.Request, id identity) {
	uid, err := pathID(r, "uid")
	if err != nil {
		handleServiceError(w, err)
		return
	}
	user, err := a.social.Unfollow(r.Context(), id.User.ID, uid)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "已取消关注", "user": user})
}
func (a *App) handleFollowing(w http.ResponseWriter, r *http.Request, id identity) {
	users, err := a.social.Following(r.Context(), id.User.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if users == nil {
		users = []domain.SocialUser{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
func (a *App) handleFriends(w http.ResponseWriter, r *http.Request, id identity) {
	users, err := a.social.Friends(r.Context(), id.User.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if users == nil {
		users = []domain.SocialUser{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
