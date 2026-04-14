package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"mime/multipart"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"ohmysmal/consts"
)

type SnippetStatus int

const SNIPPETS_DIR = "./snippets"

const (
	SNIPPET_OK SnippetStatus = iota
	SNIPPET_BANNED
)

type Snippet struct {
	Id       uuid.UUID
	AuthorId uint
	Title    string
	Flowers  uint
	Comments uint
	Status   SnippetStatus
	Date     time.Time
	RemixOf  uuid.UUID

	AuthorNickname   string // Joined.
	AuthUserFlowered bool   // Whether the currently authorized user flowered this snippet.
}

type MaybeSnippet struct {
	Snippet
	Ok bool
}

// --------------------
// Modify.
// --------------------

func PostSnippet(
	db *sql.DB,
	ctx context.Context,
	user User,
	title string,
	file multipart.File,
	header *multipart.FileHeader,
) (id uuid.UUID, err error) {
	// TODO: remove repeating whitespaces from the title.
	title = strings.TrimSpace(title)
	if title == "" {
		return uuid.UUID{}, BadRequestError{"Snippet title can't be empty."}
	} else if utf8.RuneCountInString(title) > consts.MAX_SNIPPET_TITLE_LEN {
		msg := fmt.Sprintf("Snippet title can't exceed %d characters.", consts.MAX_SNIPPET_TITLE_LEN)
		return uuid.UUID{}, BadRequestError{msg}
	}

	id = uuid.New()

	// Store file to the file system.
	err = validateAndWriteFile(id, file, header)
	if err != nil {
		log.Printf("SNIPPETS: ERROR: Snippet posting failed: File is invalid: %s", err)
		return uuid.UUID{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Insert snippet to the database.
	_, err = db.ExecContext(ctx, "INSERT INTO snippets (id, author_id, title) VALUES (?, ?, ?)", id[:], user.Id, title)
	if err != nil {
		log.Printf("SNIPPETS: ERROR: Snippet posting failed: Failed to insert the snippet data into the database: %s", err)
		return uuid.UUID{}, err
	}

	return id, nil
}

func SnippetSource(db *sql.DB, ctx context.Context, id uuid.UUID) (source string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT id FROM snippets WHERE id = ?", id)

	err = row.Err()
	if err == sql.ErrNoRows {
		return "", BadRequestError{"no such snippet"}
	} else if err != nil {
		log.Printf("SNIPPETS: ERROR: Failed to fetch snippet source code: %s", err)
		return "", err
	}

	contents, err := os.ReadFile(fmtSnippetFilename(id))
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

func validateAndWriteFile(id uuid.UUID, file multipart.File, header *multipart.FileHeader) error {
	// TODO: check for text file.

	if header.Size == 0 {
		return BadRequestError{"Source file can't be empty!"}
	} else if header.Size > consts.MAX_SNIPPET_FILE_SIZE {
		msg := fmt.Sprintf("Source file is too large! It should not exceed %d bytes of size, got %d.", consts.MAX_SNIPPET_FILE_SIZE, header.Size)
		return BadRequestError{msg}
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	const perm = 0o644 // read-write for owner, read-only for other

	err = os.WriteFile(fmtSnippetFilename(id), contents, perm)
	if err != nil {
		return err
	}

	return nil
}

func FlowerSnippet(
	db *sql.DB,
	ctx context.Context,
	user User,
	snippetId uuid.UUID,
) (flowers uint, flowered bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Request the current number of flowers in the snippet.
	err = db.QueryRowContext(ctx, "SELECT flowers FROM snippets WHERE id = ?", snippetId[:]).Scan(&flowers)
	if err == sql.ErrNoRows {
		return 0, false, BadRequestError{"no snippet with this id"}
	} else if err != nil {
		return 0, false, err
	}

	// Insert the flower to the database.
	tx, err := db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Give a flower to the snippet if the user haven't gave it before.
	// The number of flowers in the snippet row will be updated automatically because of triggers.
	result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO flowers (user_id, snippet_id) VALUES (?, ?)", user.Id, snippetId[:])
	if err != nil {
		return 0, false, err
	}

	if n, _ := result.RowsAffected(); n <= 0 {
		// 0 rows were modified meaning that a flower was already given to the snippet. Take it back!!
		// Again, the number of flowers in the snippet row will be updated
		// automatically by MySql.
		_, err := tx.ExecContext(ctx, "DELETE FROM flowers WHERE user_id = ? AND snippet_id = ?", user.Id, snippetId[:])
		if err != nil {
			return 0, false, err
		}

		if flowers > 0 {
			flowers -= 1
		} else {
			return 0, false, errors.New("cannot substruct from 0 flowers")
		}

		flowered = false
	} else {
		flowered = true
		flowers += 1
	}

	return flowers, flowered, tx.Commit()
}

func DeleteSnippet(
	db *sql.DB,
	ctx context.Context,
	user User,
	id uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var authorId uint
	row := db.QueryRowContext(ctx, "SELECT author_id FROM snippets WHERE id = ?", id[:])
	err := row.Scan(&authorId)
	if err == sql.ErrNoRows {
		return BadRequestError{"no such snippet"}
	} else if err != nil {
		return err
	}

	if !(user.Role == ROLE_ADMIN || authorId == user.Id) {
		return BadRequestError{"not an author of the comment"}
	}

	_, err = db.ExecContext(ctx, "DELETE FROM snippets WHERE id = ?", id[:])
	if err != nil {
		return err
	}

	return nil
}

// --------------------
// Request.
// --------------------

func RequestSnippets(db *sql.DB, snippets *[]Snippet, authUserId uint, hasAuthUser bool) (err error) {
	var rows *sql.Rows

	if hasAuthUser {
		// Select all snippets + add a new column "flowered" that indicates
		// wheter the user with id `authUserId` flowered this snippet.
		rows, err = db.Query(`
		SELECT
			snippets_with_author.*,
			(flowers.user_id IS NOT NULL) as flowered
		FROM snippets_with_author
		LEFT JOIN flowers ON id = flowers.snippet_id AND flowers.user_id = ?
		ORDER BY date DESC
		`, authUserId)
	} else {
		rows, err = db.Query("SELECT *, false as flowered FROM snippets_with_author ORDER BY date DESC")
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	var s Snippet

	for {
		if !rows.Next() {
			break
		}

		err = RowsScanSnippet(rows, &s)
		if err != nil {
			return err
		}

		*snippets = append(*snippets, s)
	}

	return nil
}

func RequestSnippet(
	r *http.Request,
	db *sql.DB,
	id uuid.UUID,
	authUserId uint,
	hasAuthUser bool,
) (snippet Snippet, err error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var row *sql.Row

	if hasAuthUser {
		// Select all snippets + add a new column "flowered" that indicates
		// wheter the user with id `authUserId` flowered this snippet.
		row = db.QueryRowContext(ctx, `
		SELECT
			snippets_with_author.*,
			(flowers.user_id IS NOT NULL) as flowered
		FROM snippets_with_author
		LEFT JOIN flowers ON id = flowers.snippet_id AND flowers.user_id = ?
		WHERE id = ?
		`, authUserId, id[:])
	} else {
		row = db.QueryRowContext(ctx, "SELECT *, false as flowered FROM snippets_with_author WHERE id = ?", id[:])
	}

	err = RowScanSnippet(row, &snippet)
	if err != nil {
		return snippet, err
	}

	return snippet, nil
}

// --------------------
// Utils.
// --------------------

func RowScanSnippet(row *sql.Row, s *Snippet) error {
	return row.Scan(
		&s.Id,
		&s.AuthorId,
		&s.Title,
		&s.Flowers,
		&s.Comments,
		&s.Status,
		&s.Date,
		&s.RemixOf,
		&s.AuthorNickname,
		&s.AuthUserFlowered,
	)
}
func RowsScanSnippet(rows *sql.Rows, s *Snippet) error {
	return rows.Scan(
		&s.Id,
		&s.AuthorId,
		&s.Title,
		&s.Flowers,
		&s.Comments,
		&s.Status,
		&s.Date,
		&s.RemixOf,
		&s.AuthorNickname,
		&s.AuthUserFlowered,
	)
}

func fmtSnippetFilename(id uuid.UUID) string {
	return fmt.Sprintf("%s/%s.smal", SNIPPETS_DIR, id)
}
