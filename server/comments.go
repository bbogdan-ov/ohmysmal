package server

import (
	"context"
	"database/sql"
	"time"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"

	"ohmysmal/consts"
)

type Comment struct {
	Id             uint
	AuthorId       uint
	SnippetId      uuid.UUID
	Text           string
	Date           time.Time
	AuthorNickname string // Joined.
}

// --------------------
// Modify.
// --------------------

func PostComment(
	db *sql.DB,
	ctx context.Context,
	user User,
	snippetId uuid.UUID,
	text string,
) error {
	if text == "" {
		return BadRequestError{"Comment text can't be empty."}
	} else if utf8.RuneCountInString(text) > consts.MAX_COMMENT_TEXT_LEN {
		return BadRequestError{fmt.Sprintf("Comments can't exceed %d characters.", consts.MAX_COMMENT_TEXT_LEN)}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Insert the comment to the database.
	_, err := db.ExecContext(
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

func DeleteComment(
	db *sql.DB,
	ctx context.Context,
	user User,
	id uint,
) (comment Comment, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT * FROM comments_with_author WHERE id = ?", id)
	err = RowScanComment(row, &comment)
	if err == sql.ErrNoRows {
		return comment, BadRequestError{"no such comment"}
	} else if err != nil {
		return comment, err
	}

	if !(user.Role == ROLE_ADMIN || comment.AuthorId == user.Id) {
		return comment, BadRequestError{"not an author of the comment"}
	}

	_, err = db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", id)
	if err != nil {
		return comment, err
	}

	return comment, nil
}

// --------------------
// Request.
// --------------------

func RequestSnippetComments(db *sql.DB, id uuid.UUID, comments *[]Comment) (err error) {
	rows, err := db.Query(`
	SELECT * FROM comments_with_author
	WHERE snippet_id = ?
	ORDER BY date DESC
	`, id[:])

	if err != nil {
		return err
	}

	var c Comment

	for {
		if !rows.Next() {
			break
		}

		err = RowsScanComment(rows, &c)
		if err != nil {
			return err
		}

		*comments = append(*comments, c)
	}

	return nil
}

// --------------------
// Utils.
// --------------------

func RowScanComment(row *sql.Row, c *Comment) error {
	return row.Scan(
		&c.Id,
		&c.SnippetId,
		&c.AuthorId,
		&c.Text,
		&c.Date,
		&c.AuthorNickname,
	)
}
func RowsScanComment(rows *sql.Rows, c *Comment) error {
	return rows.Scan(
		&c.Id,
		&c.SnippetId,
		&c.AuthorId,
		&c.Text,
		&c.Date,
		&c.AuthorNickname,
	)
}
