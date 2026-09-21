package web

import (
	"embed"
)

//go:embed styles.css
var stylesFS embed.FS

// stylesCSS is the compiled Tailwind/daisyUI stylesheet served at
// /assets/styles.css. Rebuild it with `npm run build` after editing
// assets/styles.css; the output lands in internal/web/ via the build script.
var stylesCSS = mustRead("styles.css")

// mustRead loads the embedded stylesheet at init.
func mustRead(name string) []byte {
	b, err := stylesFS.ReadFile(name)
	if err != nil {
		panic("embedded styles.css missing: " + err.Error())
	}
	return b
}