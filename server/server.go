package server

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
)

// TODO: move the password into an .env file.
const SOURCE_NAME = "root:root@/ohmysmal"

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
	Id       uint
	Nickname string
	Password string
	Role     UserRole
	Status   UserStatus
}

type Snippet struct {
	Id       uuid.UUID
	AuthorId uint
	Title    string
	Flowers  uint
	Comments uint
	Status   SnippetStatus

	AuthorNickname   string // Joined.
	AuthUserFlowered bool   // Whether the currently authorized user flowered this snippet.
}

type Comment struct {
	Id             uint
	AuthorId       uint
	SnippetId      uuid.UUID
	Text           string
	AuthorNickname string // Joined.
}

func ConnectDatabase() *sql.DB {
	db, err := sql.Open("mysql", SOURCE_NAME)
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

func requestUserWith(db *sql.DB, condition string, params ...any) (user User, err error) {
	row := db.QueryRow("SELECT * FROM users_with_enums "+condition, params...)
	err = row.Scan(&user.Id, &user.Nickname, &user.Password, &user.Role, &user.Status)

	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	} else if err != nil {
		return User{}, err
	}

	return user, nil
}

func RequestUserById(db *sql.DB, id uint) (user User, err error) {
	return requestUserWith(db, "WHERE id = ?", id)
}

func RequestUserByNickname(db *sql.DB, nickname string) (user User, err error) {
	return requestUserWith(db, "WHERE nickname = ?", nickname)
}

func IsNicknameTaken(db *sql.DB, nickname string) (taken bool, err error) {
	row := db.QueryRow("SELECT id FROM users WHERE nickname = ?", nickname)

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
		`, authUserId)
	} else {
		rows, err = db.Query("SELECT *, false as flowered FROM snippets_with_author")
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
	db *sql.DB,
	id uuid.UUID,
	authUserId uint,
	hasAuthUser bool,
) (snippet Snippet, err error) {
	var row *sql.Row

	if hasAuthUser {
		// Select all snippets + add a new column "flowered" that indicates
		// wheter the user with id `authUserId` flowered this snippet.
		row = db.QueryRow(`
		SELECT
			snippets_with_author.*,
			(flowers.user_id IS NOT NULL) as flowered
		FROM snippets_with_author
		LEFT JOIN flowers ON id = flowers.snippet_id AND flowers.user_id = ?
		WHERE id = ?
		`, authUserId, id[:])
	} else {
		row = db.QueryRow("SELECT *, false as flowered FROM snippets_with_author WHERE id = ?", id[:])
	}

	if RowScanSnippet(row, &snippet) != nil {
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
	`, id[:])

	if err != nil {
		return err
	}

	var c Comment

	for {
		if !rows.Next() {
			break
		}

		err = rows.Scan(&c.Id, &c.SnippetId, &c.AuthorId, &c.Text, &c.AuthorNickname)
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
		&s.AuthorNickname,
		&s.AuthUserFlowered,
	)
}
