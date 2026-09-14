// Package consoleui serves the production console embedded in the ModelCairn binary.
package consoleui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var assets embed.FS

type handler struct {
	files http.Handler
	dist  fs.FS
}

// Handler returns the immutable static-asset and SPA navigation handler.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic("embedded console is invalid: " + err.Error())
	}
	return handler{files: http.FileServer(http.FS(dist)), dist: dist}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	clean := path.Clean("/" + r.URL.Path)
	if reserved(clean) {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(clean, "/")
	if name == "" {
		h.serveIndex(w, r)
		return
	}
	if info, err := fs.Stat(h.dist, name); err == nil && !info.IsDir() {
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.files.ServeHTTP(w, r)
		return
	}
	if strings.Contains(path.Base(clean), ".") {
		http.NotFound(w, r)
		return
	}
	h.serveIndex(w, r)
}

func (h handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, err := fs.ReadFile(h.dist, "index.html")
	if err != nil {
		http.Error(w, "console unavailable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write(data)
}

func reserved(requestPath string) bool {
	for _, prefix := range []string{"/api", "/v1"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return requestPath == "/healthz" || requestPath == "/readyz"
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self'; manifest-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}
