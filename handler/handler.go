package handler

import (
	"fmt"
	"database/sql"
	"log"
	"net/http"
	"math/rand"
	"strings"
	"golang.org/x/crypto/bcrypt"

	"github.com/a-h/templ"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/robfig/go-cache"

	"ohmysmal/server"
	"ohmysmal/view"
	"ohmysmal/consts"
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
	user, authed := h.authorizedUserOrFalse(session)

	snippets := make([]server.Snippet, 0, 20)
	err := server.RequestSnippets(h.db, &snippets, user.Id, authed)
	if err != nil {
		log.Printf("ERROR: Failed to request the list of snippets: %s", err)
		ErrorPage(w, r, err)
		return
	}

	// Bunch of harassment messages.
	splashes := []string{
		"Feel UXN inside yourself",
		"Become UXN, feel UXN, love UXN",
		"UXN people",
		"UXN cures your diseases",
		"UXN, you can see it coming",
		"Ok UXN",
		"Yes, it's here, UXN",
	}
	splash := splashes[rand.Int() % len(splashes)]

	v := templ.Handler(view.HomePage(server.MaybeUser{User: user, Ok: authed}, snippets, splash))
	v.ServeHTTP(w, r)
}

func (h Handler) HandleEditor(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "GET") {
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUserOrFalse(session)

	var snippet server.Snippet
	var source string
	var err error

	snippetId, hasSnippet := UUIDQueryGetOrFalse(r, "snippet")
	if hasSnippet {
		snippet, source, err = server.SnippetSource(h.db, r.Context(), snippetId)
		if err != nil {
			ErrorPage(w, r, err)
			return
		}
	}

	v := templ.Handler(view.EditorPage(
		server.MaybeUser{User: user, Ok: authed},
		server.MaybeSnippet{Snippet: snippet, Ok: hasSnippet},
		source,
	))
	v.ServeHTTP(w, r)
}

func (h Handler) HandleSnippet(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "GET") {
		return
	}

	session := h.DefaultSession(r)
	user, authed := h.authorizedUserOrFalse(session)

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
	snippet, err = server.RequestSnippet(h.db, r.Context(), snippetId, user.Id, authed)
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
	if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) (err error) {
	ErrInvalid := server.BadRequestError{"Invalid nickname or password."}

	session := h.DefaultSession(r)

	_, authed := h.authorizedUserOrFalse(session)
	if authed {
		return server.BadRequestError{"user already authorized"}
	}

	// Parse received form data.
	err = r.ParseForm()
	if err != nil {
		return server.BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	password := r.FormValue("password")

	if nickname == "" {
		return server.BadRequestError{"Nickname is required."}
	} else if password == "" {
		return server.BadRequestError{"Password is required."}
	} else if server.ValidateNickname(nickname) != nil || server.ValidatePassword(password) != nil {
		return ErrInvalid
	}

	// Request a user with the received nickname.
	user, err := server.RequestUserByNickname(r, h.db, nickname)
	if err == sql.ErrNoRows {
		return ErrInvalid
	} else if err != nil {
		log.Printf("USERS: ERROR: Login failed: Failed to request an existing user: %s", err)
		return err
	}

	// Compare the passwords.
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Passwords aren't equal.
		return ErrInvalid
	}

	rememberUser(w, r, session, user.Id)

	log.Printf("USERS: INFO: User successfully logged in: %d, %s", user.Id, user.Nickname)
	return nil
}

func (h Handler) HandleApiRegister(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.register(w, r)
	if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}

