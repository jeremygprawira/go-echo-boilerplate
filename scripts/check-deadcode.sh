#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.."

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

allow=scripts/deadcode-allow.txt
src=$allow
[ -f "$src" ] || src=/dev/null

go build -modfile=tools/go.mod -o "$tmp/deadcode" golang.org/x/tools/cmd/deadcode
"$tmp/deadcode" -f '{{range .Funcs}}{{$.Path}}: {{.Name}}{{"\n"}}{{end}}' ./... | sort -u > "$tmp/found"
sed -e 's/#.*//' -e 's/[[:space:]]*$//' -e '/^$/d' "$src" | sort -u > "$tmp/allowed"

unlisted=$(comm -23 "$tmp/found" "$tmp/allowed")
stale=$(comm -13 "$tmp/found" "$tmp/allowed")
no_reason=$(grep -v -e '^#' -e '^[[:space:]]*$' "$src" | grep -v '#[[:space:]]*[^[:space:]]' || true)

fail=0
if [ -n "$unlisted" ]; then
	echo "deadcode: unreachable funcs that $allow does not list:"
	echo "$unlisted"
	echo "Call them, delete them, or list them with a reason."
	fail=1
fi
if [ -n "$stale" ]; then
	echo "deadcode: $allow lists funcs that are reachable or gone:"
	echo "$stale"
	fail=1
fi
if [ -n "$no_reason" ]; then
	echo "deadcode: $allow entries with no # reason:"
	echo "$no_reason"
	fail=1
fi
[ "$fail" = 0 ] && echo "deadcode: $(wc -l < "$tmp/found" | tr -d ' ') unreachable funcs, each listed with a reason"
exit "$fail"
