package static

import "embed"

// FS holds the static assets served at /static/ plus the PWA manifest and
// service worker. The gen_icons.go generator is excluded so it does not ship
// in the binary or appear under /static/.
//
//go:embed icon-192.png icon-512.png icon-512-maskable.png manifest.webmanifest sw.js
var FS embed.FS