func (h Handler) register(w http.ResponseWriter, r *http.Request) (err error) {
	session := h.DefaultSession(r)

	_, authed := h.authorizedUserOrFalse(session)
	if authed {
		return server.BadRequestError{"user already authorized"}
	}

	// Parse received form data.
	err = r.ParseForm()
	if err != nil {
		return server.BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password-confirm")

	if nickname == "" {
		return server.BadRequestError{"Nickname is required."}
	} else if password == "" {
		return server.BadRequestError{"Password is required."}
	} else if passwordConfirm == "" {
		return server.BadRequestError{"Confirm the password."}
	}

	if password != passwordConfirm {
		return server.BadRequestError{"Passwords do not match."}
	}

	id, err := server.InsertUser(h.db, r.Context(), nickname, password)
	if err != nil {
		return err
	}

	rememberUser(w, r, session, id)

	log.Printf("USERS: INFO: User successfully registered: %d, %s", id, nickname)
	return nil
}

func (h Handler) HandleApiLogout(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	h.logout(w, r)
	Redirect(w, "/")
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	session := h.DefaultSession(r)

	userId, found := h.authorizedUserId(session)
	if found {
		log.Printf("USERS: INFO: User logged out: %d", userId)
		_ = h.cache.Delete(fmtUserCacheKey(userId)) // ignore if not found
	}

	delete(session.Values, USER_ID_SESSION_KEY)
	_ = session.Save(r, w)
}

// --------------------
// Snippets API.
// --------------------

func (h Handler) HandleApiSnippet(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case "POST":
		err = h.handlePostApiSnippet(w, r)
	case "GET":
		err = h.handleGetApiSnippet(w, r)
	case "DELETE":
		err = h.handleDeleteApiSnippet(w, r)
	case "PATCH":
		err = h.handlePatchApiSnippet(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

	if err != nil {
		Error(w, err)
	}
}

func (h Handler) handlePostApiSnippet(w http.ResponseWriter, r *http.Request) error {
	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		return err
	}

	// Parse form data.
	err = r.ParseMultipartForm(consts.MAX_SNIPPET_FILE_SIZE * 3) // *3 just in case
	if err != nil {
		return server.BadRequestError{err.Error()}
	}

	title := r.FormValue("title")

	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}

	id, err := server.PostSnippet(h.db, r.Context(), user, title, file, header)
	if err != nil {
		return err
	}

	w.Write([]byte(fmt.Sprintf("%s", id)))
	return nil
}

func (h Handler) handleGetApiSnippet(w http.ResponseWriter, r *http.Request) error {
	id, err := UUIDQueryGet(r, "id")
	if err != nil {
		return err
	}

	_, source, err := server.SnippetSource(h.db, r.Context(), id)
	if err != nil {
		return err
	}

	w.Write([]byte(source))
	return nil
}

func (h Handler) handleDeleteApiSnippet(w http.ResponseWriter, r *http.Request) error {
	id, err := UUIDQueryGet(r, "id")
	if err != nil {
		return err
	}

	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		return err
	}

	err = server.DeleteSnippet(h.db, r.Context(), user, id)
	if err != nil {
		return err
	}

	Redirect(w, "/")
	return nil
}

func (h Handler) handlePatchApiSnippet(w http.ResponseWriter, r *http.Request) error {
	id, err := UUIDQueryGet(r, "id")
	if err != nil {
		return err
	}

	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		return err
	}

	// Parse form data.
	err = r.ParseMultipartForm(consts.MAX_SNIPPET_FILE_SIZE * 3) // *3 just in case
	if err != nil {
		return server.BadRequestError{err.Error()}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}

	err = server.UpdateSnippetSource(h.db, r.Context(), user, id, file, header)
	if err != nil {
		return err
	}

	return nil
}

func (h Handler) HandleApiFlower(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		Error(w, err)
		return
	}

	snippetId, err := UUIDPathValue(r, "snippet_id")
	if err != nil {
		Error(w, err)
		return
	}

	count, flowered, err := server.FlowerSnippet(h.db, r.Context(), user, snippetId)
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
	var err error
	switch r.Method {
	case "POST":
		err = h.handlePostApiComment(w, r)
	case "DELETE":
		err = h.handleDeleteApiComment(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

	if err != nil {
		Error(w, err)
	}
}

func (h Handler) handlePostApiComment(w http.ResponseWriter, r *http.Request) error {
	snippetId, err := UUIDPathValue(r, "id")
	if err != nil {
		return err
	}

	text := r.FormValue("text")

	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		return err
	}

	err = server.PostComment(h.db, r.Context(), user, snippetId, text)
	if err != nil {
		return err
	}

	// Request snippet comments.
	comments := make([]server.Comment, 0)
	err = server.RequestSnippetComments(h.db, snippetId, &comments)
	if err != nil {
		return err
	}

	v := templ.Handler(view.SnippetComments(
		snippetId,
		server.MaybeUser{User: user, Ok: true},
		comments,
	))
	v.ServeHTTP(w, r)

	return nil
}

func (h Handler) handleDeleteApiComment(w http.ResponseWriter, r *http.Request) error {
	commentId, err := UintPathValue(r, "id")
	if err != nil {
		return err
	}

	session := h.DefaultSession(r)
	user, err := h.authorizedUser(session)
	if err != nil {
		return err
	}

	comment, err := server.DeleteComment(h.db, r.Context(), user, commentId)
	if err != nil {
		return err
	}

	// Request snippet comments.
	comments := make([]server.Comment, 0)
	err = server.RequestSnippetComments(h.db, comment.SnippetId, &comments)
	if err != nil {
		return err
	}

	v := templ.Handler(view.SnippetComments(
		comment.SnippetId,
		server.MaybeUser{User: user, Ok: true},
		comments,
	))
	v.ServeHTTP(w, r)

	return nil
}
