package main

import (
	"fmt"
	"os"
	"log"
	"net"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/robfig/go-cache"

	"ohmysmal/handler"
	"ohmysmal/server"
)

func main() {
	username := envOr("OHMYSMAL_USERNAME", "root")
	password := envOr("OHMYSMAL_PASSWORD", "root")
	secret := envOr("OHMYSMAL_SECRET", "secret")
	port := envOr("OHMYSMAL_PORT", "8080")
	cert := envOr("OHMYSMAL_CERT", "")
	key := envOr("OHMYSMAL_KEY", "")

	log.Printf("OHMYSMAL_USERNAME = (%t)", username != "")
	log.Printf("OHMYSMAL_PASSWORD = (%t)", password != "")
	log.Printf("OHMYSMAL_SECRET = (%t)", secret != "")
	log.Printf("OHMYSMAL_PORT = %q", port)
	log.Printf("OHMYSMAL_CERT = %q", cert)
	log.Printf("OHMYSMAL_KEY = %q", key)

	net.DefaultResolver = &net.Resolver{PreferGo: false}

	// Setup cache.
	cache := cache.New(time.Second, time.Second*5)

	// Setup session storage.
	store := sessions.NewCookieStore([]byte(secret))

	// Setup database.
	db := server.ConnectDatabase(username, password)
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
	http.HandleFunc("/api/comment/{id}", h.UserCacheMiddleware(h.HandleApiComment))

	static := http.Dir("static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(static)))

	addr := fmt.Sprintf(":%s", port)

	// Starting the server.
	log.Printf("Listening on %s", addr)

	var err error
	if cert != "" || key != "" {
		err = http.ListenAndServeTLS(addr, cert, key, nil)
	} else {
		err = http.ListenAndServe(addr, nil)
	}

	if err != nil {
		log.Fatalf("Failed to run the server: %s", err)
	}
}

func envOr(name string, default_ string) string {
	value := os.Getenv(name)
	if value == "" {
		value = default_
	}
	return value
}
