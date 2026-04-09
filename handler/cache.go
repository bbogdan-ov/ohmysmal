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

		userId, ok := session.Values[USER_ID_SESSION_KEY].(uint)
		if !ok {
			delete(session.Values, USER_ID_SESSION_KEY)
			handler.ServeHTTP(w, r)
			return
		}

		_, found := h.cache.Get(fmtUserCacheKey(userId))
		if !found || methodUpdatesCache(r.Method) {
			h.updateUserCache(w, r, session)
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
	userId, _ := session.Values[USER_ID_SESSION_KEY]
	if userId == nil {
		return ErrUserNotAuth
	}

	id, ok := userId.(uint)
	if !ok {
		log.Printf("ERROR: User id in the session has an invalid type (%s)", reflect.TypeOf(userId))
		return ErrUserNotAuth
	}

	err = h.requestAndCacheUser(id)
	if err == server.ErrUserNotFound {
		delete(session.Values, USER_ID_SESSION_KEY)
		_ = session.Save(r, w)
		// fallthough
	} else if err != nil {
		log.Printf("ERROR: Failed to cache authorized user info: %s", err)
		return err
	}

	return nil
}

func (h Handler) requestAndCacheUser(id uint) (err error) {
	user, err := server.RequestUserById(h.db, id)
	if err != nil {
		return err
	}

	h.cache.Set(fmtUserCacheKey(id), user, time.Minute)
	return nil
}

func fmtUserCacheKey(id uint) string {
	return fmt.Sprintf("user-%d", id)
}
