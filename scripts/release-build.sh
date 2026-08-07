#!/bin/sh
# Build cross-platform BetterScan CLI release assets for install.sh and GitHub Releases.
#
# Outputs under dist/:
#   betterscan_linux_amd64.tar.gz
#   betterscan_linux_arm64.tar.gz
#   betterscan_darwin_amd64.tar.gz
#   betterscan_darwin_arm64.tar.gz
#   betterscan_windows_amd64.zip
#   betterscan_windows_arm64.zip
#   checksums.txt          (SHA-256, GNU coreutils style)
#   SHA256SUMS             (same content; common alternate name)
#
# Env:
#   VERSION     release tag or semver (default: git describe / "dev")
#   SOURCE_DIR  package to build (default: betterscan-core)
#   DIST_DIR    output directory (default: <repo>/dist)
#   BINARY      binary name (default: betterscan)

set -eu

ROOT="$(CDPATH= cd "$(dirname "$0")/.." && pwd)"
SOURCE_DIR="${SOURCE_DIR:-betterscan-core}"
DIST="${DIST_DIR:-${ROOT}/dist}"
BINARY="${BINARY:-betterscan}"
PKG="${ROOT}/${SOURCE_DIR}"

if [ -z "${VERSION:-}" ]; then
	if command -v git >/dev/null 2>&1 && [ -d "${ROOT}/.git" ]; then
		VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || true)"
	fi
fi
VERSION="${VERSION:-dev}"
# Strip leading 'v' for ldflags display consistency if needed; keep tag as-is for archives.
VERSION_LDFLAG="$VERSION"

log() {
	printf 'release-build: %s\n' "$1"
}

die() {
	printf 'release-build: error: %s\n' "$1" >&2
	exit 1
}

[ -f "${PKG}/go.mod" ] || die "missing ${SOURCE_DIR}/go.mod"
command -v go >/dev/null 2>&1 || die "go not found on PATH"

rm -rf "$DIST"
mkdir -p "$DIST"

build_unix() {
	os="$1"
	arch="$2"
	artifact="${BINARY}_${os}_${arch}.tar.gz"
	tmp="$(mktemp -d)"

	log "building ${os}/${arch} (${VERSION})"
	(
		cd "$PKG"
		CGO_ENABLED=0 GO111MODULE=on GOOS="$os" GOARCH="$arch" \
			go build -trimpath \
			-ldflags="-s -w -X main.version=${VERSION_LDFLAG}" \
			-o "${tmp}/${BINARY}" .
	) || die "go build failed for ${os}/${arch}"

	tar -czf "${DIST}/${artifact}" -C "$tmp" "$BINARY"
	rm -rf "$tmp"
	log "wrote dist/${artifact}"
}

build_windows() {
	arch="$1"
	artifact="${BINARY}_windows_${arch}.zip"
	tmp="$(mktemp -d)"
	exe="${BINARY}.exe"

	log "building windows/${arch} (${VERSION})"
	(
		cd "$PKG"
		CGO_ENABLED=0 GO111MODULE=on GOOS=windows GOARCH="$arch" \
			go build -trimpath \
			-ldflags="-s -w -X main.version=${VERSION_LDFLAG}" \
			-o "${tmp}/${exe}" .
	) || die "go build failed for windows/${arch}"

	if command -v zip >/dev/null 2>&1; then
		(cd "$tmp" && zip -q "${DIST}/${artifact}" "$exe")
	else
		# Fallback without zip(1): produce a tar.gz Windows users can still unpack
		artifact="${BINARY}_windows_${arch}.tar.gz"
		tar -czf "${DIST}/${artifact}" -C "$tmp" "$exe"
		log "zip not found; wrote dist/${artifact} instead"
		rm -rf "$tmp"
		return 0
	fi
	rm -rf "$tmp"
	log "wrote dist/${artifact}"
}

build_unix linux amd64
build_unix linux arm64
build_unix darwin amd64
build_unix darwin arm64
build_windows amd64
build_windows arm64

log "writing checksums"
(
	cd "$DIST"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum betterscan_* >checksums.txt
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 betterscan_* >checksums.txt
	else
		die "need sha256sum or shasum to write checksums"
	fi
	cp checksums.txt SHA256SUMS
)

log "done (version=${VERSION})"
ls -la "$DIST"
