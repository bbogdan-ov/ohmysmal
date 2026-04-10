package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"ohmysmal/consts"
	"ohmysmal/view"
)

const SNIPPETS_DIR = "./snippets"

func (h Handler) HandleApiSnippet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		id, err := h.postSnippet(r)
		if err != nil {
			Error(w, err)
			return
		}

		Redirect(w, fmt.Sprintf("/snippet?id=%s", id))
	case "GET":
		err := h.snippetSource(w, r)
		if err != nil {
			Error(w, err)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h Handler) HandleApiFlower(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	snippetId, count, flowered, err := h.flowerSnippet(r)
	if err != nil {
		// TODO: when user is not authorized we should show some sort of an
		// alert that says "hey, you should sign in".
		Error(w, err)
		return
	}

	// Send the updated number of flowers back.
	v := templ.Handler(view.SnippetFlowers(snippetId, count, flowered))
	v.ServeHTTP(w, r)
}

func (h Handler) postSnippet(r *http.Request) (id uuid.UUID, err error) {
	session := h.DefaultSession(r)
	user, found := h.authorizedUser(session)
	if !found {
		log.Printf("SNIPPETS: WARNING: Not authed user tried to create a snippet")
		return uuid.UUID{}, ErrUserNotAuth
	}

	// Parse form data.
	err = r.ParseMultipartForm(consts.MAX_SNIPPET_FILE_SIZE * 3) // *3 just in case
	if err != nil {
		return uuid.UUID{}, BadRequestError{err.Error()}
	}

	// TODO: remove repeating whitespaces from the title.
	title := strings.TrimSpace(r.FormValue("title"))

	if utf8.RuneCountInString(title) > consts.MAX_SNIPPET_TITLE_LEN {
		msg := fmt.Sprintf("Snippet title can't exceed %d characters.", consts.MAX_SNIPPET_TITLE_LEN)
		return uuid.UUID{}, UserError{msg}
	}

	id = uuid.New()

	// Store the received file to the server's file system.
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("SNIPPETS: ERROR: Snippet posting failed: Failed to parse form file: %s", err)
		return uuid.UUID{}, err
	}
	err = validateAndWriteFile(id, file, header)
	if err != nil {
		log.Printf("SNIPPETS: ERROR: Snippet posting failed: File is invalid: %s", err)
		return uuid.UUID{}, err
	}

	// Insert snippet to the database.
	_, err = h.db.Exec("INSERT INTO snippets (id, author_id, title) VALUES (?, ?, ?)", id[:], user.Id, title)
	if err != nil {
		log.Printf("SNIPPETS: ERROR: Snippet posting failed: Failed to insert the snippet data into the database: %s", err)
		return uuid.UUID{}, err
	}

	return id, nil
}

func (h Handler) snippetSource(w http.ResponseWriter, r *http.Request) (err error) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		return BadRequestError{"no id query param is provided"}
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return BadRequestError{err.Error()}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := h.db.QueryRowContext(ctx, "SELECT id FROM snippets WHERE id = ?", id)

	err = row.Err()
	if err == sql.ErrNoRows {
		return UserError{"No such snippet"}
	} else if err != nil {
		log.Printf("SNIPPETS: ERROR: Failed to fetch snippet source code: %s", err)
		return err
	}

	http.ServeFile(w, r, fmtSnippetFilename(id))

	return nil
}

func validateAndWriteFile(id uuid.UUID, file multipart.File, header *multipart.FileHeader) (err error) {
	// TODO: check for text file.

	if header.Size > consts.MAX_SNIPPET_FILE_SIZE {
		return UserError{"Source file is too large! It should not exceed 65KB of size."}
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

func (h Handler) flowerSnippet(r *http.Request) (snippetId uuid.UUID, flowers uint, flowered bool, err error) {
	session := h.DefaultSession(r)
	user, ok := h.authorizedUser(session)
	if !ok {
		return uuid.UUID{}, 0, false, ErrUserNotAuth
	}

	// Parse path params.
	snippetId, err = UUIDPathValue(r, "snippet_id")
	idBytes := snippetId[:]
	if err != nil {
		return uuid.UUID{}, 0, false, err
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Request the current number of flowers in the snippet.
	err = h.db.QueryRowContext(ctx, "SELECT flowers FROM snippets WHERE id = ?", idBytes).Scan(&flowers)
	if err == sql.ErrNoRows {
		return uuid.UUID{}, 0, false, BadRequestError{"no snippet with this id"}
	} else if err != nil {
		return uuid.UUID{}, 0, false, err
	}

	// Insert the flower to the database.
	tx, err := h.db.Begin()
	if err != nil {
		return uuid.UUID{}, 0, false, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Give a flower to the snippet if the user haven't gave it before.
	// The number of flowers in the snippet row will be updated automatically because of triggers.
	result, err := tx.Exec("INSERT IGNORE INTO flowers (user_id, snippet_id) VALUES (?, ?)", user.Id, idBytes)
	if err != nil {
		return uuid.UUID{}, 0, false, err
	}

	if n, _ := result.RowsAffected(); n <= 0 {
		// 0 rows were modified meaning that a flower was already given to the snippet. Take it back!!
		// Again, the number of flowers in the snippet row will be updated
		// automatically by MySql.
		_, err := tx.Exec("DELETE FROM flowers WHERE user_id = ? AND snippet_id = ?", user.Id, idBytes)
		if err != nil {
			return uuid.UUID{}, 0, false, err
		}

		if flowers > 0 {
			flowers -= 1
		} else {
			return uuid.UUID{}, 0, false, errors.New("cannot substruct from 0 flowers")
		}

		flowered = false
	} else {
		flowered = true
		flowers += 1
	}

	return snippetId, flowers, flowered, tx.Commit()
}

func fmtSnippetFilename(id uuid.UUID) string {
	return fmt.Sprintf("%s/%s.smal", SNIPPETS_DIR, id)
}
