package static

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"sync"
)

// FS holds the static assets served at /static/ plus the PWA manifest and
// service worker. The gen_icons.go generator is excluded so it does not ship
// in the binary or appear under /static/.
//
// The woff2 files are the two typefaces app.css asks for by path. They are
// variable fonts, so one file per typeface per unicode range covers every
// weight the app sets.
//
//go:embed app.css htmx.min.js icon-192.png icon-512.png icon-512-maskable.png manifest.webmanifest sw.js
//go:embed archivo-latin.woff2 archivo-latin-ext.woff2 dm-sans-latin.woff2 dm-sans-latin-ext.woff2
var FS embed.FS

var (
	fingerprintOnce sync.Once
	fingerprints    map[string]string
)

// AssetURL returns the public path for a static asset with a short hash of its
// contents attached, so the URL changes whenever the file does.
//
// This is what keeps the stylesheet honest. Markup is served network-first while
// /static/ is cached — by the service worker and by a day of max-age — so
// without this a new build's HTML arrives alongside the previous build's CSS.
// Tailwind only emits the classes it finds in the templates, so a class added in
// the same commit as the markup using it does not exist in the older stylesheet
// at all, and the element renders unstyled rather than merely stale. A
// content-addressed URL can't be answered from either cache, so the two always
// move together.
func AssetURL(name string) string {
	fingerprintOnce.Do(loadFingerprints)
	if sum := fingerprints[name]; sum != "" {
		return "/static/" + name + "?v=" + sum
	}
	return "/static/" + name
}

func loadFingerprints() {
	fingerprints = make(map[string]string)
	entries, err := FS.ReadDir(".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		data, err := FS.ReadFile(entry.Name())
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		fingerprints[entry.Name()] = hex.EncodeToString(sum[:])[:12]
	}
}
