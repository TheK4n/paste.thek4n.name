package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thek4n/paste.thek4n.name/internal/config"
)

// QuotaDB contains db connection.
type QuotaDB struct {
	Client *redis.Client
}

// ReduceQuota create specified key with expiration time
// and decreases countdown field for key.
func (db *QuotaDB) ReduceQuota(ctx context.Context, key string) error {
	exists, err := db.exists(ctx, key)
	if err != nil {
		return err
	}

	if !exists {
		// lua script because we need atomic execution
		script := `
			redis.call("HSET", KEYS[1], ARGV[1], ARGV[2])
			redis.call("EXPIRE", KEYS[1], ARGV[3])
			return 1
		`
		err := db.Client.Eval(ctx, script, []string{key}, "countdown", config.Quota, int(config.QuotaResetPeriod.Seconds())).Err()
		if err != nil {
			return fmt.Errorf("fail to set new quota key: %w", err)
		}
	}

	err = db.Client.HIncrBy(ctx, key, "countdown", -1).Err()
	if err != nil {
		return fmt.Errorf("fail to decrease quota: %w", err)
	}
	return nil
}

// IsQuotaValid checks is quota for specified key is not expired.
func (db *QuotaDB) IsQuotaValid(ctx context.Context, key string) (bool, error) {
	exists, err := db.exists(ctx, key)
	if err != nil {
		return false, err
	}
	if !exists {
		return true, nil
	}

	res, err := db.Client.HGet(ctx, key, "countdown").Int()
	if err != nil {
		return false, fmt.Errorf("fail to get countdown for key: %w", err)
	}
	return res > 0, nil
}

func (db *QuotaDB) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, fmt.Errorf("fail to check key existing: %w", err)
	}

	return keysNumber > 0, nil
}

// InitQuotaStorageDB returns valid QuotaDB.
func InitQuotaStorageDB(dbHost string, dbPort int) (*QuotaDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           2,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("fail to check connection: %w", err)
	}

	return &QuotaDB{Client: client}, nil
}
