package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"sakura-happy-cottage/internal/config"
)

//go:embed web/*
var webFS embed.FS

type App struct {
	config    config.Config
	db        *pgxpool.Pool
	limiter   *rateLimiter
	startedAt time.Time
}

type Memo struct {
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	ID           int64  `json:"id"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	storedName   string
}

type memoInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func main() {
	configPath := "config.yaml"
	if value := os.Getenv("SAKURA_HOME_CONFIG"); value != "" {
		configPath = value
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.Storage.UploadDir, 0o755); err != nil {
		log.Fatalf("create upload directory: %v", err)
	}

	ctx := context.Background()
	pool, err := connectDatabase(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	app := &App{config: cfg, db: pool, limiter: newRateLimiter(), startedAt: time.Now()}
	if err := app.migrate(ctx); err != nil {
		log.Fatalf("database migration: %v", err)
	}
	if err := app.ensureSystemSettings(ctx); err != nil {
		log.Fatalf("initialize system settings: %v", err)
	}
	if err := app.ensureBootstrapAdmin(ctx); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Sakura的快乐小屋 listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func connectDatabase(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	address := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Database.User, cfg.Database.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Database.Host, cfg.Database.Port),
		Path:   cfg.Database.Name,
	}
	query := address.Query()
	query.Set("sslmode", cfg.Database.SSLMode)
	address.RawQuery = query.Encode()
	poolConfig, err := pgxpool.ParseConfig(address.String())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	var pool *pgxpool.Pool
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		log.Printf("waiting for postgres (attempt %d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect to postgres: %w", err)
}

func (a *App) migrate(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
			must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
			email TEXT,
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
		ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT NOT NULL DEFAULT '';
		UPDATE users SET display_name = username WHERE display_name IS NULL OR btrim(display_name) = '';
		ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users(lower(username));
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users(lower(email)) WHERE email IS NOT NULL;
		CREATE TABLE IF NOT EXISTS sessions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash BYTEA NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
		CREATE TABLE IF NOT EXISTS auth_tokens (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash BYTEA NOT NULL UNIQUE,
			purpose TEXT NOT NULL CHECK (purpose IN ('email_verify', 'password_reset')),
			pending_email TEXT,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_purpose ON auth_tokens(user_id, purpose);
		CREATE TABLE IF NOT EXISTS follows (
			follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			followed_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (follower_id, followed_id),
			CHECK (follower_id <> followed_id)
		);
		CREATE INDEX IF NOT EXISTS idx_follows_followed_id ON follows(followed_id);
		CREATE TABLE IF NOT EXISTS system_settings (
			singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			registration_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			email_confirmation_required BOOLEAN NOT NULL DEFAULT FALSE,
			password_recovery_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			public_url TEXT NOT NULL,
			smtp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			smtp_host TEXT NOT NULL DEFAULT '',
			smtp_port INTEGER NOT NULL DEFAULT 587,
			smtp_username TEXT NOT NULL DEFAULT '',
			smtp_password TEXT NOT NULL DEFAULT '',
			smtp_from_name TEXT NOT NULL DEFAULT 'Sakura的快乐小屋',
			smtp_from_email TEXT NOT NULL DEFAULT '',
			smtp_encryption TEXT NOT NULL DEFAULT 'starttls' CHECK (smtp_encryption IN ('none', 'starttls', 'tls')),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE system_settings ALTER COLUMN smtp_from_name SET DEFAULT 'Sakura的快乐小屋';
		UPDATE system_settings
		SET smtp_from_name = 'Sakura的快乐小屋', updated_at = NOW()
		WHERE smtp_from_name IN (U&'\5907\5FD8\5F55', 'Sakura''s Happy Cottage');
		CREATE TABLE IF NOT EXISTS memos (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE memos ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
		CREATE TABLE IF NOT EXISTS attachments (
			id BIGSERIAL PRIMARY KEY,
			memo_id BIGINT NOT NULL REFERENCES memos(id) ON DELETE CASCADE,
			original_name TEXT NOT NULL,
			stored_name TEXT NOT NULL UNIQUE,
			content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			size BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_memos_user_status_updated ON memos(user_id, status, updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_attachments_memo_id ON attachments(memo_id);
	`)
	return err
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleIndex)
	mux.HandleFunc("GET /games", a.handleIndex)
	mux.HandleFunc("GET /people", a.handleIndex)
	mux.HandleFunc("GET /profile/", a.handleIndex)
	mux.HandleFunc("GET /assets/", a.handleAsset)
	mux.HandleFunc("GET /api/settings", a.handleSettings)
	mux.HandleFunc("POST /api/auth/register", a.handleRegister)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/forgot-password", a.handleForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", a.handleResetPassword)
	mux.HandleFunc("POST /api/auth/verify-email", a.handleVerifyEmail)
	mux.Handle("GET /api/auth/me", a.requireAuth(http.HandlerFunc(a.handleMe)))
	mux.Handle("POST /api/auth/logout", a.requireAuth(http.HandlerFunc(a.handleLogout)))
	mux.Handle("POST /api/account/password", a.requireAuth(http.HandlerFunc(a.handleChangePassword)))
	mux.Handle("POST /api/account/email", a.requireAuth(http.HandlerFunc(a.handleBindEmail)))
	mux.Handle("PATCH /api/account/profile", a.requireAuth(http.HandlerFunc(a.handleUpdateProfile)))
	mux.Handle("GET /api/users/search", a.requireAuth(http.HandlerFunc(a.handleSearchUsers)))
	mux.Handle("GET /api/users/{uid}", a.requireAuth(http.HandlerFunc(a.handleUserProfile)))
	mux.Handle("POST /api/users/{uid}/follow", a.requireAuth(http.HandlerFunc(a.handleFollowUser)))
	mux.Handle("DELETE /api/users/{uid}/follow", a.requireAuth(http.HandlerFunc(a.handleUnfollowUser)))
	mux.Handle("GET /api/social/following", a.requireAuth(http.HandlerFunc(a.handleFollowing)))
	mux.Handle("GET /api/social/friends", a.requireAuth(http.HandlerFunc(a.handleFriends)))
	mux.Handle("GET /api/admin/system", a.requireAdmin(http.HandlerFunc(a.handleSystemInfo)))
	mux.Handle("PUT /api/admin/system", a.requireAdmin(http.HandlerFunc(a.handleUpdateSystemSettings)))
	mux.Handle("GET /api/memos", a.requireAuth(http.HandlerFunc(a.handleList)))
	mux.Handle("POST /api/memos", a.requireAuth(http.HandlerFunc(a.handleCreate)))
	mux.Handle("PATCH /api/memos/", a.requireAuth(http.HandlerFunc(a.handleUpdate)))
	mux.Handle("DELETE /api/memos/", a.requireAuth(http.HandlerFunc(a.handleDelete)))
	mux.Handle("GET /api/attachments/", a.requireAuth(http.HandlerFunc(a.handleAttachment)))
	return loggingMiddleware(securityMiddleware(mux))
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadSystemSettings(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"max_upload_bytes":            a.config.Storage.MaxUploadBytes,
		"smtp_enabled":                settings.SMTPEnabled,
		"registration_enabled":        settings.RegistrationEnabled,
		"email_confirmation_required": settings.EmailConfirmationRequired,
		"password_recovery_enabled":   settings.PasswordRecoveryEnabled,
	})
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
					writeError(w, http.StatusForbidden, "请求来源无效")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/games" && r.URL.Path != "/people" && !strings.HasPrefix(r.URL.Path, "/profile/") {
		http.NotFound(w, r)
		return
	}
	serveEmbedded(w, "web/index.html", "text/html; charset=utf-8")
}

