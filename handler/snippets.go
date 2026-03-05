package handler

import (
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"ohmysmal/consts"
	"ohmysmal/view"
)

func (h Handler) HandleApiSnippet(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.postSnippet(r)
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

	snippetId, number, flowered, err := h.flowerSnippet(r)
	if err != nil {
		// TODO: when user is not authorized we should show some sort of an
		// alert that says "hey, you should sign in".
		Error(w, err)
		return
	}

	// Send the updated number of flowers back.
	v := templ.Handler(view.SnippetFlowerButton(snippetId, number, flowered))
	v.ServeHTTP(w, r)
}

func (h Handler) postSnippet(r *http.Request) (err error) {
	user, found := h.authorizedUser()
	if !found {
		return ErrUserNotAuth
	}

	// Parse form data.
	err = r.ParseMultipartForm(consts.MAX_SNIPPET_FILE_SIZE * 3) // *3 just in case
	if err != nil {
		return BadRequestError{err.Error()}
	}

	// TODO: remove repeating whitespaces from the description.
	description := strings.TrimSpace(r.FormValue("description"))

	if utf8.RuneCountInString(description) > consts.MAX_SNIPPET_DESCRIPTION_LEN {
		return UserError{fmt.Sprintf("Snippet description can't exceed %d characters.", consts.MAX_SNIPPET_DESCRIPTION_LEN)}
	}

	// Store the received file to the server's file system.
	file, header, err := r.FormFile("file")
	if err != nil {
		return err
	}
	filename, err := validateAndWriteFile(file, header)
	if err != nil {
		return err
	}

	// Insert snippet to the database.
	stmt, err := h.db.Prepare("INSERT INTO snippets (author_id, filename, description) VALUES (?, ?, ?)")
	if err != nil {
		return nil
	}
	defer stmt.Close()

	_, err = stmt.Exec(user.Id, filename, description)
	if err != nil {
		return err
	}

	return nil
}

func validateAndWriteFile(file multipart.File, header *multipart.FileHeader) (filename string, err error) {
	// TODO: check for text file.

	if header.Size > consts.MAX_SNIPPET_FILE_SIZE {
		return "", UserError{"Source file is too large! It should not exceed 65KB of size."}
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	filename = uuid.New().String()
	path := fmt.Sprintf("./snippets/%s.smal", filename)
	const perm = 0o644 // read-write for owner, read-only for other

	err = os.WriteFile(path, contents, perm)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func (h Handler) flowerSnippet(r *http.Request) (snippetId uint, flowers uint, flowered bool, err error) {
	user, ok := h.authorizedUser()
	if !ok {
		return 0, 0, false, ErrUserNotAuth
	}

	// Parse path params.
	snippetId, err = UintPathValue(r, "snippet_id")
	if err != nil {
		return 0, 0, false, err
	}

	// Request the current number of flowers in the snippet.
	err = h.db.QueryRow("SELECT flowers FROM snippets WHERE id = ?", snippetId).Scan(&flowers)
	if err == sql.ErrNoRows {
		return 0, 0, false, BadRequestError{"no snippet with this id"}
	} else if err != nil {
		return 0, 0, false, err
	}

	// Insert the flower to the database.
	tx, err := h.db.Begin()
	if err != nil {
		return 0, 0, false, err
	}
	defer tx.Rollback()

	// Give a flower to the snippet if the user haven't gave it before.
	// The number of flowers in the snippet row will be updated automatically because of triggers.
	result, err := tx.Exec("INSERT IGNORE INTO flowers (user_id, snippet_id) VALUES (?, ?)", user.Id, snippetId)
	if err != nil {
		return 0, 0, false, err
	}

	if n, _ := result.RowsAffected(); n <= 0 {
		// 0 rows were modified meaning that a flower was already given to the snippet. Take it back!!
		// Again, the number of flowers in the snippet row will be updated
		// automatically by MySql.
		_, err := tx.Exec("DELETE FROM flowers WHERE user_id = ? AND snippet_id = ?", user.Id, snippetId)
		if err != nil {
			return 0, 0, false, err
		}

		flowered = false
		flowers -= 1
	} else {
		flowered = true
		flowers += 1
	}

	return snippetId, flowers, flowered, tx.Commit()
}
