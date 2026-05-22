// Package dashboard embeds the static HTML dashboard files.
package dashboard

import _ "embed"

//go:embed index.html
var IndexHTML []byte
