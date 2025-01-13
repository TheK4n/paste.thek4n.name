package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thek4n/paste.thek4n.name/cmd/storage"
)

type Users struct {
	Db *redis.Client
}

func main() {
	cfg := storage.Config{
		Addr:        "localhost:6379",
		Password:    "",
		User:        "",
		DB:          0,
		MaxRetries:  5,
		DialTimeout: 10 * time.Second,
		Timeout:     5 * time.Second,
	}

	db, err := storage.NewClient(context.Background(), cfg)
	if err != nil {
		panic(err)
	}

	users := Users{Db: db}

	mux := http.NewServeMux()

	mux.HandleFunc("/", users.saveHandler)

	log.Print("Server started...")

	http.ListenAndServe(":8080", mux)
}

func (users *Users) saveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf(
			"Error on reading body: %s. Response to client with code %d",
			err.Error(),
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	uniqKey, err := generateUniqKey(users.Db)
	if err != nil {
		log.Printf("Error on generating unique key: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = users.Db.Set(context.Background(), uniqKey, body, 0).Err()
	if err != nil {
		log.Printf(
			"Error on setting key: %s",
			err.Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Set body with key '%s'", uniqKey)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scheme := detectScheme(r)

	_, err = fmt.Fprintf(w, "%s%s/%s", scheme, r.Host, uniqKey)
	if err != nil {
		log.Printf("Error on answer: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// func (users *Users) getHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		w.WriteHeader(http.StatusMethodNotAllowed)
// 		return
// 	}
//
//
// 	keysNumber, err := users.Db.Exists(context.Background(), key).Uint64()
// }
//

func detectScheme(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	} else {
		return "https://"
	}
}

func generateUniqKey(db *redis.Client) (string, error) {
	length := 14

	key := generateKey(length)

	keysNumber, err := db.Exists(context.Background(), key).Uint64()
	if err != nil {
		return "", err
	}

	if keysNumber > 0 {
		return generateUniqKey(db)
	}

	return key, nil
}

func generateKey(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[r.Intn(len(chars))]
	}

	return string(result)
}