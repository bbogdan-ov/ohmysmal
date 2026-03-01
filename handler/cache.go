package handler

import (
	"log"
	"ohmysmal/database"
	"reflect"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Caches the authorized user info on each request to reduce number of requests to the database.
func (h Handler) UserCacheMiddleware(c *gin.Context) {
	if ignoreUrl(c.Request.URL.Path) {
		return
	}

	session := sessions.Default(c)
	h.updateUserCache(session)
}

func ignoreUrl(url string) bool {
	return strings.HasPrefix(url, "/static") ||
		url == "/favicon.ico"
}

func (h Handler) updateUserCache(session sessions.Session) (err error) {
	userId := session.Get(USER_ID_SESSION_KEY)
	if userId == nil {
		return ErrUserNotAuth
	}

	id, ok := userId.(uint)
	if !ok {
		log.Printf("ERROR: User id in the session has an invalid type (%s)", reflect.TypeOf(userId))
		return ErrUserNotAuth
	}

	err = h.requestAndCacheUser(id)
	if err == database.ErrUserNotFound {
		session.Delete(USER_ID_SESSION_KEY)
		session.Save()
		// fallthough
	} else if err != nil {
		log.Printf("ERROR: Failed to cache authorized user info: %s", err)
		return err
	}

	return nil
}

func (h Handler) requestAndCacheUser(id uint) (err error) {
	_, found := h.cache.Get(USER_CACHE_KEY)
	if found {
		// User is cached, do nothing.
		return nil
	}

	user, err := database.RequestUserById(h.db, id)
	if err != nil {
		return err
	}

	h.cache.Set(USER_CACHE_KEY, user, time.Minute)
	return nil
}
