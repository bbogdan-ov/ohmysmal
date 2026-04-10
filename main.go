package main

import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/robfig/go-cache"

	"ohmysmal/handler"
	"ohmysmal/server"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	net.DefaultResolver = &net.Resolver{PreferGo: false}

	// Setup cache.
	cache := cache.New(time.Second, time.Second*5)

	// Setup session storage.
	store := sessions.NewCookieStore([]byte("secret")) // TODO: pass a secret key through env vars.

	// Setup database.
	db := server.ConnectDatabase()
	defer db.Close()

	h := handler.New(db, cache, store)

	// Handle routes.
	http.HandleFunc("/", h.UserCacheMiddleware(h.HandleHome))
	http.HandleFunc("/editor", h.UserCacheMiddleware(h.HandleEditor))
	http.HandleFunc("/snippet", h.UserCacheMiddleware(h.HandleSnippet))
	http.HandleFunc("/hey", h.UserCacheMiddleware(h.HandleHey))

	http.HandleFunc("/api/login", h.UserCacheMiddleware(h.HandleApiLogin))
	http.HandleFunc("/api/logout", h.UserCacheMiddleware(h.HandleApiLogout))
	http.HandleFunc("/api/register", h.UserCacheMiddleware(h.HandleApiRegister))
	http.HandleFunc("/api/snippet", h.UserCacheMiddleware(h.HandleApiSnippet))
	http.HandleFunc("/api/flower/{snippet_id}", h.UserCacheMiddleware(h.HandleApiFlower))
	http.HandleFunc("/api/comment/{snippet_id}", h.UserCacheMiddleware(h.HandleApiComment))

	static := http.Dir("static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(static)))

	// Starting the server.
	log.Printf("Listening on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Failed to run the server: %s", err)
	}
}
