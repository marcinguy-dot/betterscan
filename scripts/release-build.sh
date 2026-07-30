#!/bin/sh
# Build cross-platform BetterScan CLI release assets for install.sh.
set -e

ROOT="$(CDPATH= cd "$(dirname "$0")/.." && pwd)"
DIST="${ROOT}/dist"
BINARY="betterscan"
VERSION="${CI_COMMIT_TAG:-dev}"

log() {
	printf 'release-build: %s\n' "$1"
}

build_target() {
	os="$1"
	arch="$2"
	artifact="${BINARY}_${os}_${arch}.tar.gz"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	log "building ${os}/${arch} (${VERSION})"
	(
		cd "${ROOT}/betterscan"
		GO111MODULE=on CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
			go build -ldflags="-s -w" -o "${tmp}/${BINARY}" .
	)
	tar -czf "${DIST}/${artifact}" -C "$tmp" "$BINARY"
	log "wrote dist/${artifact}"
}

rm -rf "$DIST"
mkdir -p "$DIST"

build_target linux amd64
build_target linux arm64
build_target darwin amd64
build_target darwin arm64

log "done"
