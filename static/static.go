// Package static serves the app's embedded assets (currently one stylesheet).
package static

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"

	"sync"
)

//go:embed styles.css
var files embed.FS

// Prefix is the URL path the assets are mounted under.
const Prefix = "/static/"

var version = sync.OnceValue(func() string {
	b, err := files.ReadFile("styles.css")
	if err != nil {
		return "0"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:4])
})

// StylesURL is the stylesheet URL with a content hash, so a changed
// stylesheet is never served from a browser's cache.
func StylesURL() string {
	return string(Prefix + "styles.css?v=" + version())
}

// Handler serves the embedded assets. Mount it at Prefix.
func Handler() http.Handler {
	sub, _ := fs.Sub(files, ".")
	return http.StripPrefix(Prefix, http.FileServerFS(sub))
}
