// Package ratelimit обмежує частоту чутливих операцій (наприклад, /auth/login)
// незалежно для кожного ключа (IP-адреса, логін тощо).
//
// Реалізація in-memory: коректна для одного процесу DELMOS (модульний моноліт,
// ADR-001). Горизонтальне масштабування вимагатиме спільного лічильника поза цим пакетом.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	mu       sync.Mutex
	limiters map[string]*entry
	r        rate.Limit
	burst    int
	idleTTL  time.Duration
}

type entry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// New створює лімітер: r подій за секунду в усталеному режимі, burst — миттєвий запас.
func New(r rate.Limit, burst int) *Limiter {
	return &Limiter{
		limiters: make(map[string]*entry),
		r:        r,
		burst:    burst,
		idleTTL:  10 * time.Minute,
	}
}

// Allow повертає false, якщо ключ вичерпав дозволену частоту запитів.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.evictLocked()

	e, ok := l.limiters[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.r, l.burst)}
		l.limiters[key] = e
	}
	e.lastAccess = time.Now()

	return e.limiter.Allow()
}

// evictLocked видаляє записи, що довго не використовувалися, щоб мапа не росла необмежено.
func (l *Limiter) evictLocked() {
	cutoff := time.Now().Add(-l.idleTTL)
	for key, e := range l.limiters {
		if e.lastAccess.Before(cutoff) {
			delete(l.limiters, key)
		}
	}
}
