package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword returns a bcrypt hash.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

// VerifyPassword reports whether plain matches hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// ErrWeakPassword is returned by ValidatePassword.
var ErrWeakPassword = errors.New("密码至少 8 位，且需同时包含字母和数字")

// ValidatePassword enforces the baseline policy: >= 8 chars, letters and digits.
func ValidatePassword(plain string) error {
	if len(plain) < 8 || len(plain) > 72 {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range plain {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}
