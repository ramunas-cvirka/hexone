#!/bin/sh
set -eu

key="${1:-display}"

raw_version="${HEXONE_VERSION:-}"
if [ -z "$raw_version" ]; then
	raw_version="$(git describe --tags --dirty --always --match 'v*' 2>/dev/null || printf 'dev')"
fi

tag_version="${HEXONE_TAG_VERSION:-}"
if [ -z "$tag_version" ]; then
	tag_version="$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || printf 'v0.0.0')"
fi

semver="${HEXONE_SEMVER:-}"
if [ -z "$semver" ]; then
	semver="$(printf '%s' "$tag_version" | sed -E 's/^v//; s/[^0-9.].*$//')"
fi
if [ -z "$semver" ]; then
	semver="0.0.0"
fi

old_ifs="${IFS}"
IFS=.
set -- $semver
IFS="${old_ifs}"
major="${1:-0}"
minor="${2:-0}"
patch="${3:-0}"

build="${HEXONE_BUILD_NUMBER:-}"
if [ -z "$build" ]; then
	build=0
	case "$raw_version" in
		"$tag_version")
			build=0
			;;
		"$tag_version"-*-g*)
			rest="${raw_version#"$tag_version"-}"
			build="${rest%%-*}"
			;;
	esac
fi
case "$build" in
	''|*[!0-9]*)
		build=0
		;;
esac

commit="${HEXONE_COMMIT:-}"
if [ -z "$commit" ]; then
	commit="$(git rev-parse --short HEAD 2>/dev/null || printf 'unknown')"
fi

file_version="${major}.${minor}.${patch}.${build}"
comma_version="${major},${minor},${patch},${build}"
semver_full="${major}.${minor}.${patch}"

case "$key" in
	display)
		printf '%s\n' "$raw_version"
		;;
	semver)
		printf '%s\n' "$semver_full"
		;;
	file)
		printf '%s\n' "$file_version"
		;;
	commas)
		printf '%s\n' "$comma_version"
		;;
	commit)
		printf '%s\n' "$commit"
		;;
	tag)
		printf '%s\n' "$tag_version"
		;;
	*)
		printf 'unknown derive_version key: %s\n' "$key" >&2
		exit 1
		;;
esac
