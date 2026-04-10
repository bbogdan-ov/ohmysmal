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
)

const (
	SESSION_NAME        = "ohmysmal"
	USER_ID_SESSION_KEY = "id"
)

var (
	ErrUserAlreadyAuth = BadRequestError{"user is already authorized"}
	ErrUserNotAuth     = BadRequestError{"user is not authorized"}
	ErrNicknameInvalid = BadRequestError{"nickname is invalid"}
	ErrNicknameTooLong = BadRequestError{"nickname is too long"}
)

// NOTE: any other `error` types are "internal server errors".

// This error returns whenever a user did something wrong, for example tried to
// log in with an incorrect nickname or password.
type UserError struct {
	Message string
}

func (e UserError) Error() string {
	return e.Message
}

// An invalid request was received.
type BadRequestError struct {
	Message string
}

func (e BadRequestError) Error() string {
	return e.Message
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
		return 0, BadRequestError{fmt.Sprintf(`no "%s" is provided in the URL`, name)}
	}

	num, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, BadRequestError{fmt.Sprintf(`param "%s" is not a 'uint': %s`, name, err)}
	}

	return uint(num), nil
}

// Parse a path value of type `uuid.UUID` from a request.
func UUIDPathValue(r *http.Request, name string) (uuid.UUID, error) {
	str := r.PathValue(name)
	if str == "" {
		return uuid.UUID{}, BadRequestError{fmt.Sprintf(`no "%s" is provided in the URL`, name)}
	}

	id, err := uuid.Parse(str)
	if err != nil {
		msg := fmt.Sprintf(`param "%s" is an invalid UUID: %s`, name, err)
		return uuid.UUID{}, BadRequestError{msg}
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
	if errors.As(err, &UserError{}) {
		// Making errors is ok, don't worry <3
		http.Error(w, err.Error(), http.StatusOK)
	} else if errors.As(err, &BadRequestError{}) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		log.Printf("ERROR: Server error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Write an error page on server error, otherwise write an error message into
// the response body.
func ErrorPage(w http.ResponseWriter, r *http.Request, err error) {
	if errors.As(err, &UserError{}) {
		// Making errors is ok, don't worry <3
		http.Error(w, err.Error(), http.StatusOK)
	} else if errors.As(err, &BadRequestError{}) {
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
	w.Header().Add("Location", location)
	w.WriteHeader(http.StatusMovedPermanently)
}
