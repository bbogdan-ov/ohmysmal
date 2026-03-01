package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ohmysmal/view"
)

const (
	MAX_SNIPPET_FILE_SIZE = 65535 // 65kb
)

func (h Handler) HandleApiSnippet(c *gin.Context) {
	err := h.postSnippet(c)
	if err != nil {
		writeError(c, err)
		return
	}

	writeRedirect(c, "/")
}

func (h Handler) HandleApiFlower(c *gin.Context) {
	snippetId, number, flowered, err := h.flowerSnippet(c)
	if err != nil {
		// TODO: when user is not authorized we should show some sort of an
		// alert that says "hey, you should sign in".
		writeError(c, err)
		return
	}

	// Send the updated number of flowers back.
	c.HTML(http.StatusOK, "", view.SnippetFlowerButton(snippetId, number, flowered))
}

func (h Handler) postSnippet(c *gin.Context) (err error) {
	session := sessions.Default(c)

	// Update the cached user because we need an up-to-date data.
	h.updateUserCache(session)

	user, ok := h.authorizedUser()
	if !ok {
		return ErrUserNotAuth
	}

	// Parse form data.
	err = c.Request.ParseMultipartForm(MAX_SNIPPET_FILE_SIZE * 3) // *3 just in case
	if err != nil {
		return BadRequestError{err.Error()}
	}

	// TODO: remove repeating whitespaces from the description.
	description := strings.TrimSpace(c.Request.FormValue("description"))

	// Store the received file into the server's file system.
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return err
	}
	filename, err := validateAndWriteFile(file, header)
	if err != nil {
		return err
	}

	// Write snippet into the database.
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

	if header.Size > MAX_SNIPPET_FILE_SIZE {
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

func (h Handler) flowerSnippet(c *gin.Context) (snippetId uint, flowers uint, flowered bool, err error) {
	user, ok := h.authorizedUser()
	if !ok {
		return 0, 0, false, ErrUserNotAuth
	}

	idStr, ok := c.Params.Get("id")
	if !ok {
		return 0, 0, false, BadRequestError{"no id is provided"}
	}

	num, err := strconv.ParseUint(idStr, 10, 32)
	snippetId = uint(num)
	if err != nil {
		return 0, 0, false, BadRequestError{fmt.Sprintf("id param is of an invalid type: %s", err)}
	}

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
	} else {
		flowered = true
	}

	err = tx.QueryRow("SELECT flowers FROM snippets WHERE id = ?", snippetId).Scan(&flowers)
	if err != nil {
		return 0, 0, false, err
	}

	return snippetId, flowers, flowered, tx.Commit()
}
