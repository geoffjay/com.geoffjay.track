#!/bin/sh
# Build the compiled stylesheet served at /assets/styles.css.
#
# Tailwind v4 scans only the project directory, but the UI is assembled from
# templ-ui's daisyUI components (and the AppShell container) whose classes
# live in the Go module cache. Two inputs fix that:
#
#   1. templ-ui's safelist generator — `templ-ui safelist` emits the complete
#      `@source inline(...)` list for every class the components can render
#      (including runtime-composed names like btn-primary). It resolves at the
#      version pinned in go.mod, so it always matches the components in use.
#   2. An @source of templ-ui's module dir so layout utility literals in its
#      .templ files (h-screen, w-64, overflow-hidden, ...) are candidates.
set -eu

# Resolve the templ-ui module directory from go.mod (works across version bumps).
templui_dir=$(go list -m -f '{{.Dir}}' github.com/geoffjay/templ-ui)
if [ -z "$templui_dir" ]; then
	echo "error: could not resolve github.com/geoffjay/templ-ui module dir" >&2
	exit 1
fi

# Generated safelist for all runtime-composed daisyUI class names.
go run github.com/geoffjay/templ-ui/cmd/templ-ui safelist \
	-o assets/vendor/templ-ui/safelist.css

# Source-scan directives for class literals in templ-ui's templates.
printf '@source "%s/daisyui";\n@source "%s/containers";\n' \
	"$templui_dir" "$templui_dir" > assets/templ-ui-sources.css

./node_modules/.bin/tailwindcss -i ./assets/styles.css -o ./internal/web/styles.css --minify