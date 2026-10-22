// Package auth содержит чистые функции авторизации: пароли, токены сессий, ограничение перебора.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Параметры argon2id (рекомендация OWASP): 19 МиБ памяти, 2 прохода, 1 поток.
const (
	argonMemory  = 19456 // КиБ
	argonTime    = 2
	argonThreads = 1
	saltLen      = 16
	keyLen       = 32
)

var errBadHash = errors.New("хеш пароля не в формате argon2id PHC")

// HashPassword возвращает хеш в формате PHC: $argon2id$v=19$m=...,t=...,p=...$соль$ключ.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("генерация соли: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword сравнивает пароль с PHC-хешем. Несовпадение — (false, nil), битый хеш — ошибка.
// Параметры берутся из самой строки, поэтому старые хеши проверяются и после смены констант.
func VerifyPassword(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", соль, ключ
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errBadHash
	}
	if memory == 0 || time == 0 || threads == 0 {
		return false, errBadHash
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, errBadHash
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
