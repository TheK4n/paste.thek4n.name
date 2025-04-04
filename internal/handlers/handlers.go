package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const ONE_MEBIBYTE = 1048576

const SECONDS_IN_MONTH = time.Second * 60 * 60 * 24 * 30
const DEFAULT_TTL_SECONDS = SECONDS_IN_MONTH
const MIN_TTL = time.Second * 60

const SECONDS_IN_YEAR = time.Second * 60 * 60 * 24 * 30 * 12
const MAX_TTL = SECONDS_IN_YEAR

const HEALTHCHECK_TIMEOUT = time.Second * 3

const REDIRECT_BODY = `<html><head>
<title>303 See Other</title>
</head><body>
<h1>See Other</h1>
<p>The document has moved <a href="%s">here</a>.</p>
</body></html>`

type Application struct {
	Version string
	Db      storage.KeysDB
}

type HealthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

// Checks database availability and returns version
func (app *Application) Healthcheck(w http.ResponseWriter, r *http.Request) {
	availability := true
	msg := "ok"

	ctx, cancel := context.WithTimeout(context.Background(), HEALTHCHECK_TIMEOUT)
	defer cancel()

	if !app.Db.Ping(ctx) {
		availability = false
		msg = "Error connection to database"
	}

	resp := &HealthcheckResponse{
		Version:      app.Version,
		Availability: availability,
		Msg:          msg,
	}

	answer, err := json.Marshal(resp)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte(answer))

	if err != nil {
		log.Printf("Error on answer healthcheck: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (app *Application) Cache(w http.ResponseWriter, r *http.Request) {
	ttl, errGetTTL := getTTL(r)
	if errGetTTL != nil {
		log.Printf(
			"Error on parsing ttl: %s. Response to client %s with code %d",
			errGetTTL.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	disposable, errGetDisposable := getDisposable(r)
	if errGetDisposable != nil {
		log.Printf(
			"Error on validating disposable argument: %s. Response to client %s with code %d",
			errGetDisposable.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	isUrl, errGetURL := getURL(r)
	if errGetURL != nil {
		log.Printf(
			"Error on validating url argument: %s. Response to client %s with code %d",
			errGetURL.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if r.ContentLength > ONE_MEBIBYTE {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	body, readBodyErr := io.ReadAll(r.Body)

	if readBodyErr != io.EOF && readBodyErr != nil {
		log.Printf(
			"Error on reading body: %s. Response to client %s with code %d",
			readBodyErr.Error(),
			r.RemoteAddr,
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isUrl {
		body = []byte(strings.TrimSpace(string(body)))
		if !validateUrl(string(body)) {
			log.Printf(
				"Error on validating url body. Response to client %s with code %d",
				r.RemoteAddr,
				http.StatusUnprocessableEntity,
			)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}

	var record storage.Record
	record.Body = body
	record.Disposable = disposable != 0
	record.Countdown = disposable
	record.URL = isUrl
	record.Clicks = 0

	key, err := keys.Cache(app.Db, 4*time.Second, ttl, record)
	if err != nil {
		log.Printf("Error on setting key: %s, suffered user %s", err, r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	scheme := detectScheme(r)

	_, answerErr := fmt.Fprintf(w, "%s%s/%s/", scheme, r.Host, key)
	if answerErr != nil {
		log.Printf("Error on answer: %s", answerErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Set key '%s' size=%d ttl=%s countdown=%d url=%t", key, len(body), ttl, disposable, isUrl)
}

func (app *Application) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	record, getKeyErr := keys.Get(app.Db, key, 4*time.Second)

	if getKeyErr != nil {
		if getKeyErr == storage.ErrKeyNotFound || errors.Unwrap(getKeyErr) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
			return
		} else {
			log.Printf(
				"Error on getting key: %s, suffered user %s",
				getKeyErr.Error(),
				r.RemoteAddr,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if record.URL {
		answer := make([]byte, 0)
		answer = fmt.Appendf(answer, REDIRECT_BODY, string(record.Body))
		w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(answer))
		http.Redirect(w, r, strings.TrimSpace(string(record.Body)), http.StatusSeeOther)
		_, writeErr := w.Write(answer)
		if writeErr != nil {
			log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Printf("Redirect url by key '%s' from %s", key, r.RemoteAddr)
		return
	}

	w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(record.Body))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(record.Body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get content by key '%s' from %s", key, r.RemoteAddr)
}

func (app *Application) GetClicks(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	clicks, err := keys.GetClicks(app.Db, key, 4*time.Second)
	if err != nil {
		if err == storage.ErrKeyNotFound || errors.Unwrap(err) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
			return
		} else {
			log.Printf(
				"Error on getting key: %s, suffered user %s",
				err.Error(),
				r.RemoteAddr,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	body := []byte(strconv.Itoa(clicks))
	w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(body))
	w.WriteHeader(http.StatusOK)

	_, writeErr := w.Write(body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get clicks by key '%s' from %s", key, r.RemoteAddr)
}

func getTTL(r *http.Request) (time.Duration, error) {
	ttlQuery := r.URL.Query().Get("ttl")

	if ttlQuery == "" {
		return DEFAULT_TTL_SECONDS, nil
	}

	ttl, err := time.ParseDuration(ttlQuery)

	if err != nil {
		return 0, err
	}

	if ttl < MIN_TTL {
		return 0, fmt.Errorf("TTL can`t be less then %s", MIN_TTL)
	}

	if ttl > MAX_TTL {
		return 0, fmt.Errorf("TTL can`t be more then %s", MAX_TTL)
	}

	return ttl, nil
}

func getDisposable(r *http.Request) (int, error) {
	disposableQuery := r.URL.Query().Get("disposable")

	if disposableQuery == "" {
		return 0, nil
	}

	disposable, err := strconv.Atoi(disposableQuery)

	if err != nil {
		return 0, err
	}

	if disposable < 0 {
		return 0, fmt.Errorf("Disposable argument can`t be less then zero")
	}

	if disposable > 255 {
		return 0, fmt.Errorf("Disposable argument can`t be more then 255")
	}

	return disposable, nil
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

func detectScheme(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	} else {
		return "https://"
	}
}

func validateUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