func (a *App) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	switch name {
	case "styles.css":
		serveEmbedded(w, "web/styles.css", "text/css; charset=utf-8")
	case "app.js":
		serveEmbedded(w, "web/app.js", "text/javascript; charset=utf-8")
	case "logo.svg":
		serveEmbedded(w, "web/logo.svg", "image/svg+xml")
	default:
		http.NotFound(w, r)
	}
}

func serveEmbedded(w http.ResponseWriter, name, contentType string) {
	content, err := webFS.ReadFile(name)
	if err != nil {
		http.Error(w, "frontend unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(content)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	userID := currentIdentity(r).User.ID
	status := r.URL.Query().Get("status")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	query := `SELECT id, title, description, status, created_at, completed_at, updated_at FROM memos WHERE user_id = $1`
	args := []any{userID}
	conditions := make([]string, 0, 2)
	if status == "open" || status == "done" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", len(args), len(args)))
	}
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY CASE WHEN status = 'open' THEN 0 ELSE 1 END, updated_at DESC"
	rows, err := a.db.Query(r.Context(), query, args...)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer rows.Close()
	memos := make([]Memo, 0)
	for rows.Next() {
		var memo Memo
		if err := rows.Scan(&memo.ID, &memo.Title, &memo.Description, &memo.Status, &memo.CreatedAt, &memo.CompletedAt, &memo.UpdatedAt); err != nil {
			writeInternalError(w, err)
			return
		}
		memo.Attachments, err = a.attachments(r.Context(), memo.ID, userID)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		memos = append(memos, memo)
	}
	if err := rows.Err(); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memos": memos, "count": len(memos)})
}

func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := currentIdentity(r).User.ID
	r.Body = http.MaxBytesReader(w, r.Body, a.config.Storage.MaxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "提交内容无效或附件总大小超过限制")
		return
	}
	input := memoInput{
		Title:       strings.TrimSpace(r.FormValue("title")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Status:      r.FormValue("status"),
	}
	if err := validateMemoInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	files := r.MultipartForm.File["attachments"]
	var total int64
	for _, file := range files {
		total += file.Size
	}
	if total > a.config.Storage.MaxUploadBytes {
		writeError(w, http.StatusBadRequest, "附件总大小超过限制")
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var memo Memo
	err = tx.QueryRow(r.Context(), `
		INSERT INTO memos(user_id, title, description, status, completed_at)
		VALUES($1, $2, $3, $4, CASE WHEN $4 = 'done' THEN NOW() ELSE NULL END)
		RETURNING id, title, description, status, created_at, completed_at, updated_at`,
		userID, input.Title, input.Description, input.Status,
	).Scan(&memo.ID, &memo.Title, &memo.Description, &memo.Status, &memo.CreatedAt, &memo.CompletedAt, &memo.UpdatedAt)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	attachments, storedPaths, err := a.saveAttachments(r.Context(), tx, memo.ID, files)
	if err != nil {
		removeFiles(storedPaths)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		removeFiles(storedPaths)
		writeInternalError(w, err)
		return
	}
	memo.Attachments = attachments
	writeJSON(w, http.StatusCreated, memo)
}

func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	userID := currentIdentity(r).User.ID
	id, err := pathID(r.URL.Path, "/api/memos/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var input memoInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 内容无效")
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if err := validateMemoInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var memo Memo
	err = a.db.QueryRow(r.Context(), `
		UPDATE memos
		SET title = $1, description = $2, status = $3,
			completed_at = CASE WHEN $3 = 'done' THEN COALESCE(completed_at, NOW()) ELSE NULL END,
			updated_at = NOW()
		WHERE id = $4 AND user_id = $5
		RETURNING id, title, description, status, created_at, completed_at, updated_at`,
		input.Title, input.Description, input.Status, id, userID,
	).Scan(&memo.ID, &memo.Title, &memo.Description, &memo.Status, &memo.CreatedAt, &memo.CompletedAt, &memo.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	memo.Attachments, err = a.attachments(r.Context(), memo.ID, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memo)
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := currentIdentity(r).User.ID
	id, err := pathID(r.URL.Path, "/api/memos/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	attachments, err := a.attachments(r.Context(), id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	result, err := a.db.Exec(r.Context(), "DELETE FROM memos WHERE id = $1 AND user_id = $2", id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	for _, attachment := range attachments {
		if err := os.Remove(filepath.Join(a.config.Storage.UploadDir, attachment.storedName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove attachment %d: %v", attachment.ID, err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAttachment(w http.ResponseWriter, r *http.Request) {
	userID := currentIdentity(r).User.ID
	id, err := pathID(r.URL.Path, "/api/attachments/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var attachment Attachment
	err = a.db.QueryRow(r.Context(), `
		SELECT a.id, a.original_name, a.stored_name, a.content_type, a.size
		FROM attachments a
		JOIN memos m ON m.id = a.memo_id
		WHERE a.id = $1 AND m.user_id = $2`, id, userID,
	).Scan(&attachment.ID, &attachment.OriginalName, &attachment.storedName, &attachment.ContentType, &attachment.Size)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		writeInternalError(w, err)
		return
	}
	file, err := os.Open(filepath.Join(a.config.Storage.UploadDir, attachment.storedName))
	if err != nil {
		writeError(w, http.StatusNotFound, "附件文件不存在")
		return
	}
	defer file.Close()
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" && safeInlineImage(attachment.ContentType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": attachment.OriginalName}))
	http.ServeContent(w, r, attachment.OriginalName, time.Time{}, file)
}

func (a *App) attachments(ctx context.Context, memoID, userID int64) ([]Attachment, error) {
	rows, err := a.db.Query(ctx, `
		SELECT a.id, a.original_name, a.stored_name, a.content_type, a.size
		FROM attachments a
		JOIN memos m ON m.id = a.memo_id
		WHERE a.memo_id = $1 AND m.user_id = $2
		ORDER BY a.id`, memoID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Attachment, 0)
	for rows.Next() {
		var item Attachment
		if err := rows.Scan(&item.ID, &item.OriginalName, &item.storedName, &item.ContentType, &item.Size); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *App) saveAttachments(ctx context.Context, tx pgx.Tx, memoID int64, files []*multipart.FileHeader) ([]Attachment, []string, error) {
	attachments := make([]Attachment, 0, len(files))
	storedPaths := make([]string, 0, len(files))
	for _, header := range files {
		storedName, err := uniqueFileName(header.Filename)
		if err != nil {
			return nil, storedPaths, err
		}
		target := filepath.Join(a.config.Storage.UploadDir, storedName)
		if err := saveUploadedFile(header, target); err != nil {
			return nil, storedPaths, fmt.Errorf("保存附件 %q 失败: %w", header.Filename, err)
		}
		storedPaths = append(storedPaths, target)
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		attachment := Attachment{
			OriginalName: filepath.Base(header.Filename),
			ContentType:  contentType,
			Size:         header.Size,
			storedName:   storedName,
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO attachments(memo_id, original_name, stored_name, content_type, size)
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			memoID, attachment.OriginalName, attachment.storedName, attachment.ContentType, attachment.Size,
		).Scan(&attachment.ID)
		if err != nil {
			return nil, storedPaths, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, storedPaths, nil
}

func uniqueFileName(original string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate attachment name: %w", err)
	}
	extension := strings.ToLower(filepath.Ext(original))
	if len(extension) > 12 {
		extension = ""
	}
	for _, char := range strings.TrimPrefix(extension, ".") {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			extension = ""
			break
		}
	}
	return hex.EncodeToString(buffer) + extension, nil
}

func saveUploadedFile(header *multipart.FileHeader, target string) error {
	source, err := header.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func removeFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func validateMemoInput(input *memoInput) error {
	if input.Title == "" {
		return errors.New("标题不能为空")
	}
	if len([]rune(input.Title)) > 120 {
		return errors.New("标题不能超过 120 个字符")
	}
	if input.Status != "open" && input.Status != "done" {
		return errors.New("状态只能是 open 或 done")
	}
	return nil
}

func pathID(path, prefix string) (int64, error) {
	value := strings.TrimPrefix(path, prefix)
	if value == path || strings.Contains(value, "/") || value == "" {
		return 0, errors.New("资源编号无效")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("资源编号无效")
	}
	return id, nil
}

func safeInlineImage(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
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
