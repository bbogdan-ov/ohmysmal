package main

import (
	"log"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/go-cache"

	"ohmysmal/database"
	"ohmysmal/handler"
)

func main() {
	router := gin.Default()
	router.SetTrustedProxies(nil) // do not trust anyone.
	// Setup the default HTML renderer.
	htmlRenderer := router.HTMLRender
	router.HTMLRender = &HTMLTemplRenderer{FallbackHtmlRenderer: htmlRenderer}

	// Setup cache.
	store := cache.New(time.Second, time.Second*5)

	// Setup database.
	db := database.Connect()
	defer db.Close()

	h := handler.New(db, store)

	// Setup session storage.
	cookieStore := cookie.NewStore([]byte("secret")) // TODO: pass a secret key through env vars.
	router.Use(sessions.Sessions("ohmysmal", cookieStore))
	router.Use(h.UserCacheMiddleware)

	// Handle routes.
	router.GET("/", h.HandleHome)
	router.GET("/hey", h.HandleHey)
	router.POST("/api/login", h.HandleApiLogin)
	router.POST("/api/logout", h.HandleApiLogout)
	router.POST("/api/register", h.HandleApiRegister)
	router.POST("/api/snippet", h.HandleApiSnippet)
	router.POST("/api/flower/:id", h.HandleApiFlower)

	router.Static("/static", "./static")

	// Starting the server.
	err := router.Run()
	if err != nil {
		log.Fatalf("Failed to run the server: %s", err)
	}
}
