#!/bin/sh
# Build the compiled stylesheet served at /assets/styles.css.
#
# Tailwind v4 scans only the project directory, but the UI is assembled from
# jughead's daisyUI components (and the AppShell container) whose classes live
# in the Go module cache. We resolve that module's directory and emit it as an
# @source directive so every class literal in jughead's .templ/.go files is
# picked up as a candidate.
set -eu

# Resolve the jughead module directory from go.mod (works across version bumps).
jughead_dir=$(go list -m -f '{{.Dir}}' github.com/geoffjay/jughead)
if [ -z "$jughead_dir" ]; then
	echo "error: could not resolve github.com/geoffjay/jughead module dir" >&2
	exit 1
fi

printf '@source "%s/templates";\n' "$jughead_dir" > assets/jughead-sources.css
./node_modules/.bin/tailwindcss -i ./assets/styles.css -o ./internal/web/styles.css --minify
