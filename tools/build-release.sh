#!/bin/bash
set -euo pipefail

platform="${1:-}"
prefix="${RELEASE_PREFIX:?}"

build() {
	local target="${1:?}"
	local variant="${2:-}"
	local goos goarch goarm tags suffix output args

	goos="${target%%/*}"
	goarch="${target##*/}"
	goarm=""

	if [[ "$goarch" == arm-* ]]; then
		goarm="${goarch#arm-}"
		goarch="arm"
	fi

	tags="${RELEASE_TAGS:-}"
	if [[ "$variant" == "gogit" ]]; then
		tags="gogit${tags:+ ${tags}}"
		suffix="-gogit-${target//\//-}"
	else
		suffix="-${target//\//-}"
	fi

	output="${prefix}${suffix}"
	args=()
	if [[ "$goos" == "windows" ]]; then
		output="${output}.exe"
		args+=(-buildmode=exe)
	fi
	if [[ -n "$tags" ]]; then
		args+=(-tags "$tags")
	fi
	if [[ -n "${RELEASE_LDFLAGS:-}" ]]; then
		args+=(-ldflags "$RELEASE_LDFLAGS")
	fi

	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" "${GO:-go}" build "${args[@]}" -o "$output" .
}

for target in ${RELEASE_ARCHS:-}; do
	if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
		build "$target"
	fi
done

for target in ${RELEASE_GOGIT_ARCHS:-}; do
	if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
		build "$target" gogit
	fi
done
