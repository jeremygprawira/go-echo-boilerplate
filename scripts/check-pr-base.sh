#!/usr/bin/env bash
# Fails when a PR targets a branch other than the default branch that has no open PR.
# Usage: scripts/check-pr-base.sh <pr-number> [--at-creation]
# --at-creation replays the check as of the PR's creation time, for past PRs.
set -euo pipefail
pr=$1
read_json() { gh pr view "$pr" --json baseRefName,createdAt -q "$1"; }
base=$(read_json .baseRefName)
created=$(read_json .createdAt)
default=$(gh repo view --json defaultBranchRef -q .defaultBranchRef.name)
[ "$base" = "$default" ] && { echo "pr-base: #$pr targets $default, ok"; exit 0; }
if [ "${2:-}" = "--at-creation" ]; then
	q='[.[] | select(.createdAt <= "'"$created"'" and (.state=="OPEN" or .mergedAt > "'"$created"'"))] | length'
else
	q='[.[] | select(.state=="OPEN")] | length'
fi
live=$(gh pr list --state all --head "$base" --json state,createdAt,mergedAt -q "$q")
if [ "$live" -eq 0 ]; then
	echo "pr-base: #$pr targets $base, which has no open PR. Retarget it to $default." >&2
	exit 1
fi
echo "pr-base: #$pr is stacked on open PR head $base, ok"
