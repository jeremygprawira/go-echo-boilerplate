#!/usr/bin/env bash
# Fails when this tree has unreachable funcs that the base tree does not.
# Usage: scripts/check-deadcode.sh [base-ref]   (default: merge base with origin/master)
set -euo pipefail
export LC_ALL=C

root=$(git rev-parse --show-toplevel)
if [ "$(git rev-parse --is-shallow-repository)" = true ]; then
	echo "deadcode: shallow clone, fetch full history to find the merge base" >&2
	exit 1
fi
base=${1:-$(git merge-base origin/master HEAD)}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

go build -modfile="$root/tools/go.mod" -o "$tmp/deadcode" golang.org/x/tools/cmd/deadcode

funcs_by_pkg_and_name() { (cd "$1" && "$tmp/deadcode" -f '{{range .Funcs}}{{$.Path}}: {{.Name}}{{"\n"}}{{end}}' ./... | sort -u); }

mkdir "$tmp/base"
git -C "$root" archive "$base" | tar -x -C "$tmp/base"
funcs_by_pkg_and_name "$tmp/base" > "$tmp/before"
funcs_by_pkg_and_name "$root" > "$tmp/after"

allow="$root/scripts/deadcode-allow.txt"
touch "$tmp/allow"
[ -f "$allow" ] && sed -e 's/#.*//' -e 's/[[:space:]]*$//' -e '/^$/d' "$allow" | sort -u > "$tmp/allow"

new=$(comm -13 "$tmp/before" "$tmp/after" | comm -23 - "$tmp/allow")
if [ -n "$new" ]; then
	echo "deadcode: new unreachable funcs versus $base:"
	echo "$new"
	echo "Call them, delete them, or list them in scripts/deadcode-allow.txt with a reason."
	exit 1
fi
echo "deadcode: no new unreachable funcs versus $base ($(wc -l < "$tmp/after" | tr -d ' ') existing)"
