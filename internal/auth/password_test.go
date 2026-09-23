package auth_test

import (
	"strings"
	"testing"

	"delmos/internal/auth"
)

func TestHashAndVerifyPasswordRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("хешування: %v", err)
	}

	ok, err := auth.VerifyPassword(hash, "correct horse battery staple")
	if err != nil {
		t.Fatalf("перевірка: %v", err)
	}
	if !ok {
		t.Error("правильний пароль має проходити перевірку")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("хешування: %v", err)
	}

	ok, err := auth.VerifyPassword(hash, "wrong password")
	if err != nil {
		t.Fatalf("неочікувана помилка перевірки: %v", err)
	}
	if ok {
		t.Error("невірний пароль не повинен проходити перевірку")
	}
}

func TestHashPasswordNeverRepeatsSalt(t *testing.T) {
	first, err := auth.HashPassword("same password")
	if err != nil {
		t.Fatalf("хешування: %v", err)
	}
	second, err := auth.HashPassword("same password")
	if err != nil {
		t.Fatalf("хешування: %v", err)
	}

	if first == second {
		t.Error("однаковий пароль має давати різні хеші через випадкову сіль")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if _, err := auth.VerifyPassword("not-a-valid-hash", "anything"); err == nil {
		t.Error("некоректний формат хешу має повертати помилку, а не панікувати")
	}
}

func TestPasswordHashNeverContainsRawPassword(t *testing.T) {
	const password = "unmistakable-marker-value"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("хешування: %v", err)
	}

	if strings.Contains(hash, password) {
		t.Error("хеш не повинен містити пароль у відкритому вигляді")
	}
}
