package handler

import (
	"log"
	"net/http"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"

	"ohmysmal/consts"
	"ohmysmal/server"
)

func (h Handler) HandleApiLogin(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.login(w, r)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}
func (h Handler) HandleApiRegister(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	err := h.register(w, r)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		Error(w, err)
		return
	}

	Redirect(w, "/")
}
func (h Handler) HandleApiLogout(w http.ResponseWriter, r *http.Request) {
	if !EnsureMethod(w, r, "POST") {
		return
	}

	h.logout(w, r)
	Redirect(w, "/")
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) (err error) {
	const INVALID_MSG = "Invalid nickname or password."

	session := h.DefaultSession(r)

	if _, ok := h.authorizedUser(session); ok {
		log.Printf("USERS: WARNING: Already authed user tried to log in, do nothing")
		return ErrUserAlreadyAuth
	}

	// Parse received form data.
	err = r.ParseForm()
	if err != nil {
		return BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	password := r.FormValue("password")

	if nickname == "" {
		return UserError{"Nickname is required."}
	} else if password == "" {
		return UserError{"Password is required."}
	} else if validateNickname(nickname) != nil || !validatePassword(password) {
		return UserError{INVALID_MSG}
	}

	// Request a user with the received nickname.
	user, err := server.RequestUserByNickname(r, h.db, nickname)
	if err == server.ErrUserNotFound {
		return UserError{INVALID_MSG}
	} else if err != nil {
		log.Printf("USERS: ERROR: Login failed: Failed to request an existing user: %s", err)
		return err
	}

	// Compare the passwords.
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Passwords aren't equal.
		return UserError{INVALID_MSG}
	}

	rememberUser(w, r, session, user.Id)

	log.Printf("USERS: INFO: User successfully logged in: %d, %s", user.Id, user.Nickname)
	return nil
}

func (h Handler) register(w http.ResponseWriter, r *http.Request) (err error) {
	session := h.DefaultSession(r)

	if _, ok := h.authorizedUser(session); ok {
		log.Printf("USERS: WARNING: Already authed user tried to register, do nothing")
		return ErrUserAlreadyAuth
	}

	// Parse received form data.
	err = r.ParseForm()
	if err != nil {
		return BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password-confirm")

	if nickname == "" {
		return UserError{"Nickname is required."}
	} else if password == "" {
		return UserError{"Password is required."}
	} else if passwordConfirm == "" {
		return UserError{"Confirm the password."}
	}

	// Validate nickname.
	err = validateNickname(nickname)
	if err == ErrNicknameTooLong {
		return UserError{"The nickname is too long."}
	} else if err != nil {
		return UserError{"Sorry, but nicknames can only contain A-Z, a-z, 0-9, _ and -"}
	}

	// Passwords validate.
	if !validatePassword(password) {
		return UserError{"The password is too long."}
	} else if password != passwordConfirm {
		return UserError{"Passwords do not match."}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return BadRequestError{err.Error()}
	}

	// Check whether the nickname is taken.
	taken, err := server.IsNicknameTaken(r, h.db, nickname)
	if err != nil {
		log.Printf("USERS: ERROR: Registration failed: Failed to check for nickname existance: %s", err)
		return err
	}
	if taken {
		return UserError{"This nickname is already taken by someone else :("}
	}

	// Insert user to the server.
	result, err := h.db.Exec("INSERT INTO users (nickname, password) VALUES (?, ?)", nickname, hashedPassword)
	if err != nil {
		log.Printf("USERS: ERROR: Registration failed: Failed to insert user data into the database: %s", err)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("USERS: ERROR: Registration failed: Failed to retrieve registered user id: %s", err)
		return err
	}

	rememberUser(w, r, session, uint(id))

	log.Printf("USERS: INFO: User successfully registered: %d, %s", id, nickname)
	return nil
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	session := h.DefaultSession(r)

	userId, found := h.authorizedUserId(session)
	if found {
		log.Printf("USERS: INFO: User logged out: %d", userId)
		_ = h.cache.Delete(fmtUserCacheKey(userId)) // ignore if not found
	}

	delete(session.Values, USER_ID_SESSION_KEY)
	_ = session.Save(r, w)
}

func (h Handler) authorizedUserId(session *sessions.Session) (id uint, found bool) {
	value := session.Values[USER_ID_SESSION_KEY]
	if value == nil {
		return 0, false
	}

	userId, ok := value.(uint)
	if !ok {
		log.Printf("USERS: ERROR: User ID in the session is not an `uint`")
		delete(session.Values, USER_ID_SESSION_KEY)
		return 0, false
	}

	return userId, true
}

// Returns the authorized user. The info is stored in the cache,
// it may be outdated but that's fine i guess.
func (h Handler) authorizedUser(session *sessions.Session) (user server.User, found bool) {
	id, found := h.authorizedUserId(session)
	if !found {
		return server.User{}, false
	}

	value, found := h.cache.Get(fmtUserCacheKey(id))
	if value == nil {
		return server.User{}, false
	}

	user, ok := value.(server.User)
	if !ok {
		log.Printf("USERS: ERROR: Stored user info in the cache is of an invalid type (%s)", reflect.TypeOf(value))
		return server.User{}, false
	} else {
		return user, ok
	}
}

func rememberUser(w http.ResponseWriter, r *http.Request, session *sessions.Session, id uint) {
	session.Values[USER_ID_SESSION_KEY] = id
	err := session.Save(r, w)
	if err != nil {
		log.Printf("USERS: ERROR: Failed to save the session: %s", err)
	}
}

// Returns whether a nickname is within the length limit
// and has only allowed characters.
func validateNickname(nickname string) (err error) {
	if utf8.RuneCountInString(nickname) > consts.MAX_NICKNAME_LEN {
		return ErrNicknameTooLong
	}

	for _, rune := range nickname {
		digit := rune >= '0' && rune <= '9'
		upper := rune >= 'A' && rune <= 'Z'
		lower := rune >= 'a' && rune <= 'z'
		special := rune == '_' || rune <= '-'
		if !(digit || upper || lower || special) {
			return ErrNicknameInvalid
		}
	}

	return nil
}

// Returns whether a password is within the length limit.
func validatePassword(password string) bool {
	const MAX_BCRYPT_LEN = 72

	return utf8.RuneCountInString(password) <= consts.MAX_PASSWORD_LEN &&
		len([]byte(password)) <= MAX_BCRYPT_LEN
}
