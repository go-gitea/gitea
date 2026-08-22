#!/bin/bash
set -euo pipefail

#GO=echo # uncomment this line to dry run for testing without really build all binaries

RELEASE_LDFLAGS="-s -w $LDFLAGS"
RELEASE_TAGS="bindata $TAGS"
RELEASE_PATH_PREFIX="${DIST}/binaries/gitea-${VERSION##*/}"

RELEASE_PLATFORMS_DEFAULT=(
  linux/amd64 linux/arm-5 linux/arm-6 linux/arm64 linux/riscv64 \
  windows/amd64 windows/arm64 \
  darwin/amd64 darwin/arm64 \
  freebsd/amd64
)

RELEASE_PLATFORMS_GOGIT=(windows/amd64 windows/arm64)

build() {
	local target="${1:?}"
	local variant="${2:-}"

	local goos="${target%%/*}"
	local goarch="${target##*/}"
	local goarm=""
	if [[ "$goarch" == arm-* ]]; then
		goarm="${goarch#arm-}"
		goarch="arm"
	fi

	local tags="${RELEASE_TAGS:-}"
	local suffix
	if [[ "$variant" == "gogit" ]]; then
		tags="gogit${tags:+ ${tags}}"
		suffix="-gogit-${target//\//-}"
	else
		suffix="-${target//\//-}"
	fi

	local output="${RELEASE_PATH_PREFIX}${suffix}"
	local args=()
	if [[ "$goos" == "windows" ]]; then
		output="${output}.exe"
	fi
  args+=(-tags "$tags")
  args+=(-ldflags "$RELEASE_LDFLAGS")
  # must disable CGO (host complier & linker) to get host-independent static builds
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" "${GO:-go}" build "${args[@]}" -o "$output" .
}

main() {
  local platform="${1:-}"
  for target in "${RELEASE_PLATFORMS_DEFAULT[@]}"; do
    if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
      build "$target"
    fi
  done

  for target in "${RELEASE_PLATFORMS_GOGIT[@]}"; do
    if [[ -z "$platform" || "$target" == "$platform/"* ]]; then
      build "$target" "gogit"
    fi
  done
}

main "$@"
