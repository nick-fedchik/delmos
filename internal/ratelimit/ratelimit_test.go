package ratelimit_test

import (
	"testing"
	"time"

	"golang.org/x/time/rate"

	"delmos/internal/ratelimit"
)

func TestAllowBlocksAfterBurstExhausted(t *testing.T) {
	limiter := ratelimit.New(rate.Every(time.Hour), 2)

	first := limiter.Allow("1.2.3.4")
	second := limiter.Allow("1.2.3.4")
	if !first || !second {
		t.Fatal("перші дві спроби в межах burst мають бути дозволені")
	}
	if limiter.Allow("1.2.3.4") {
		t.Error("третя спроба понад burst має бути заблокована")
	}
}

func TestAllowTracksKeysIndependently(t *testing.T) {
	limiter := ratelimit.New(rate.Every(time.Hour), 1)

	if !limiter.Allow("1.1.1.1") {
		t.Fatal("перша IP-адреса має отримати дозвіл")
	}
	if !limiter.Allow("2.2.2.2") {
		t.Error("інша IP-адреса не повинна залежати від лічильника першої")
	}
	if limiter.Allow("1.1.1.1") {
		t.Error("та сама IP-адреса має лишатися заблокованою в межах вікна")
	}
}
