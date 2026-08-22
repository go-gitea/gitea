#!/bin/bash
set -euo pipefail

#GO=echo # dry run for testing

RELEASE_LDFLAGS="-s -w $LDFLAGS"
RELEASE_TAGS="$TAGS"
RELEASE_PATH_PREFIX="$(DIST)/binaries/gitea-$(VERSION)"

RELEASE_PLATFORMS_DEFAULT=linux/amd64 linux/386 linux/arm-5 linux/arm-6 linux/arm64 linux/riscv64 \
  windows/386 windows/amd64 windows/arm64 \
  darwin/amd64 darwin/arm64 \
  freebsd/amd64

RELEASE_PLATFORMS_GOGIT=windows/386 windows/amd64 windows/arm64

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

	output="${RELEASE_PATH_PREFIX}${suffix}"
	args=()
	if [[ "$goos" == "windows" ]]; then
		output="${output}.exe"
	fi
  args+=(-tags "$tags")
  args+=(-ldflags "$RELEASE_LDFLAGS")
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" "${GO:-go}" build "${args[@]}" -o "$output" .
}

main() {
  platform="${1:-}"
  for target in ${RELEASE_PLATFORMS_DEFAULT:-}; do
    if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
      build "$target"
    fi
  done

  for target in ${RELEASE_PLATFORMS_GOGIT:-}; do
    if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
      build "$target" "gogit"
    fi
  done
}

main "$@"
