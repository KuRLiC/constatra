package webui

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed index.html
var indexHTML string

//go:embed app.css
var appCSS string

//go:embed app.js
var appJS string

// Handler serves the dependency-free embedded dashboard.
func Handler(shutdownToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(strings.ReplaceAll(indexHTML, "__SHUTDOWN_TOKEN__", shutdownToken)))
	})
	mux.HandleFunc("GET /app.css", serveAsset("text/css; charset=utf-8", appCSS))
	mux.HandleFunc("GET /app.js", serveAsset("text/javascript; charset=utf-8", appJS))
	return mux
}

func serveAsset(contentType, content string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(content))
	}
}
