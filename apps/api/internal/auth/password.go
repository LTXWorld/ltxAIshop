package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordSaltLength = 16
	passwordKeyLength  = 32
	passwordMemory     = 64 * 1024
	passwordTime       = 3
	passwordThreads    = 2
)

func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}

	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		passwordMemory,
		passwordTime,
		passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password string, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}

	memory, err := parseParam(params[0], "m")
	if err != nil {
		return false
	}
	timeCost, err := parseParam(params[1], "t")
	if err != nil {
		return false
	}
	threads, err := parseParam(params[2], "p")
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	got := argon2.IDKey([]byte(password), salt, uint32(timeCost), uint32(memory), uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func parseParam(value string, name string) (int, error) {
	prefix := name + "="
	if !strings.HasPrefix(value, prefix) {
		return 0, errors.New("invalid parameter")
	}
	return strconv.Atoi(strings.TrimPrefix(value, prefix))
}
