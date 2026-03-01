package handler

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/go-cache"

	"ohmysmal/database"
	"ohmysmal/view"
)

const (
	USER_ID_SESSION_KEY = "id"
	USER_CACHE_KEY      = "user"
	MAX_PASSWORD_LEN    = 64 // max length of a password in runes (utf8 characters)
	MAX_NICKNAME_LEN    = 32
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

type Handler struct {
	db    *sql.DB
	cache *cache.Cache
}

func New(db *sql.DB, store *cache.Cache) Handler {
	return Handler{db, store}
}

// --------------------
// Pages
// --------------------

func (h Handler) HandleHome(c *gin.Context) {
	user, ok := h.authorizedUser()

	snippets := make([]database.Snippet, 0, 20)
	_, err := database.RequestSnippets(h.db, &snippets, user.Id, ok)
	if err != nil {
		log.Printf("ERROR: Failed to request the list of snippets: %s", err)
		// fallthough
	}

	c.HTML(http.StatusOK, "", view.Home(user, ok, snippets))
}
func (h Handler) HandleHey(c *gin.Context) {
	c.String(http.StatusOK, "hello")
}

// --------------------
// Utils
// --------------------

// Write an error message into the response body.
func writeError(c *gin.Context, err error) {
	if errors.As(err, &UserError{}) {
		// Making errors is ok, don't worry <3
		c.String(http.StatusOK, err.Error())
	} else if errors.As(err, &BadRequestError{}) {
		c.String(http.StatusBadRequest, err.Error())
	} else {
		log.Printf("ERROR: Server error: %s", err)
		c.String(http.StatusInternalServerError, err.Error())
	}
}

// Write the "HX-writeRedirect" (HTMX redirect) header the response body. HTMX
// doesn't know how to work with normal "Location" header.
func writeRedirect(c *gin.Context, location string) {
	c.Header("HX-Redirect", location)
	c.Status(http.StatusMovedPermanently)
}
