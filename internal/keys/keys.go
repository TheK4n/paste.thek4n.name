package keys

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

var (
	ErrKeyAlreadyTaken = errors.New("key already taken")
)

// Get key from db using timeout for context
func Get(db storage.KeysDB, key string, timeout time.Duration) (storage.KeyRecordAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return db.Get(ctx, key)
}

// Get clicks for key from db using timeout for context
func GetClicks(db storage.KeysDB, key string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return db.GetClicks(ctx, key)
}

// Cache record using timeout for context
// requestedKey - you can request custom key, if it exists func returns error ErrKeyAlreadyTaken
// if requestedKey is empty, func generates new unique key with length
// ttl - time to live for key, after this time, key will automaticly deletes
func Cache(db storage.KeysDB, timeout time.Duration, requestedKey string, ttl time.Duration, length int, record storage.KeyRecord) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var uniqKey string
	var err error
	if requestedKey != "" {
		exists, err := db.Exists(context.Background(), requestedKey)
		if err != nil {
			return "", fmt.Errorf("error on checking key: %w", err)
		}

		if !exists {
			uniqKey = requestedKey
		} else {
			return "", ErrKeyAlreadyTaken
		}
	} else {
		uniqKey, err = generateUniqKey(ctx, db, length, config.MAX_KEY_LENGTH, config.ATTEMPTS_TO_INCREASE_KEY_MIN_LENGHT, config.CHARSET)
		if err != nil {
			return "", fmt.Errorf("error on generating unique key: %w", err)
		}
	}

	err = db.Set(ctx, uniqKey, ttl, record)
	if err != nil {
		return "", fmt.Errorf("error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

// Generates unique key with minimum lenght of minLength using charset
// increases minLength if was attemptsToIncreaseMinLength attempts generate unique key
// Return error if database error or context done or maxLength reached
func generateUniqKey(
	ctx context.Context, db storage.KeysDB,
	minLength int, maxLength int,
	attemptsToIncreaseMinLength int,
	charset string,
) (string, error) {
	key := generateKey(minLength, charset)
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return "", err
	}
	currentAttemptsCountdown := attemptsToIncreaseMinLength

	for exists {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout")
		default:
		}

		key = generateKey(minLength, charset)
		exists, err = db.Exists(ctx, key)
		if err != nil {
			return "", err
		}
		currentAttemptsCountdown--

		if currentAttemptsCountdown < 1 {
			minLength++
			currentAttemptsCountdown = attemptsToIncreaseMinLength
		}

		if minLength > maxLength {
			return "", fmt.Errorf("max key length reached")
		}
	}

	return key, nil
}

// Generate new random key with specified length using charset
func generateKey(length int, charset string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	charsetLen := len(charset)

	for i := range length {
		result[i] = charset[r.Intn(charsetLen)]
	}

	return string(result)
}
