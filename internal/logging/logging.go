// Package logging налаштовує структуроване журналювання згідно з
// docs/operations/LOGGING_AND_OBSERVABILITY.md.
package logging

import (
	"io"
	"log/slog"
)

// New створює логер із рівнем і форматом з конфігурації. Некоректні значення
// відсікаються валідацією конфігурації, тому тут використовуються безпечні типові варіанти.
func New(level, format string, out io.Writer) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	options := &slog.HandlerOptions{Level: slogLevel}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(out, options)
	} else {
		handler = slog.NewJSONHandler(out, options)
	}

	return slog.New(handler)
}
