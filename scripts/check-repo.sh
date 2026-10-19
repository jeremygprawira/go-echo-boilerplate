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

# The table had 61 rows when this floor was set. The margin lets rows go as
# their mistakes become impossible, while a lost table or block still fails.
min_rule_rows=50

# The repo's first commit. A copy without it, such as a repo made from this
# template or a shallow clone, has none of the commits the table cites.
history_anchor=3a6d57d

rule_rows() {
	awk '/^## Rules and what enforces them$/{on=1; next} /^## /{on=0} on && /^\| / && !/^\| (Rule|---) /' AGENTS.md
}

check_rule_table_present() {
	local rows
	if [ ! -r AGENTS.md ]; then
		echo "AGENTS.md is missing or unreadable, so the rule table was not checked"
		return 1
	fi
	if ! grep -q '^## Rules and what enforces them$' AGENTS.md; then
		echo "AGENTS.md has no \"## Rules and what enforces them\" heading, so the rule table was not checked"
		return 1
	fi
	rows=$(rule_rows | wc -l | tr -d ' ')
	if [ "$rows" -lt "$min_rule_rows" ]; then
		echo "the rule table in AGENTS.md has $rows rows, fewer than $min_rule_rows. Restore the rows, or lower min_rule_rows in scripts/check-repo.sh when you drop rules on purpose"
		return 1
	fi
}

check_rule_table_commits() {
	local row cell sha full rest probe n merges history=1 rc=0
	if ! git rev-parse -q --verify "$history_anchor^{commit}" >/dev/null; then
		echo "check-repo: no project history here, so the rule table's commit citations were not checked"
		history=0
	fi
	merges=$(git log --merges --format=%s)
	while IFS= read -r row; do
		cell=$(awk -F'|' '{print $(NF-1)}' <<<"$row")
		if ! grep -qE '\b[0-9a-f]{7,40}\b|PR #[0-9]+|Probe: |Mutants? M[0-9]' <<<"$cell"; then
			echo "rule table row names no past commit, PR or probe in its Fails on cell: $row"
			rc=1
		fi
		rest=$cell
		while [[ $rest == *"Probe:"* ]]; do
			rest=${rest#*Probe:}
			probe=$(sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' <<<"${rest%%Probe:*}")
			if [ "${#probe}" -lt 20 ]; then
				echo "rule table row has a Probe: that does not say what it breaks (under 20 characters): $row"
				rc=1
			fi
		done
		[ "$history" = 1 ] || continue
		for n in $(grep -oE 'PR #[0-9]+' <<<"$cell" | tr -d 'PR #'); do
			if ! grep -qE "pull request #$n( |$)" <<<"$merges"; then
				echo "rule table cites PR #$n, which no merge commit in HEAD's history names: $row"
				rc=1
			fi
		done
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

enabled_linters() {
	awk '/^[^[:space:]#]/{block=0} /^  enable:/{block=1; next} block && /^    - /{print $2; next} /^  [^ -]/{block=0}' .golangci.yml
	grep -q '^ *enabled-checks:.*ruleguard' .golangci.yml && echo ruleguard
}

check_cited_test() {
	local name=$1 command=$2 row=$3 loc file start body dir main
	loc=$(git grep -nE "^func $name\(" -- '*_test.go' | head -1)
	if [ -z "$loc" ]; then
		echo "rule table names $name, which no _test.go defines: $row"
		return 1
	fi
	file=${loc%%:*}
	start=${loc#*:}
	start=${start%%:*}
	body=$(awk -v start="$start" 'NR >= start {print} NR > start && /^}/ {exit}' "$file")
	if grep -qE '^	t\.Skip(f|Now)?\(' <<<"$body"; then
		echo "rule table names $name, which skips unconditionally: $row"
		return 1
	fi
	[ "$command" = "make test" ] || return 0
	dir=$(dirname "$file")
	main=$(git grep -l '^func TestMain(' -- ":(glob)$dir/*_test.go" | xargs grep -l TEST_DATABASE_URL /dev/null)
	if [ -n "$main" ] || grep -q TEST_DATABASE_URL <<<"$body"; then
		echo "rule table pairs $name with make test, but it skips without TEST_DATABASE_URL. Pair it with make test-integration: $row"
		return 1
	fi
}

check_rule_table_commands() {
	local row cell token word command enabled rc=0
	enabled=$(enabled_linters)
	while IFS= read -r row; do
		cell=$(awk -F'|' '{print $(NF-2)}' <<<"$row")
		if ! grep -qE '`(make |scripts/|go test|go build|Test)' <<<"$cell"; then
			echo "rule table row names no command in its Enforced by cell. Name a backticked make target, scripts/ path, go test, go build or Test func: $row"
			rc=1
		fi
		for word in $(grep -oE '\b(depguard|ruleguard|errcheck|forbidigo|importas|gofmt|gocritic)\b' <<<"$cell" | sort -u); do
			if ! grep -qx "$word" <<<"$enabled"; then
				echo "rule table names linter $word, which .golangci.yml does not enable: $row"
				rc=1
			fi
		done
		command=
		while IFS= read -r token; do
			case $token in
			"make "*)
				word=${token#make }; word=${word%% *}
				command="make $word"
				grep -qE "^$word:" Makefile || { echo "rule table names make $word, which the Makefile does not define: $row"; rc=1; } ;;
			"go "*) command=$token ;;
			*/* | .*)
				word=${token%% *}
				[ -e "$word" ] || { echo "rule table names $word, which does not exist: $row"; rc=1; } ;;
			Test* | Example*)
				check_cited_test "$token" "$command" "$row" || rc=1 ;;
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
if check_rule_table_present; then
	check_rule_table_commits || fail=1
	check_rule_table_commands || fail=1
else
	fail=1
fi
[ "$fail" = 0 ] && echo "check-repo: ok"
exit "$fail"
