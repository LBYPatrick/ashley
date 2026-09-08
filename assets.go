// Package ashley embeds the built-in skill source files for standalone binaries.
package ashley

import (
	"embed"
	"strings"
)

// Assets contains the same source definitions used by the Python generator.
// Custom repositories can supply an os.DirFS instead.
//
//go:embed skills/*.jsonc components res res/code/typescript/.* VERSION
var Assets embed.FS

// Version returns the release version embedded at build time.
func Version() string {
	data, _ := Assets.ReadFile("VERSION")
	return strings.TrimSpace(string(data))
}
