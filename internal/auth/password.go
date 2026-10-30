package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const passwordPrefix = "$argon2id$v=19$m=65536,t=1,p=2$"

var errPassword = errors.New("password mismatch")

// Argon2id consumes the full password, including values beyond bcrypt's limit.
// Authentication requests remain bounded by the shared JSON body limit.
func hashPassword(password string) (string, error) {
	if password == "" {
		return "", errPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 2, 32)
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func verifyPassword(hash, password string) error {
	if password == "" {
		return errPassword
	}
	if strings.HasPrefix(hash, "$2") {
		// Keep old account passwords valid, without bcrypt truncating longer input.
		if len(password) > 72 {
			return errPassword
		}
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	}
	if !strings.HasPrefix(hash, passwordPrefix) {
		return errPassword
	}
	parts := strings.Split(strings.TrimPrefix(hash, passwordPrefix), "$")
	if len(parts) != 2 {
		return errPassword
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return errPassword
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(expected) != 32 {
		return errPassword
	}
	actual := argon2.IDKey([]byte(password), salt, 1, 64*1024, 2, 32)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return errPassword
	}
	return nil
}
