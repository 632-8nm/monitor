package monitor

import "embed"

// The frontend (index.html + static/) is embedded into the binary at build
// time, making deployment a single file with no external assets to ship.
//
//go:embed web
var embeddedFS embed.FS
