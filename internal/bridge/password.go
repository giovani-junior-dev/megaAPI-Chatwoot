package bridge

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMem     uint32 = 64 * 1024
	argonTime    uint32 = 3
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen uint32 = 16
)

func HashPassword(pwd string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pwd), salt, argonTime, argonMem, argonThreads, argonKeyLen)
	return formatPHC(salt, key), nil
}

func formatPHC(salt, key []byte) string {
	b64 := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMem, argonTime, argonThreads, b64(salt), b64(key))
}

func VerifyPassword(pwd, phc string) (bool, error) {
	salt, key, err := parsePHC(phc)
	if err != nil {
		return false, err
	}
	cmp := argon2.IDKey([]byte(pwd), salt, argonTime, argonMem, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(cmp, key) == 1, nil
}

func parsePHC(phc string) ([]byte, []byte, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, errors.New("invalid argon2id hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, err
	}
	return salt, key, nil
}
