package server

import (
	"context"
	"unicode/utf8"
	"database/sql"
	"net/http"
	"time"
	"golang.org/x/crypto/bcrypt"

	"ohmysmal/consts"
)

type UserRole int
type UserStatus int

const (
	ROLE_INVALID  UserRole = iota
	ROLE_USER
	ROLE_ADMIN
)

const (
	USER_OK UserStatus = iota
	USER_BANNED
)

type User struct {
	Id           uint
	Nickname     string
	Password     string
	Role         UserRole
	Status       UserStatus
	RegisterDate time.Time
}

type MaybeUser struct {
	User
	Ok bool
}

// --------------------
// Modify.
// --------------------

func InsertUser(db *sql.DB, ctx context.Context, nickname, password string) (id uint, err error) {
	err = ValidateNickname(nickname)
	if err != nil {
		return 0, err
	}

	err = ValidatePassword(password)
	if err != nil {
		return 0, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, BadRequestError{err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = ensureNicknameNotTaken(db, ctx, nickname)
	if err != nil {
		return 0, err
	}

	// Insert user to the server.
	result, err := db.ExecContext(ctx, "INSERT INTO users (nickname, password) VALUES (?, ?)", nickname, hashedPassword)
	if err != nil {
		return 0, err
	}

	id64, err := result.LastInsertId()
	return uint(id64), err
}

// --------------------
// Request.
// --------------------

func requestUserWith(r *http.Request, db *sql.DB, condition string, params ...any) (user User, err error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT * FROM users_with_enums "+condition, params...)
	err = row.Scan(&user.Id, &user.Nickname, &user.Password, &user.Role, &user.Status, &user.RegisterDate)
	if err != nil {
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

// Check whether the nickname is taken.
func ensureNicknameNotTaken(db *sql.DB, ctx context.Context, nickname string) error {
	var id int
	row := db.QueryRowContext(ctx, "SELECT id FROM users WHERE nickname = ?", nickname)
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		// Nickname is not taken, evething is fine.
		return nil
	} else if err != nil {
		return err
	}

	return BadRequestError{"This nickname is already taken by someone else :("}
}

// --------------------
// Utils.
// --------------------

// Returns whether a password is within the length limit.
func ValidatePassword(password string) error {
	const MAX_BCRYPT_LEN = 72

	ok := utf8.RuneCountInString(password) <= consts.MAX_PASSWORD_LEN &&
		len([]byte(password)) <= MAX_BCRYPT_LEN
	if !ok {
		return BadRequestError{"The password is too long."}
	} else {
		return nil
	}
}

// Returns whether a nickname is within the length limit
// and has only allowed characters.
func ValidateNickname(nickname string) (err error) {
	if utf8.RuneCountInString(nickname) > consts.MAX_NICKNAME_LEN {
		return BadRequestError{"The nickname is too long."}
	}

	for _, rune := range nickname {
		digit := rune >= '0' && rune <= '9'
		upper := rune >= 'A' && rune <= 'Z'
		lower := rune >= 'a' && rune <= 'z'
		special := rune == '_' || rune <= '-'
		if !(digit || upper || lower || special) {
			return BadRequestError{"Sorry, but nicknames can only contain A-Z, a-z, 0-9, _ and -"}
		}
	}

	return nil
}

func UserCanDeleteSnippet(user User, snippet Snippet) bool {
	return user.Role == ROLE_ADMIN || snippet.AuthorId == user.Id
}
func UserCanDeleteComment(user User, comment Comment) bool {
	return user.Role == ROLE_ADMIN || comment.AuthorId == user.Id
}
