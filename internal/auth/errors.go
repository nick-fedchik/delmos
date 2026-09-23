package auth

import "errors"

var (
	// ErrInvalidCredentials охоплює і невідомий login, і неправильний пароль,
	// щоб відповідь не розкривала, який саме факт невірний (SWR-44 §2).
	ErrInvalidCredentials = errors.New("невірний логін або пароль")
	ErrAccountInactive    = errors.New("обліковий запис деактивовано")
	ErrSessionInvalid     = errors.New("сесія недійсна або завершена")
	ErrCSRFMismatch       = errors.New("невідповідність CSRF-токена")
	ErrRateLimited        = errors.New("забагато спроб входу, спробуйте пізніше")
)
