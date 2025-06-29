package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type APIKeyRecord struct {
	ID    string `redis:"id"`
	Valid bool   `redis:"valid"`
}

type APIKeysDB struct {
	Client *redis.Client
}

func (db *APIKeysDB) Get(ctx context.Context, key string) (APIKeyRecord, error) {
	var record APIKeyRecord
	exists, err := db.exists(ctx, key)
	if err != nil {
		return record, err
	}

	if !exists {
		return record, nil
	}

	err = db.Client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return record, err
	}

	return record, nil
}

func (db *APIKeysDB) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

func (db *APIKeysDB) Ping(ctx context.Context) bool {
	return db.Client.Ping(ctx).Err() == nil
}

func InitAPIKeysStorageDB(dbHost string, dbPort int) (*APIKeysDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           1,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Printf("Connected to database 1 (apikeys) on %s:%d\n", dbHost, dbPort)

	return &APIKeysDB{Client: client}, nil
}
