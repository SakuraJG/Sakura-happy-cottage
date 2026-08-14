package cache

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"sakura-happy-cottage/internal/config"
)

type Redis struct {
	client *redis.Client
	prefix string
}

func Open(ctx context.Context, cfg config.Config) (*Redis, error) {
	options := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	if cfg.Redis.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(options)
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = client.Ping(pingCtx).Err()
		cancel()
		if err == nil {
			return &Redis{client: client, prefix: cfg.Redis.KeyPrefix}, nil
		}
		time.Sleep(2 * time.Second)
	}
	_ = client.Close()
	return nil, fmt.Errorf("connect to redis: %w", err)
}

func (r *Redis) Close() error { return r.client.Close() }

func (r *Redis) PutSession(ctx context.Context, hash []byte, userID int64, ttl time.Duration) error {
	member := hex.EncodeToString(hash)
	sessionKey := r.key("session", member)
	userKey := r.key("user-sessions", strconv.FormatInt(userID, 10))
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, sessionKey, userID, ttl)
	pipe.SAdd(ctx, userKey, member)
	pipe.Expire(ctx, userKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) SessionUserID(ctx context.Context, hash []byte) (int64, error) {
	value, err := r.client.Get(ctx, r.key("session", hex.EncodeToString(hash))).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, ErrSessionNotFound
	}
	return value, err
}

func (r *Redis) TouchSession(ctx context.Context, hash []byte, userID int64, ttl time.Duration) error {
	pipe := r.client.TxPipeline()
	pipe.Expire(ctx, r.key("session", hex.EncodeToString(hash)), ttl)
	pipe.Expire(ctx, r.key("user-sessions", strconv.FormatInt(userID, 10)), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) DeleteSession(ctx context.Context, hash []byte, userID int64) error {
	member := hex.EncodeToString(hash)
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, r.key("session", member))
	pipe.SRem(ctx, r.key("user-sessions", strconv.FormatInt(userID, 10)), member)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Redis) DeleteOtherSessions(ctx context.Context, userID int64, keepHash []byte) error {
	return r.deleteUserSessions(ctx, userID, hex.EncodeToString(keepHash))
}

func (r *Redis) DeleteUserSessions(ctx context.Context, userID int64) error {
	return r.deleteUserSessions(ctx, userID, "")
}

func (r *Redis) deleteUserSessions(ctx context.Context, userID int64, keep string) error {
	userKey := r.key("user-sessions", strconv.FormatInt(userID, 10))
	members, err := r.client.SMembers(ctx, userKey).Result()
	if err != nil {
		return err
	}
	pipe := r.client.TxPipeline()
	for _, member := range members {
		if member != keep {
			pipe.Del(ctx, r.key("session", member))
			pipe.SRem(ctx, userKey, member)
		}
	}
	if keep == "" {
		pipe.Del(ctx, userKey)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *Redis) Allow(ctx context.Context, bucket string, limit int, window time.Duration) (bool, error) {
	key := r.key("rate", bucket)
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := r.client.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= int64(limit), nil
}

func (r *Redis) key(parts ...string) string {
	result := r.prefix
	for _, part := range parts {
		result += ":" + part
	}
	return result
}

var ErrSessionNotFound = errors.New("session not found")
