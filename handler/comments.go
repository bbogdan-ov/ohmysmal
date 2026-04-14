package handler

import (
	"context"
	"database/sql"
	"time"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"ohmysmal/consts"
	"ohmysmal/server"
)

func (h Handler) postComment(r *http.Request, user server.User, snippetId uuid.UUID) error {
	// Parse form data.
	err := r.ParseForm()
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Insert the comment to the database.
	_, err = h.db.ExecContext(
		ctx,
		"INSERT INTO comments (snippet_id, author_id, text) VALUES (?, ?, ?)",
		snippetId[:],
		user.Id,
		text,
	)
	if err != nil {
		return err
	}

	return nil
}

func (h Handler) deleteComment(r *http.Request, user server.User, id uint) (comment server.Comment, err error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := h.db.QueryRowContext(ctx, "SELECT * FROM comments_with_author WHERE id = ?", id)
	err = server.RowScanComment(row, &comment)
	if err == sql.ErrNoRows {
		return comment, BadRequestError{"no such comment"}
	} else if err != nil {
		return comment, err
	}

	if !(user.Role == server.ROLE_ADMIN || comment.AuthorId == user.Id) {
		return comment, BadRequestError{"not an author of the comment"}
	}

	_, err = h.db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", id)
	if err != nil {
		return comment, err
	}

	return comment, nil
}
