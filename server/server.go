package server

import (
	"fmt"
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserRole int
type UserStatus int
type SnippetStatus int

const (
	ROLE_USER UserRole = iota
	ROLE_ADMIN
)

const (
	USER_OK UserStatus = iota
	USER_BANNED
)

const (
	SNIPPET_OK SnippetStatus = iota
	SNIPPET_BANNED
)

type User struct {
	Id           uint
	Nickname     string
	Password     string
	Role         UserRole
	Status       UserStatus
	RegisterDate time.Time
}

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

type MaybeUser struct {
	User
	Ok bool
}

type MaybeSnippet struct {
	Snippet
	Ok bool
}

type Comment struct {
	Id             uint
	AuthorId       uint
	SnippetId      uuid.UUID
	Text           string
	Date           time.Time
	AuthorNickname string // Joined.
}

func ConnectDatabase(username, password string) *sql.DB {
	source := fmt.Sprintf("%s:%s@/ohmysmal?parseTime=true&loc=Local", username, password)
	db, err := sql.Open("mysql", source)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %s", err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping the database: %s", err)
	}

	log.Print("Successfully connected to the database")

	return db
}

func requestUserWith(r *http.Request, db *sql.DB, condition string, params ...any) (user User, err error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT * FROM users_with_enums "+condition, params...)
	err = row.Scan(&user.Id, &user.Nickname, &user.Password, &user.Role, &user.Status, &user.RegisterDate)

	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	} else if err != nil {
		return User{}, err
	}

	return user, nil
}

func RequestUserById(r *http.Request, db *sql.DB, id uint) (user User, err error) {
	return requestUserWith(r, db, "WHERE id = ?", id)
}

func RequestUserByNickname(r *http.Request, db *sql.DB, nickname string) (user User, err error) {
	return requestUserWith(r, db, "WHERE nickname = ?", nickname)
}

func IsNicknameTaken(r *http.Request, db *sql.DB, nickname string) (taken bool, err error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT id FROM users WHERE nickname = ?", nickname)

	var id int
	err = row.Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

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

func RequestSnippetComments(db *sql.DB, id uuid.UUID, comments *[]Comment) (err error) {
	rows, err := db.Query(`
	SELECT comments.*, users.nickname as author_nickname
	FROM comments
	JOIN users ON author_id = users.id
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

		err = rows.Scan(&c.Id, &c.SnippetId, &c.AuthorId, &c.Text, &c.Date, &c.AuthorNickname)
		if err != nil {
			return err
		}

		*comments = append(*comments, c)
	}

	return nil
}

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
