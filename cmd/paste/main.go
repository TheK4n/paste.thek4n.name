package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"

	flags "github.com/jessevdk/go-flags"
)

type Options struct {
	Port   int    `short:"p" long:"port" description:"Port to listen"`
	Host   string `long:"host" description:"Host to listen"`
	Ping   bool   `long:"ping" description:"Enable ping handler"`
	DBPort int    `long:"dbport" description:"Database port"`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(2)
	}

	log.Println("Connecting to database...")

	redisHost := os.Getenv("REDIS_HOST")
	db, err := storage.InitStorageDB(redisHost, opts.DBPort)
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	handlers := handlers.Handlers{Db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/", handlers.Get)
	mux.HandleFunc("POST /", handlers.Cache)
	if opts.Ping {
		mux.HandleFunc("GET /ping/", handlers.Pingpong)
	}

	hostport := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	log.Printf("Server started on %s ...", hostport)
	log.Fatal(http.ListenAndServe(hostport, mux))
}
