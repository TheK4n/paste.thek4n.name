// Package handlers provides handlers
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const redirectBody = `<html><head>
<title>303 See Other</title>
</head><body>
<h1>See Other</h1>
<p>The document has moved <a href="%s">here</a>.</p>
</body></html>`

// Application struct contains databases connections.
type Application struct {
	Version   string
	DB        storage.KeysDB
	APIKeysDB storage.APIKeysDB
	QuotaDB   storage.QuotaDB
}

type healthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

// Healthcheck checks database availability and returns version.
func (app *Application) Healthcheck(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	availability := true
	msg := "ok"

	ctx, cancel := context.WithTimeout(context.Background(), config.HealthcheckTimeout)
	defer cancel()

	if !app.DB.Ping(ctx) {
		availability = false
		msg = "Error connection to database"
	}

	resp := &healthcheckResponse{
		Version:      app.Version,
		Availability: availability,
		Msg:          msg,
	}

	answer, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error on answer healthcheck: %s, suffered user %s", err.Error(), remoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(answer)
	if err != nil {
		log.Printf("Error on answer healthcheck: %s, suffered user %s", err.Error(), remoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Cache handle request to set key.
func (app *Application) Cache(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	authorized := false
	apikey := r.URL.Query().Get("apikey")
	if apikey != "" {
		var err error
		authorized, err = app.validateApikey(apikey)
		if err != nil {
			log.Printf(
				"Error on checking apikey: %s. Response to client %s with code %d",
				err.Error(),
				remoteAddr,
				http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if !authorized {
		quotaValid, err := app.QuotaDB.IsQuotaValid(context.Background(), remoteAddr)
		if err != nil {
			log.Printf(
				"Error on checking quota: %s. Response to client %s with code %d",
				err.Error(),
				remoteAddr,
				http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !quotaValid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, "Your quota for today is exhausted.")
			return
		}

		err = app.QuotaDB.ReduceQuota(context.Background(), remoteAddr)
		if err != nil {
			log.Printf(
				"Error on reduce quota: %s. Response to client %s with code %d",
				err.Error(),
				remoteAddr,
				http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if authorized {
		log.Printf(
			"Using authorized apikey from %s",
			remoteAddr,
		)
	}

	ttl, errGetTTL := getTTL(r)
	if errGetTTL != nil {
		log.Printf(
			"Error on parsing ttl: %s. Response to client %s with code %d",
			errGetTTL.Error(),
			remoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'ttl' parameter")
		return
	}

	if ttl == time.Duration(0) {
		if !authorized {
			log.Printf(
				"Unathorized attempt to set persist key",
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	length, errGetLength := getLength(r)
	if errGetLength != nil {
		log.Printf(
			"Error on parsing length: %s. Response to client %s with code %d",
			errGetLength.Error(),
			remoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'len' parameter")
		return
	}

	if length < config.UnprivelegedMinKeyLength {
		if !authorized {
			log.Printf(
				"Unathorized attempt to set short key with length %d",
				length,
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	disposable, errGetDisposable := getDisposable(r)
	if errGetDisposable != nil {
		log.Printf(
			"Error on validating disposable argument: %s. Response to client %s with code %d",
			errGetDisposable.Error(),
			remoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'disposable' parameter")
		return
	}

	isURL, errGetURL := getURL(r)
	if errGetURL != nil {
		log.Printf(
			"Error on validating url argument: %s. Response to client %s with code %d",
			errGetURL.Error(),
			remoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'url' parameter")
		return
	}

	requestedKey, errGetRequestedKey := getRequestedKey(r)
	if errGetRequestedKey != nil {
		log.Printf(
			"Error on validating 'key' argument: %s. Response to client %s with code %d",
			errGetRequestedKey.Error(),
			remoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, errGetRequestedKey.Error())
		return
	}

	if !authorized {
		if requestedKey != "" {
			log.Printf(
				"Unathorized attempt to set custom key",
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if !authorized {
		if r.ContentLength > config.UnprevelegedMaxBodySize {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = fmt.Fprintf(w, "Body too large. Maximum is %d bytes", config.UnprevelegedMaxBodySize)
			return
		}
	}

	if r.ContentLength > config.PrevelegedMaxBodySize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = fmt.Fprintf(w, "Body too large. Maximum is %d bytes", config.PrevelegedMaxBodySize)
		return
	}

	body, readBodyErr := io.ReadAll(r.Body)

	if readBodyErr != io.EOF && readBodyErr != nil {
		log.Printf(
			"Error on reading body: %s. Response to client %s with code %d",
			readBodyErr.Error(),
			remoteAddr,
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isURL {
		body = []byte(strings.TrimSpace(string(body)))
		if !validateURL(string(body)) {
			log.Printf(
				"Error on validating url body. Response to client %s with code %d",
				remoteAddr,
				http.StatusUnprocessableEntity,
			)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = fmt.Fprint(w, "Invalid 'url' parameter")
			return
		}
	}

	var record storage.KeyRecord
	record.Body = body
	record.Disposable = disposable != 0
	record.Countdown = disposable
	record.URL = isURL
	record.Clicks = 0

	var key string
	var err error

	if requestedKey == "" {
		key, err = keys.CacheGeneratedKey(app.DB, 4*time.Second, ttl, length, record)
	} else {
		key, err = keys.CacheRequestedKey(app.DB, 4*time.Second, requestedKey, ttl, record)
		if err != nil {
			if errors.Is(err, keys.ErrKeyAlreadyTaken) {
				log.Printf("Try to take already taken key from %s: Error: %s", remoteAddr, err)
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprint(w, "Key already taken")
				return
			}
		}
	}

	if err != nil {
		log.Printf("Error on setting key: %s, suffered user %s", err, remoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	proto := detectProto(r)

	_, answerErr := fmt.Fprintf(w, "%s://%s/%s/", proto, r.Host, key)
	if answerErr != nil {
		log.Printf("Error on answer: %s", answerErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Set key '%s' size=%d ttl=%s countdown=%d url=%t from %s", key, len(body), ttl, disposable, isURL, remoteAddr)
}

// Get handle getting key.
func (app *Application) Get(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	key := r.PathValue("key")

	record, getKeyErr := keys.Get(app.DB, key, 4*time.Second)

	if getKeyErr != nil {
		if getKeyErr == storage.ErrKeyNotFound || errors.Unwrap(getKeyErr) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), remoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, remoteAddr)
			return
		}
		log.Printf(
			"Error on getting key: %s, suffered user %s",
			getKeyErr.Error(),
			remoteAddr,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if record.URL {
		answer := make([]byte, 0)
		answer = fmt.Appendf(answer, redirectBody, string(record.Body))
		w.Header().Set("content-type", http.DetectContentType(answer))
		http.Redirect(w, r, strings.TrimSpace(string(record.Body)), http.StatusSeeOther)
		_, writeErr := w.Write(answer)
		if writeErr != nil {
			log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), remoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Printf("Redirect url by key '%s' from %s", key, remoteAddr)
		return
	}

	w.Header().Set("content-type", http.DetectContentType(record.Body))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(record.Body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), remoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get content by key '%s' from %s", key, remoteAddr)
}

// GetClicks handle getting clicks for key request.
func (app *Application) GetClicks(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	key := r.PathValue("key")

	clicks, err := keys.GetClicks(app.DB, key, 4*time.Second)
	if err != nil {
		if err == storage.ErrKeyNotFound || errors.Unwrap(err) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := fmt.Fprint(w, "404 Not Found")
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), remoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, remoteAddr)
			return
		}
		log.Printf(
			"Error on getting key: %s, suffered user %s",
			err.Error(),
			remoteAddr,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body := []byte(strconv.Itoa(clicks))
	w.Header().Set("content-type", http.DetectContentType(body))
	w.WriteHeader(http.StatusOK)

	_, writeErr := w.Write(body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), remoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get clicks by key '%s' from %s", key, remoteAddr)
}

func getTTL(r *http.Request) (time.Duration, error) {
	ttlQuery := r.URL.Query().Get("ttl")

	if ttlQuery == "" {
		return config.DefaultTTL, nil
	}

	ttl, err := time.ParseDuration(ttlQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse duration: %w", err)
	}

	if ttl < config.MinTTL {
		return 0, fmt.Errorf("TTL can`t be less then %s", config.MinTTL)
	}

	if ttl > config.MaxTTL {
		return 0, fmt.Errorf("TTL can`t be more then %s", config.MaxTTL)
	}

	return ttl, nil
}

func getRequestedKey(r *http.Request) (string, error) {
	requestedKey := r.URL.Query().Get("key")

	if requestedKey == "" {
		return "", nil
	}

	if len(requestedKey) > config.MaxKeyLength {
		return "", fmt.Errorf("requested key length more then max")
	}

	if len(requestedKey) < config.PrivelegedMinKeyLength {
		return "", fmt.Errorf("requested key length less then min")
	}

	for _, char := range requestedKey {
		if !strings.ContainsRune(config.Charset, char) {
			return "", fmt.Errorf("requested key contains illegal char")
		}
	}

	return requestedKey, nil
}

func getDisposable(r *http.Request) (int, error) {
	disposableQuery := r.URL.Query().Get("disposable")

	if disposableQuery == "" {
		return 0, nil
	}

	disposable, err := strconv.Atoi(disposableQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse disposable: %w", err)
	}

	if disposable < 0 {
		return 0, fmt.Errorf("disposable argument can`t be less then zero")
	}

	if disposable > 255 {
		return 0, fmt.Errorf("disposable argument can`t be more then 255")
	}

	return disposable, nil
}

func getLength(r *http.Request) (int, error) {
	lengthQuery := r.URL.Query().Get("len")

	if lengthQuery == "" {
		return config.DefaultKeyLength, nil
	}

	length, err := strconv.Atoi(lengthQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse length: %w", err)
	}

	if length < config.PrivelegedMinKeyLength {
		return 0, fmt.Errorf("length can`t be less then %d", config.PrivelegedMinKeyLength)
	}

	if length > config.MaxKeyLength {
		return 0, fmt.Errorf("length can`t be more then %d", config.MaxKeyLength)
	}

	return length, nil
}

func getURL(r *http.Request) (bool, error) {
	URLQuery := r.URL.Query().Get("url")

	if URLQuery == "" {
		return false, nil
	}

	if URLQuery == "true" {
		return true, nil
	}

	if URLQuery == "false" {
		return false, nil
	}

	return false, fmt.Errorf("URL argument can be only 'true' or 'false'")
}

func detectProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}

	proto := r.Header.Get("X-Forwarded-Proto")
	if proto != "" {
		return proto
	}

	return "http"
}

func validateURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (app *Application) validateApikey(str string) (bool, error) {
	record, err := app.APIKeysDB.Get(context.Background(), str)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fail to get api key: %w", err)
	}

	return record.Valid, nil
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
