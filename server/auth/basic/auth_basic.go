package basic

// Authentication by login-password.

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tinode/chat/server/auth"
	"github.com/tinode/chat/server/store"
	"github.com/tinode/chat/server/store/types"

	"golang.org/x/crypto/bcrypt"
)

// Define default constraints on login and password
const (
	defaultMinLoginLength = 1
	defaultMaxLoginLength = 32

	defaultMinPasswordLength = 3
)

// authenticator is the type to map authentication methods to.
type authenticator struct {
	name      string
	addToTags bool

	minPasswordLength int
	minLoginLength    int
}

func (a *authenticator) checkLoginPolicy(uname string) error {
	if len([]rune(uname)) < a.minLoginLength || len([]rune(uname)) > defaultMaxLoginLength {
		return types.ErrPolicy
	}

	return nil
}

func (a *authenticator) checkPasswordPolicy(password string) error {
	if len([]rune(password)) < a.minPasswordLength {
		return types.ErrPolicy
	}

	return nil
}

func parseSecret(bsecret []byte) (uname, password string, err error) {
	secret := string(bsecret)

	splitAt := strings.Index(secret, ":")
	if splitAt < 0 {
		err = types.ErrMalformed
		return
	}

	uname = strings.ToLower(secret[:splitAt])
	password = secret[splitAt+1:]
	return
}

// Init initializes the basic authenticator.
func (a *authenticator) Init(jsonconf, name string) error {
	if a.name != "" {
		return errors.New("auth_basic: already initialized as " + a.name + "; " + name)
	}

	type configType struct {
		// AddToTags indicates that the user name should be used as a searchable tag.
		AddToTags         bool `json:"add_to_tags"`
		MinPasswordLength int  `json:"min_password_length"`
		MinLoginLength    int  `json:"min_login_length"`
	}

	var config configType
	if err := json.Unmarshal([]byte(jsonconf), &config); err != nil {
		return errors.New("auth_basic: failed to parse config: " + err.Error() + "(" + jsonconf + ")")
	}
	a.name = name
	a.addToTags = config.AddToTags
	a.minPasswordLength = config.MinPasswordLength
	if a.minPasswordLength <= 0 {
		a.minPasswordLength = defaultMinPasswordLength
	}
	a.minLoginLength = config.MinLoginLength
	if a.minLoginLength > defaultMaxLoginLength {
		return errors.New("auth_basic: min_login_length exceeds the limit")
	}
	if a.minLoginLength <= 0 {
		a.minLoginLength = defaultMinLoginLength
	}

	return nil
}

// AddRecord adds a basic authentication record to DB.
func (a *authenticator) AddRecord(rec *auth.Rec, secret []byte) (*auth.Rec, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, err
	}

	if err = a.checkLoginPolicy(uname); err != nil {
		return nil, err
	}

	if err = a.checkPasswordPolicy(password); err != nil {
		return nil, err
	}

	passhash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var expires time.Time
	if rec.Lifetime > 0 {
		expires = time.Now().Add(rec.Lifetime).UTC().Round(time.Millisecond)
	}

	authLevel := rec.AuthLevel
	if authLevel == auth.LevelNone {
		authLevel = auth.LevelAuth
	}

	dup, err := store.Users.AddAuthRecord(rec.Uid, authLevel, a.name, uname, passhash, expires)
	if dup {
		return nil, types.ErrDuplicate
	} else if err != nil {
		return nil, err
	}

	rec.AuthLevel = authLevel
	if a.addToTags {
		rec.Tags = append(rec.Tags, a.name+":"+uname)
	}
	return rec, nil
}

// UpdateRecord updates password for basic authentication.
func (a *authenticator) UpdateRecord(rec *auth.Rec, secret []byte) (*auth.Rec, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, err
	}

	login, _, _, _, err := store.Users.GetAuthRecord(rec.Uid, a.name)
	if err != nil {
		return nil, err
	}
	// User does not have a record.
	if login == "" {
		return nil, types.ErrNotFound
	}

	if uname == "" || uname == login {
		// User is changing just the password.
		uname = login
	} else if err = a.checkLoginPolicy(uname); err != nil {
		return nil, err
	}

	if err = a.checkPasswordPolicy(password); err != nil {
		return nil, err
	}

	passhash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, types.ErrInternal
	}
	var expires time.Time
	if rec.Lifetime > 0 {
		expires = types.TimeNow().Add(rec.Lifetime)
	}
	_, err = store.Users.UpdateAuthRecord(rec.Uid, auth.LevelAuth, a.name, uname, passhash, expires)
	if err != nil {
		return nil, err
	}

	// Remove old tag from the list of tags
	oldTag := a.name + ":" + login
	for i, tag := range rec.Tags {
		if tag == oldTag {
			rec.Tags[i] = rec.Tags[len(rec.Tags)-1]
			rec.Tags = rec.Tags[:len(rec.Tags)-1]
			break
		}
	}
	// Add new tag
	rec.Tags = append(rec.Tags, a.name+":"+uname)

	return rec, nil
}

// Authenticate checks login and password.
func (a *authenticator) Authenticate(secret []byte) (*auth.Rec, []byte, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	uid, authLvl, passhash, expires, err := store.Users.GetAuthUniqueRecord(a.name, uname)
	if err != nil {
		return nil, nil, err
	}
	if uid.IsZero() {
		// Invalid login.
		return nil, nil, types.ErrFailed
	}
	if !expires.IsZero() && expires.Before(time.Now()) {
		// The record has expired
		return nil, nil, types.ErrExpired
	}

	err = bcrypt.CompareHashAndPassword([]byte(passhash), []byte(password))
	if err != nil {
		// Invalid password
		return nil, nil, types.ErrFailed
	}

	var lifetime time.Duration
	if !expires.IsZero() {
		lifetime = time.Until(expires)
	}
	return &auth.Rec{
		Uid:       uid,
		AuthLevel: authLvl,
		Lifetime:  lifetime,
		Features:  0}, nil, nil
}

// IsUnique checks login uniqueness.
func (a *authenticator) IsUnique(secret []byte) (bool, error) {
	uname, _, err := parseSecret(secret)
	if err != nil {
		return false, err
	}

	if err := a.checkLoginPolicy(uname); err != nil {
		return false, err
	}

	uid, _, _, _, err := store.Users.GetAuthUniqueRecord(a.name, uname)
	if err != nil {
		return false, err
	}

	if uid.IsZero() {
		return true, nil
	}
	return false, types.ErrDuplicate
}

// GenSecret is not supported, generates an error.
func (authenticator) GenSecret(rec *auth.Rec) ([]byte, time.Time, error) {
	return nil, time.Time{}, types.ErrUnsupported
}

func (a *authenticator) DelRecords(uid types.Uid) error {
	return store.Users.DelAuthRecords(uid, a.name)
}

func init() {
	store.RegisterAuthScheme("basic", &authenticator{})
}
