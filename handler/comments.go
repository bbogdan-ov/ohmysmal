package handler

import (
	"fmt"
	"net/http"
	"ohmysmal/consts"
	"strings"
	"unicode/utf8"
)

func (h Handler) postComment(r *http.Request) (author string, text string, err error) {
	session := h.DefaultSession(r)
	user, found := h.authorizedUser(session)
	if !found {
		return "", "", ErrUserNotAuth
	}

	// Parse path params.
	snippetId, err := UUIDPathValue(r, "snippet_id")
	if err != nil {
		return "", "", err
	}

	// Parse form data.
	err = r.ParseForm()
	if err != nil {
		return "", "", err
	}

	text = strings.TrimSpace(r.FormValue("text")) // NOTE: allow repeating whitespaces because why not.
	if text == "" {
		return "", "", UserError{"Text is required."}
	}

	if utf8.RuneCountInString(text) > consts.MAX_COMMENT_TEXT_LEN {
		return "", "", UserError{fmt.Sprintf("Comments can't exceed %d characters.", consts.MAX_COMMENT_TEXT_LEN)}
	}

	// Insert the comment to the database.
	_, err = h.db.Exec(
		"INSERT INTO comments (snippet_id, author_id, text) VALUES (?, ?, ?)",
		snippetId[:],
		user.Id,
		text,
	)
	if err != nil {
		return "", "", err
	}

	return user.Nickname, text, nil
}
