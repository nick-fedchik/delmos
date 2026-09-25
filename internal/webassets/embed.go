// Package webassets serves the Vue application compiled into the DELMOS binary.
package webassets

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Files is populated by `make build`, which copies web/dist into this package.
//
//go:embed all:dist
var Files embed.FS

// Handler serves versioned static assets and falls back to the SPA entry point
// for client-side routes.
func Handler() http.Handler {
	assets, err := fs.Sub(Files, "dist")
	if err != nil {
		panic("embedded web assets are unavailable: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if assetPath == "" || !strings.Contains(path.Base(assetPath), ".") {
			http.ServeFileFS(w, r, assets, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
