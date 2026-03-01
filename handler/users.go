package handler

import (
	"log"
	"ohmysmal/database"
	"strings"
	"unicode/utf8"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h Handler) HandleApiLogin(c *gin.Context) {
	err := h.login(c)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		writeError(c, err)
		return
	}

	writeRedirect(c, "/")
}
func (h Handler) HandleApiRegister(c *gin.Context) {
	err := h.register(c)
	if err == ErrUserAlreadyAuth {
		// fallthough
	} else if err != nil {
		writeError(c, err)
		return
	}

	writeRedirect(c, "/")
}
func (h Handler) HandleApiLogout(c *gin.Context) {
	h.logout(c)
	writeRedirect(c, "/")
}

func (h Handler) login(c *gin.Context) (err error) {
	const INVALID_MSG = "Invalid nickname or password."

	session := sessions.Default(c)

	if _, ok := h.authorizedUser(); ok {
		return ErrUserAlreadyAuth
	}

	// Parse received form data.
	err = c.Request.ParseForm() // nah, i don't want to use the c.Bind()
	if err != nil {
		return BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(c.Request.FormValue("nickname"))
	password := c.Request.FormValue("password")

	if nickname == "" {
		return UserError{"Nickname is required."}
	} else if password == "" {
		return UserError{"Password is required."}
	} else if validateNickname(nickname) != nil || !validatePassword(password) {
		return UserError{INVALID_MSG}
	}

	// Request a user with the received nickname.
	user, err := database.RequestUserByNickname(h.db, nickname)
	if err == database.ErrUserNotFound {
		return UserError{INVALID_MSG}
	} else if err != nil {
		return err
	}

	// Compare the passwords.
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Passwords aren't equal.
		return UserError{INVALID_MSG}
	}

	rememberUser(session, user.Id)
	return nil
}

func (h Handler) register(c *gin.Context) (err error) {
	session := sessions.Default(c)

	if _, ok := h.authorizedUser(); ok {
		return ErrUserAlreadyAuth
	}

	// Parse received form data.
	err = c.Request.ParseForm()
	if err != nil {
		return BadRequestError{err.Error()}
	}

	nickname := strings.TrimSpace(c.Request.FormValue("nickname"))
	password := c.Request.FormValue("password")
	passwordConfirm := c.Request.FormValue("password-confirm")

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
	taken, err := database.IsNicknameTaken(h.db, nickname)
	if err != nil {
		return err
	}
	if taken {
		return UserError{"This nickname is already taken by someone else :("}
	}

	// Write user to the database.
	id, err := database.WriteUser(h.db, nickname, hashedPassword)
	if err != nil {
		return err
	}

	rememberUser(session, uint(id))

	return nil
}

func (h Handler) logout(c *gin.Context) {
	session := sessions.Default(c)

	_ = h.cache.Delete(USER_CACHE_KEY) // ignore if not found

	session.Delete(USER_ID_SESSION_KEY)
	err := session.Save()
	if err != nil {
		log.Printf("ERROR: Failed to save the session: %s", err)
	}
}

// Returns the authorized user. The info is stored in the cache,
// it may be outdated but that's fine i guess.
func (h Handler) authorizedUser() (user database.User, found bool) {
	value, found := h.cache.Get(USER_CACHE_KEY)
	user, ok := value.(database.User)
	if !ok {
		log.Printf("ERROR: Stored user info in the cache is of an invalid type")
		return database.User{}, false
	} else {
		return user, ok
	}
}

func rememberUser(session sessions.Session, id uint) {
	session.Set(USER_ID_SESSION_KEY, id)
	err := session.Save()
	if err != nil {
		log.Printf("ERROR: Failed to save the session: %s", err)
	}
}

// Returns whether a nickname is within the length limit
// and has only allowed characters.
func validateNickname(nickname string) (err error) {
	if utf8.RuneCountInString(nickname) > MAX_NICKNAME_LEN {
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

	return utf8.RuneCountInString(password) <= MAX_PASSWORD_LEN &&
		len([]byte(password)) <= MAX_BCRYPT_LEN
}
