package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"gorm.io/gorm"
	"sakura-happy-cottage/internal/config"
	"sakura-happy-cottage/internal/platform/cache"
	"sakura-happy-cottage/internal/platform/database"
	"sakura-happy-cottage/internal/repository"
	"sakura-happy-cottage/internal/service"
)

//go:embed web/dist/*
var webFS embed.FS

type App struct {
	cfg      config.Config
	db       *gorm.DB
	redis    *cache.Redis
	started  time.Time
	auth     *service.AuthService
	memos    *service.MemoService
	social   *service.SocialService
	settings *repository.SettingsRepository
	users    *repository.UserRepository
}

func main() {
	configPath := os.Getenv("SAKURA_HOME_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.Storage.UploadDir, 0o755); err != nil {
		log.Fatalf("create upload directory: %v", err)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migration: %v", err)
	}
	redisClient, err := cache.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	users := repository.NewUserRepository(db)
	settings := repository.NewSettingsRepository(db)
	if err := settings.Ensure(ctx, fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)); err != nil {
		log.Fatalf("initialize system settings: %v", err)
	}
	adminHash := ""
	if cfg.Admin.Password != "" {
		hash, hashErr := bcryptHash(cfg.Admin.Password, cfg.Auth.BcryptCost)
		if hashErr != nil {
			log.Fatal(hashErr)
		}
		adminHash = hash
	}
	if err := users.EnsureBootstrapAdmin(ctx, cfg.Admin.Username, adminHash); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	auth := service.NewAuthService(users, repository.NewTokenRepository(db), settings, redisClient, cfg)
	app := &App{cfg: cfg, db: db, redis: redisClient, started: time.Now(), auth: auth, memos: service.NewMemoService(repository.NewMemoRepository(db), cfg), social: service.NewSocialService(repository.NewSocialRepository(db)), settings: settings, users: users}
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port), Handler: app.routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Printf("Sakura的快乐小屋 listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func bcryptHash(password string, cost int) (string, error) {
	// Kept in the composition root so bootstrap credentials never cross the HTTP layer.
	return service.HashPassword(password, cost)
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings", a.handleSettings)
	mux.HandleFunc("POST /api/auth/register", a.handleRegister)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.withAuth(a.handleLogout))
	mux.HandleFunc("GET /api/auth/me", a.withAuth(a.handleMe))
	mux.HandleFunc("POST /api/auth/forgot-password", a.handleForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", a.handleResetPassword)
	mux.HandleFunc("POST /api/auth/verify-email", a.handleVerifyEmail)
	mux.HandleFunc("POST /api/account/password", a.withAuth(a.handleChangePassword))
	mux.HandleFunc("POST /api/account/email", a.withAuth(a.handleBindEmail))
	mux.HandleFunc("PATCH /api/account/profile", a.withAuth(a.handleUpdateProfile))
	mux.HandleFunc("GET /api/memos", a.withAuth(a.handleListMemos))
	mux.HandleFunc("POST /api/memos", a.withAuth(a.handleCreateMemo))
	mux.HandleFunc("PATCH /api/memos/{id}", a.withAuth(a.handleUpdateMemo))
	mux.HandleFunc("DELETE /api/memos/{id}", a.withAuth(a.handleDeleteMemo))
	mux.HandleFunc("GET /api/attachments/{id}", a.withAuth(a.handleAttachment))
	mux.HandleFunc("GET /api/users/search", a.withAuth(a.handleSearchUsers))
	mux.HandleFunc("GET /api/users/{uid}", a.withAuth(a.handleUserProfile))
	mux.HandleFunc("POST /api/users/{uid}/follow", a.withAuth(a.handleFollow))
	mux.HandleFunc("DELETE /api/users/{uid}/follow", a.withAuth(a.handleUnfollow))
	mux.HandleFunc("GET /api/social/following", a.withAuth(a.handleFollowing))
	mux.HandleFunc("GET /api/social/friends", a.withAuth(a.handleFriends))
	mux.HandleFunc("GET /api/admin/system", a.withAdmin(a.handleSystemInfo))
	mux.HandleFunc("PUT /api/admin/system", a.withAdmin(a.handleUpdateSystem))
	static := http.FileServer(http.FS(webFSSubtree()))
	mux.Handle("/assets/", static)
	mux.HandleFunc("/", a.handleFrontend)
	return logging(security(mux))
}

func webFSSubtree() fs.FS {
	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	return dist
}

func (a *App) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	if r.URL.Path != "/" && path.Ext(r.URL.Path) != "" {
		http.NotFound(w, r)
		return
	}
	data, err := webFS.ReadFile("web/dist/index.html")
	if err != nil {
		http.Error(w, "frontend unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
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
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}
