package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nos "github.com/dextryz/nostr"
	"github.com/nbd-wtf/go-nostr"
)

var (
	ADDR = "127.0.0.1"
	PORT = "8080"
)

func main() {

	log.Println("starting...")

	ctx := context.Background()

	// Elasticsearch store relay
	relay, err := nostr.RelayConnect(ctx, "ws://localhost:3334")
	if err != nil {
		panic(err)
	}

	// Slice store relay
	cache, err := nostr.RelayConnect(ctx, "ws://localhost:3335")
	if err != nil {
		panic(err)
	}

	path, ok := os.LookupEnv("NOSTR")
	if !ok {
		log.Fatalln("NOSTR env var not set")
	}

	cfg, err := nos.LoadConfig(path)
	if err != nil {
		panic(err)
	}

	h := Handler{
		cfg:   cfg,
		relay: relay,
		cache: cache,
	}

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	fs = http.FileServer(http.Dir("./fonts"))
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", fs))

	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("GET /articles", h.Articles)
	mux.HandleFunc("GET /validate", h.Validate)
	mux.HandleFunc("GET /search", h.Search)
	mux.HandleFunc("GET /articles/{naddr}", h.Article)

	s := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", ADDR, PORT),
		Handler: mux,
	}

	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		err := s.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-stop

	// Create a context with a timeout for the server's shutdown process
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = h.Close()
	if err != nil {
		log.Fatalf("closing subscriptions failed:%+v", err)
	}

	err = s.Shutdown(ctx)
	if err != nil {
		log.Fatalf("server shutdown failed:%+v", err)
	}

	log.Println("server gracefully stopped")
}
