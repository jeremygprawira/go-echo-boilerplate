#!/usr/bin/env bash
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

check_docker_go_version() {
	local gomod line tag rc=0
	gomod=$(go mod edit -json | sed -n 's/^[[:space:]]*"Go": "\([0-9]*\.[0-9]*\).*/\1/p')
	while IFS= read -r line; do
		tag=$(sed -n 's/.*FROM golang:\([0-9]*\.[0-9]*\).*/\1/p' <<<"${line#*:*:}")
		if [ "$tag" != "$gomod" ]; then
			echo "go version drift: go.mod is go $gomod but ${line%%:*} has: ${line#*:*:}"
			rc=1
		fi
	done < <(git ls-files -z -- '*Dockerfile*' | xargs -0 grep -n 'FROM golang:' /dev/null)
	return "$rc"
}

check_no_tracked_env_config() {
	local bad
	bad=$(git ls-files 'config/config.*.yaml' | grep -v '^config/config\.local\.example\.yaml$')
	[ -z "$bad" ] && return 0
	echo "tracked env config (keep it local, it holds secrets):"
	echo "$bad"
	return 1
}

check_no_tracked_agent_files() {
	local bad
	bad=$(git ls-files '.serena' 'docs/superpowers')
	[ -z "$bad" ] && return 0
	echo "tracked agent tooling files (not project docs):"
	echo "$bad"
	return 1
}

rule_rows() {
	awk '/^## Rules and what enforces them/{on=1; next} /^## /{on=0} on && /^\| / && !/^\| (Rule|---) /' AGENTS.md
}

check_rule_table_commits() {
	local row cell sha full rc=0
	while IFS= read -r row; do
		cell=$(awk -F'|' '{print $(NF-1)}' <<<"$row")
		if ! grep -qE '\b[0-9a-f]{7,40}\b|PR #[0-9]+|Probe: |Mutants? M[0-9]' <<<"$cell"; then
			echo "rule table row names no past commit, PR or probe in its Fails on cell: $row"
			rc=1
		fi
		for sha in $(grep -oE '\b[0-9a-f]{7,40}\b' <<<"$cell"); do
			full=$(git rev-parse -q --verify "$sha^{commit}")
			if [ -z "$full" ] || ! git merge-base --is-ancestor "$full" HEAD; then
				echo "rule table cites $sha, which is not an ancestor of HEAD: $row"
				rc=1
			fi
		done
	done < <(rule_rows)
	return "$rc"
}

check_rule_table_commands() {
	local row cell token word rc=0
	while IFS= read -r row; do
		cell=$(awk -F'|' '{print $(NF-2)}' <<<"$row")
		for word in $(grep -oE '\b(depguard|ruleguard|errcheck|forbidigo|importas|gofmt|gocritic)\b' <<<"$cell"); do
			grep -qw "$word" .golangci.yml || { echo "rule table names linter $word, which .golangci.yml does not enable: $row"; rc=1; }
		done
		while IFS= read -r token; do
			case $token in
			"make "*)
				word=${token#make }; word=${word%% *}
				grep -qE "^$word:" Makefile || { echo "rule table names make $word, which the Makefile does not define: $row"; rc=1; } ;;
			"go "*) ;;
			*/* | .*)
				word=${token%% *}
				[ -e "$word" ] || { echo "rule table names $word, which does not exist: $row"; rc=1; } ;;
			Test* | Example*)
				git grep -qE "^func $token\(" -- '*_test.go' || { echo "rule table names $token, which no _test.go defines: $row"; rc=1; } ;;
			*)
				word=$(grep -oE '[A-Za-z_][A-Za-z0-9_-]*' <<<"${token%%(*}" | tail -1)
				git grep -qw -- "$word" -- '*.go' .golangci.yml || { echo "rule table names $token, which no Go file or .golangci.yml mentions: $row"; rc=1; } ;;
			esac
		done < <(grep -oE '`[^`]+`' <<<"$cell" | tr -d '`')
	done < <(rule_rows)
	return "$rc"
}

fail=0
check_docker_go_version || fail=1
check_no_tracked_env_config || fail=1
check_no_tracked_agent_files || fail=1
check_rule_table_commits || fail=1
check_rule_table_commands || fail=1
[ "$fail" = 0 ] && echo "check-repo: ok"
exit "$fail"
