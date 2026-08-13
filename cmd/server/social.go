package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type socialUser struct {
	UID            int64     `json:"uid"`
	Username       string    `json:"username"`
	Bio            string    `json:"bio"`
	CreatedAt      time.Time `json:"created_at"`
	Following      bool      `json:"following"`
	FollowedBy     bool      `json:"followed_by"`
	Friend         bool      `json:"friend"`
	IsSelf         bool      `json:"is_self"`
	FollowingCount int64     `json:"following_count"`
	FollowerCount  int64     `json:"follower_count"`
	FriendCount    int64     `json:"friend_count"`
}

const socialUserSelect = `
	SELECT u.id, u.display_name, u.bio, u.created_at,
	       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = $1 AND f.followed_id = u.id) AS following,
	       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = u.id AND f.followed_id = $1) AS followed_by,
	       EXISTS(SELECT 1 FROM follows f1 JOIN follows f2 ON f2.follower_id = f1.followed_id AND f2.followed_id = f1.follower_id WHERE f1.follower_id = $1 AND f1.followed_id = u.id) AS friend,
	       u.id = $1 AS is_self,
	       (SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.id) AS following_count,
	       (SELECT COUNT(*) FROM follows f WHERE f.followed_id = u.id) AS follower_count,
	       (SELECT COUNT(*) FROM follows f1 JOIN follows f2 ON f2.follower_id = f1.followed_id AND f2.followed_id = f1.follower_id WHERE f1.follower_id = u.id) AS friend_count
	FROM users u`

func scanSocialUser(row pgx.Row) (socialUser, error) {
	var user socialUser
	err := row.Scan(
		&user.UID, &user.Username, &user.Bio, &user.CreatedAt,
		&user.Following, &user.FollowedBy, &user.Friend, &user.IsSelf,
		&user.FollowingCount, &user.FollowerCount, &user.FriendCount,
	)
	return user, err
}

func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Bio      string `json:"bio"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Bio = strings.TrimSpace(input.Bio)
	if err := validateDisplayName(input.Username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(input.Bio) > 160 {
		writeError(w, http.StatusBadRequest, "个人简介不能超过 160 个字符")
		return
	}
	identity := currentIdentity(r)
	var user User
	err := a.db.QueryRow(r.Context(), `
		UPDATE users SET display_name = $1, bio = $2, updated_at = NOW() WHERE id = $3
		RETURNING id, id, username, display_name, bio, role, must_change_password, COALESCE(email, ''), email_verified, created_at`,
		input.Username, input.Bio, identity.User.ID,
	).Scan(&user.ID, &user.UID, &user.Account, &user.Username, &user.Bio, &user.Role, &user.MustChangePassword, &user.Email, &user.EmailVerified, &user.CreatedAt)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "个人资料已保存", "user": user})
}

func validateDisplayName(value string) error {
	length := utf8.RuneCountInString(value)
	if length < 2 || length > 32 {
		return errors.New("用户名需为 2-32 个字符")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("用户名不能包含控制字符")
		}
	}
	return nil
}

func (a *App) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{"users": []socialUser{}})
		return
	}
	if utf8.RuneCountInString(query) > 50 {
		writeError(w, http.StatusBadRequest, "搜索内容不能超过 50 个字符")
		return
	}
	uid, _ := strconv.ParseInt(query, 10, 64)
	currentUserID := currentIdentity(r).User.ID
	rows, err := a.db.Query(r.Context(), socialUserSelect+`
		WHERE strpos(lower(u.display_name), lower($2)) > 0 OR ($3 > 0 AND u.id = $3)
		ORDER BY CASE WHEN u.id = $3 THEN 0 ELSE 1 END, u.display_name, u.id
		LIMIT 20`, currentUserID, query, uid)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer rows.Close()
	users := make([]socialUser, 0)
	for rows.Next() {
		user, scanErr := scanSocialUser(rows)
		if scanErr != nil {
			writeInternalError(w, scanErr)
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) handleUserProfile(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseUID(w, r)
	if !ok {
		return
	}
	user, err := scanSocialUser(a.db.QueryRow(r.Context(), socialUserSelect+" WHERE u.id = $2", currentIdentity(r).User.ID, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) handleFollowUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseUID(w, r)
	if !ok {
		return
	}
	currentUserID := currentIdentity(r).User.ID
	if uid == currentUserID {
		writeError(w, http.StatusBadRequest, "不能关注自己")
		return
	}
	result, err := a.db.Exec(r.Context(), `
		INSERT INTO follows(follower_id, followed_id)
		SELECT $1, id FROM users WHERE id = $2
		ON CONFLICT DO NOTHING`, currentUserID, uid)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := a.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", uid).Scan(&exists); err != nil {
			writeInternalError(w, err)
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
	}
	user, err := scanSocialUser(a.db.QueryRow(r.Context(), socialUserSelect+" WHERE u.id = $2", currentUserID, uid))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	message := "已关注"
	if user.Friend {
		message = "已互相关注，成为好友"
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": message, "user": user})
}

func (a *App) handleUnfollowUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseUID(w, r)
	if !ok {
		return
	}
	currentUserID := currentIdentity(r).User.ID
	if _, err := a.db.Exec(r.Context(), "DELETE FROM follows WHERE follower_id = $1 AND followed_id = $2", currentUserID, uid); err != nil {
		writeInternalError(w, err)
		return
	}
	user, err := scanSocialUser(a.db.QueryRow(r.Context(), socialUserSelect+" WHERE u.id = $2", currentUserID, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "已取消关注", "user": user})
}

func (a *App) handleFollowing(w http.ResponseWriter, r *http.Request) {
	a.handleSocialList(w, r, `
		JOIN follows mine ON mine.followed_id = u.id AND mine.follower_id = $1
		ORDER BY mine.created_at DESC`)
}

func (a *App) handleFriends(w http.ResponseWriter, r *http.Request) {
	a.handleSocialList(w, r, `
		JOIN follows mine ON mine.followed_id = u.id AND mine.follower_id = $1
		JOIN follows theirs ON theirs.follower_id = u.id AND theirs.followed_id = $1
		ORDER BY GREATEST(mine.created_at, theirs.created_at) DESC`)
}

func (a *App) handleSocialList(w http.ResponseWriter, r *http.Request, joins string) {
	rows, err := a.db.Query(r.Context(), socialUserSelect+joins, currentIdentity(r).User.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer rows.Close()
	users := make([]socialUser, 0)
	for rows.Next() {
		user, scanErr := scanSocialUser(rows)
		if scanErr != nil {
			writeInternalError(w, scanErr)
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func parseUID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil || uid <= 0 {
		writeError(w, http.StatusBadRequest, "UID 无效")
		return 0, false
	}
	return uid, true
}
