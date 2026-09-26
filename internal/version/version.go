// Package version зберігає єдиний номер версії продукту згідно з docs/VERSIONING.md.
package version

// Значення Commit і BuildDate підставляються через -ldflags під час збірки (див. Makefile).
var (
	Version   = "1.0.17-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String повертає рядок версії для логів, /healthz та прапорця -version.
func String() string {
	return Version + " (commit " + Commit + ", built " + BuildDate + ")"
}
