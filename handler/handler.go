package handler

import (
	"fmt"
	"database/sql"
	"log"
	"net/http"

	"github.com/a-h/templ"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/robfig/go-cache"

	"ohmysmal/server"
	"ohmysmal/view"
)

type Handler struct {
	db    *sql.DB
	cache *cache.Cache
	store *sessions.CookieStore
}

func New(db *sql.DB, cache *cache.Cache, store *sessions.CookieStore) Handler {
	return Handler{db, cache, store}
}

// --------------------
// Pages.
// --------------------

func (h Handler) HandleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !EnsureMethod(w, r, "GET") {
		return
	}

	session := h.DefaultSession(r)
	user, ok := h.authorizedUser(session)

	snippets := make([]server.Snippet, 0, 20)
	err := server.RequestSnippets(h.db, &snippets, user.Id, ok)
	if err != nil {
		log.Printf("ERROR: Failed to request the list of snippets: %s", err)
		ErrorPage(w, r, err)
		return
	}

	v := templ.Handler(view.HomePage(server.MaybeUser{User: user, Ok: ok}, snippets))
	v.ServeHTTP(w, r)
}

func (h Handler) HandleEditor(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "GET") {
		return
	}

	session := h.DefaultSession(r)
	user, ok := h.authorizedUser(session)

	v := templ.Handler(view.EditorPage(server.MaybeUser{User: user, Ok: ok}))
	v.ServeHTTP(w, r)
}

func (h Handler) HandleSnippet(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "GET") {
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUser(session)

	var snippet server.Snippet
	var comments []server.Comment
	ok := true

	// Parse snippet id from the URL.
	snippetId, err := UUIDQueryGet(r, "id")
	if err != nil {
		// Invalid id, just redirect.
		Redirect(w, "/")
		return
	}

	// Request the snippet.
	snippet, err = server.RequestSnippet(r, h.db, snippetId, user.Id, authed)
	if err == sql.ErrNoRows {
		ok = false
	} else if err != nil {
		ErrorPage(w, r, err)
		return
	}

	if ok {
		// Request snippet comments.
		comments = make([]server.Comment, 0)
		err = server.RequestSnippetComments(h.db, snippetId, &comments)
		if err != nil {
			ErrorPage(w, r, err)
			return
		}
	}

	// Render the page.
	v := templ.Handler(view.SnippetPage(
		server.MaybeSnippet{Snippet: snippet, Ok: ok},
		server.MaybeUser{User: user, Ok: authed},
		comments,
	))
	v.ServeHTTP(w, r)
}

func (h Handler) HandleHey(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "GET") {
		return
	}

	w.Write([]byte("hello"))
}

// --------------------
// Users API.
// --------------------

func (h Handler) HandleApiLogin(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.login(w, r)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}
func (h Handler) HandleApiRegister(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.register(w, r)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}
func (h Handler) HandleApiLogout(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	h.logout(w, r)
	Redirect(w, "/")
}

// --------------------
// Snippets API.
// --------------------

func (h Handler) HandleApiSnippet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		id, err := h.postSnippet(r)
		if err != nil {
			Error(w, err)
			return
		}

		w.Write([]byte(fmt.Sprintf("%s", id)))
	case "GET":
		err := h.snippetSource(w, r)
		if err != nil {
			Error(w, err)
			return
		}
	case "DELETE":
		h.handleDeleteApiSnippet(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h Handler) handleDeleteApiSnippet(w http.ResponseWriter, r *http.Request) {
	id, err := UUIDQueryGet(r, "id")
	if err != nil {
		Error(w, err)
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUser(session)
	if !authed {
		Error(w, ErrUserNotAuth)
		return
	}

	err = h.deleteSnippet(r, user, id)
	if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}

func (h Handler) HandleApiFlower(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	snippetId, count, flowered, err := h.flowerSnippet(r)
	if err != nil {
		Error(w, err)
		return
	}

	// Send the updated number of flowers back.
	v := templ.Handler(view.SnippetFlowers(snippetId, count, flowered, true))
	v.ServeHTTP(w, r)
}

// --------------------
// Comments API.
// --------------------

func (h Handler) HandleApiComment(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		h.handlePostApiComment(w, r)
	case "DELETE":
		h.handleDeleteApiComment(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h Handler) handlePostApiComment(w http.ResponseWriter, r *http.Request) {
	snippetId, err := UUIDPathValue(r, "id")
	if err != nil {
		Error(w, err)
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUser(session)
	if !authed {
		Error(w, ErrUserNotAuth)
		return
	}

	err = h.postComment(r, user, snippetId)
	if err != nil {
		Error(w, err)
		return
	}

	// Request snippet comments.
	comments := make([]server.Comment, 0)
	err = server.RequestSnippetComments(h.db, snippetId, &comments)
	if err != nil {
		Error(w, err)
		return
	}

	v := templ.Handler(view.SnippetComments(
		snippetId,
		server.MaybeUser{User: user, Ok: true},
		comments,
	))
	v.ServeHTTP(w, r)
}

func (h Handler) handleDeleteApiComment(w http.ResponseWriter, r *http.Request) {
	commentId, err := UintPathValue(r, "id")
	if err != nil {
		Error(w, err)
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUser(session)
	if !authed {
		Error(w, ErrUserNotAuth)
		return
	}

	comment, err := h.deleteComment(r, user, commentId)
	if err != nil {
		Error(w, err)
		return
	}

	// Request snippet comments.
	comments := make([]server.Comment, 0)
	err = server.RequestSnippetComments(h.db, comment.SnippetId, &comments)
	if err != nil {
		Error(w, err)
		return
	}

	v := templ.Handler(view.SnippetComments(
		comment.SnippetId,
		server.MaybeUser{User: user, Ok: true},
		comments,
	))
	v.ServeHTTP(w, r)
}
