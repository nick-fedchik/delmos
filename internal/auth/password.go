// Package auth реалізує локальний вхід, серверні сесії та мінімальний Scoped
// RBAC першого зрізу (docs/architecture/ACCESS_CONTROL.md, SWR-42..48).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Параметри Argon2id за рекомендацією OWASP Password Storage Cheat Sheet
// (не регламентовано нормативними документами DELMOS — свідомий вибір команди).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword повертає рядок у форматі PHC ($argon2id$v=19$m=...,t=...,p=...$сіль$хеш).
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("генерація солі: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// VerifyPassword звіряє пароль з хешем у сталий за часом спосіб.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("непідтримуваний формат хешу пароля")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("розбір версії argon2id: %w", err)
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("розбір параметрів argon2id: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("розбір солі: %w", err)
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("розбір хешу: %w", err)
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want))) //nolint:gosec // довжина want — розмір виводу argon2id з цього ж хешу, завжди мала

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// SecureToken повертає 256-бітний випадковий токен (для сесій) та його SHA-256
// у вигляді, придатному для зберігання й порівняння в БД. Хеш обчислюється від
// того самого base64-рядка, що й HashToken, — інакше пошук сесії за токеном
// із cookie ніколи не знайде щойно створений запис.
func SecureToken() (raw string, hashed []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("генерація токена: %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// RandomSecret повертає 256-бітний секрет у base64 та сирому вигляді (для CSRF, де
// відкинутий парний текст треба порівнювати з тим, що зберігається в БД).
func RandomSecret() (raw string, value []byte, err error) {
	value = make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", nil, fmt.Errorf("генерація секрету: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(value), value, nil
}

// HashToken обчислює SHA-256 отриманого від клієнта токена для пошуку в БД.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
