package handler

import (
	"fmt"
	"log"
	"net/http"
	"ohmysmal/server"
	"reflect"
	"strings"
	"time"

	"github.com/gorilla/sessions"
)

// Caches the authorized user info on each request to reduce number of requests to the database.
func (h Handler) UserCacheMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ignoreUrl(r.URL.Path) {
			return
		}

		session := h.DefaultSession(r)

		sessionUserId := session.Values[USER_ID_SESSION_KEY]
		if sessionUserId == nil {
			handler.ServeHTTP(w, r)
			return
		}

		userId, ok := sessionUserId.(uint)
		if !ok {
			log.Printf("CACHE: WARNING: User ID in the session is invalid, destroying the session")
			delete(session.Values, USER_ID_SESSION_KEY)
			_ = session.Save(r, w)

			handler.ServeHTTP(w, r)
			return
		}

		_, found := h.cache.Get(fmtUserCacheKey(userId))
		if !found || methodUpdatesCache(r.Method) {
			_ = h.updateUserCache(w, r, session)
		}

		handler.ServeHTTP(w, r)
	}
}

func ignoreUrl(url string) bool {
	return strings.HasPrefix(url, "/static") ||
		url == "/favicon.ico"
}

func methodUpdatesCache(method string) bool {
	// Any "modifying" method should update the cache to get an up-to-date user info.
	return method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH"
}

// Updates the currently authorized user cache. Cache will be updated even if
// it is already present or it is not expired yet.
func (h Handler) updateUserCache(w http.ResponseWriter, r *http.Request, session *sessions.Session) (err error) {
	log.Printf("CACHE: INFO: Updating auth user cache...")

	userId, _ := session.Values[USER_ID_SESSION_KEY]
	if userId == nil {
		log.Printf("CACHE: WARNING: User is not authorized, nothing to update")
		return ErrUserNotAuth
	}

	id, ok := userId.(uint)
	if !ok {
		log.Printf("CACHE: ERROR: User id in the session has an invalid type (%s)", reflect.TypeOf(userId))
		return ErrUserNotAuth
	}

	err = h.requestAndCacheUser(r, id)
	if err == server.ErrUserNotFound {
		log.Printf("CACHE: WARNING: Authorized user was not found in the database when trying to update cache, destroying user's session")

		delete(session.Values, USER_ID_SESSION_KEY)
		_ = session.Save(r, w)
		// fallthough
	} else if err != nil {
		log.Printf("CACHE: ERROR: Failed to cache authorized user info: %s", err)
		return err
	}

	return nil
}

func (h Handler) requestAndCacheUser(r *http.Request, id uint) (err error) {
	user, err := server.RequestUserById(r, h.db, id)
	if err != nil {
		return err
	}

	log.Printf("CACHE: INFO: Updated auth user cache: %d, %s", user.Id, user.Nickname)
	h.cache.Set(fmtUserCacheKey(id), user, time.Minute)
	return nil
}

func fmtUserCacheKey(id uint) string {
	return fmt.Sprintf("user-%d", id)
}
