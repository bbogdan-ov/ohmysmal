package handler

import (
	"fmt"
	"net/http"
	"ohmysmal/consts"
	"strings"
	"unicode/utf8"
)

func (h Handler) HandleApiComment(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.postComment(r)
	if err != nil {
		Error(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h Handler) postComment(r *http.Request) (err error) {
	user, found := h.authorizedUser()
	if !found {
		return ErrUserNotAuth
	}

	// Parse path params.
	snippetId, err := UintPathValue(r, "snippet_id")
	if err != nil {
		return err
	}

	// Parse form data.
	err = r.ParseForm()
	if err != nil {
		return err
	}

	text := strings.TrimSpace(r.FormValue("text")) // NOTE: allow repeating whitespaces because why not.
	if text == "" {
		return UserError{"Text is required."}
	}

	if utf8.RuneCountInString(text) > consts.MAX_COMMENT_TEXT_LEN {
		return UserError{fmt.Sprintf("Comments can't exceed %d characters.", consts.MAX_COMMENT_TEXT_LEN)}
	}

	// Insert the comment to the database.
	_, err = h.db.Exec(
		"INSERT INTO comments (snippet_id, author_id, text) VALUES (?, ?, ?)",
		snippetId,
		user.Id,
		text,
	)
	if err != nil {
		return err
	}

	return nil
}
