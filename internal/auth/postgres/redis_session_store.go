package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

type RedisSessionStore struct {
	client *redis.Client
}

func NewRedisSessionStore(client *redis.Client) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (s *RedisSessionStore) Save(tokenHash string, user domain.User, expiresAt time.Time) error {
	b, err := json.Marshal(user)
	if err != nil {
		return err
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return s.client.Set(context.Background(), key(tokenHash), b, ttl).Err()
}

func (s *RedisSessionStore) Get(tokenHash string) (*domain.User, error) {
	raw, err := s.client.Get(context.Background(), key(tokenHash)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	var u domain.User
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *RedisSessionStore) Delete(tokenHash string) error {
	return s.client.Del(context.Background(), key(tokenHash)).Err()
}

func key(tokenHash string) string {
	return "session:" + tokenHash
}
