package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"ohmysmal/view"
	"ohmysmal/server"
)

const (
	SESSION_NAME        = "ohmysmal"
	USER_ID_SESSION_KEY = "id"
)

func (h Handler) authorizedUserId(session *sessions.Session) (id uint, found bool) {
	value := session.Values[USER_ID_SESSION_KEY]
	if value == nil {
		return 0, false
	}

	userId, ok := value.(uint)
	if !ok {
		log.Printf("USERS: ERROR: User ID in the session is not an `uint`")
		delete(session.Values, USER_ID_SESSION_KEY)
		return 0, false
	}

	return userId, true
}

func (h Handler) authorizedUserOrFalse(session *sessions.Session) (user server.User, found bool) {
	id, found := h.authorizedUserId(session)
	if !found {
		return server.User{}, false
	}

	value, _ := h.cache.Get(fmtUserCacheKey(id))
	if value == nil {
		return server.User{}, false
	}

	user, found = value.(server.User)
	return user, found
}

// Returns the authorized user. The info is stored in the cache,
// it may be outdated but that's fine i guess.
func (h Handler) authorizedUser(session *sessions.Session) (user server.User, err error) {
	user, found := h.authorizedUserOrFalse(session)
	if found {
		return user, nil
	} else {
		return server.User{}, server.BadRequestError{"user is not authorized"}
	}
}

func rememberUser(w http.ResponseWriter, r *http.Request, session *sessions.Session, id uint) {
	session.Values[USER_ID_SESSION_KEY] = id
	err := session.Save(r, w)
	if err != nil {
		log.Printf("USERS: ERROR: Failed to save the session: %s", err)
	}
}

func (h Handler) DefaultSession(r *http.Request) *sessions.Session {
	s, err := h.store.Get(r, SESSION_NAME)
	if err != nil {
		log.Printf("ERROR: Failed to get a session, creating a new one: %s", err)
		// fallthough
	}
	return s
}

// Parse a path value of type `uint` from a request.
func UintPathValue(r *http.Request, name string) (uint, error) {
	str := r.PathValue(name)
	if str == "" {
		return 0, server.BadRequestError{fmt.Sprintf(`no "%s" path value is provided in the URL`, name)}
	}

	num, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, server.BadRequestError{fmt.Sprintf(`path value "%s" is not a 'uint': %s`, name, err)}
	}

	return uint(num), nil
}

// Parse a path value of type `uuid.UUID` from a request.
func UUIDPathValue(r *http.Request, name string) (uuid.UUID, error) {
	str := r.PathValue(name)
	if str == "" {
		return uuid.UUID{}, server.BadRequestError{fmt.Sprintf(`no "%s" path value is provided in the URL`, name)}
	}

	id, err := uuid.Parse(str)
	if err != nil {
		msg := fmt.Sprintf(`path value "%s" is an invalid UUID: %s`, name, err)
		return uuid.UUID{}, server.BadRequestError{msg}
	}

	return id, nil
}

func UUIDQueryGetOrFalse(r *http.Request, name string) (id uuid.UUID, found bool) {
	str := r.URL.Query().Get(name)
	if str == "" {
		return id, false
	}
	id, err := uuid.Parse(str)
	if err != nil {
		return id, false
	}

	return id, true
}
func UUIDQueryGet(r *http.Request, name string) (id uuid.UUID, err error) {
	str := r.URL.Query().Get(name)
	if str == "" {
		msg := fmt.Sprintf(`no "%s" query param is provided in the URL`, name)
		return uuid.UUID{}, server.BadRequestError{msg}
	}

	id, err = uuid.Parse(str)
	if err != nil {
		msg := fmt.Sprintf(`query param "%s" is an invalid UUID: %s`, name, err)
		return uuid.UUID{}, server.BadRequestError{msg}
	}

	return id, nil
}

// Returns whether the method in a request is equal to `method`, if not,
// returns `false` and writes `http.Error()`.
func EnsureMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// Write an error message into the response body.
func Error(w http.ResponseWriter, err error) {
	if errors.As(err, &server.BadRequestError{}) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		log.Printf("ERROR: Server error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Write an error page on server error, otherwise write an error message into
// the response body.
func ErrorPage(w http.ResponseWriter, r *http.Request, err error) {
	if errors.As(err, &server.BadRequestError{}) {
		// Making errors is ok, don't worry <3
		http.Error(w, err.Error(), http.StatusOK)
	} else if errors.As(err, &server.BadRequestError{}) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		log.Printf("ERROR: Server error, writing an error page: %s", err)

		v := templ.Handler(view.ErrorPage(http.StatusInternalServerError, err.Error()))
		v.ServeHTTP(w, r)
	}
}

// Write the "HX-Redirect" (HTMX redirect) header the response body. HTMX
// doesn't know how to work with normal "Location" header.
func Redirect(w http.ResponseWriter, location string) {
	w.Header().Add("HX-Redirect", location)
	w.WriteHeader(http.StatusMovedPermanently)
}
