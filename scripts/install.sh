#!/bin/sh
# BetterScan CLI installer (macOS, Linux, WSL)
#
# Usage (any of these):
#   curl -fsSL https://raw.githubusercontent.com/betterscan-io/betterscan/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/betterscan-io/betterscan/main/install.sh | sh
#   curl -fsSL https://github.com/betterscan-io/betterscan/raw/main/scripts/install.sh | sh
#
# Options:
#   -b DIR   Install directory (default: ~/.local/bin)
#   -v VER   Release tag (default: latest release; falls back to building main)
#   -h       Show help
#
# Environment:
#   BETTERSCAN_INSTALL_OWNER   default: betterscan-io
#   BETTERSCAN_INSTALL_REPO    default: betterscan
#   BETTERSCAN_INSTALL_HOST    default: github.com
#   BETTERSCAN_INSTALL_BINARY  default: betterscan
#   BETTERSCAN_INSTALL_GO      optional path to go binary
#   BETTERSCAN_SOURCE_DIR      monorepo package to build (default: betterscan-core)

set -eu

OWNER="${BETTERSCAN_INSTALL_OWNER:-betterscan-io}"
REPO="${BETTERSCAN_INSTALL_REPO:-betterscan}"
HOST="${BETTERSCAN_INSTALL_HOST:-github.com}"
BINARY="${BETTERSCAN_INSTALL_BINARY:-betterscan}"
SOURCE_DIR="${BETTERSCAN_SOURCE_DIR:-betterscan-core}"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"
VERSION=""
INSTALL_DIR=""
TMPDIR_INSTALL=""

usage() {
	cat <<EOF
BetterScan CLI installer (macOS / Linux / WSL)

Usage:
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/scripts/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/scripts/install.sh | sh -s -- [options]

Options:
  -b DIR   Install directory (default: ${DEFAULT_INSTALL_DIR})
  -v VER   Release tag to install (default: latest, or build from source)
  -h       Show this help

Environment:
  BETTERSCAN_INSTALL_OWNER   Repository owner (default: ${OWNER})
  BETTERSCAN_INSTALL_REPO    Repository name (default: ${REPO})
  BETTERSCAN_INSTALL_HOST    Forge host (default: ${HOST})
  BETTERSCAN_INSTALL_BINARY  Binary name (default: ${BINARY})
  BETTERSCAN_SOURCE_DIR      Source package for fallback build (default: ${SOURCE_DIR})
EOF
}

log() {
	printf 'betterscan-install: %s\n' "$1"
}

err() {
	printf 'betterscan-install: error: %s\n' "$1" >&2
}

die() {
	err "$1"
	exit 1
}

cleanup() {
	if [ -n "${TMPDIR_INSTALL}" ] && [ -d "${TMPDIR_INSTALL}" ]; then
		rm -rf "${TMPDIR_INSTALL}"
	fi
}

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		die "required command not found: $1"
	fi
}

# Portable temp dir (macOS, Linux, WSL/BusyBox-friendly)
make_tmpdir() {
	if TMPDIR_INSTALL="$(mktemp -d 2>/dev/null)"; then
		:
	elif TMPDIR_INSTALL="$(mktemp -d -t betterscan 2>/dev/null)"; then
		:
	else
		TMPDIR_INSTALL="${TMPDIR:-/tmp}/betterscan-install.$$"
		mkdir -p "$TMPDIR_INSTALL" || die "could not create temp directory"
	fi
	trap cleanup EXIT INT TERM HUP
}

detect_os() {
	os="$(uname -s 2>/dev/null || echo unknown)"
	os="$(printf '%s' "$os" | tr '[:upper:]' '[:lower:]')"
	case "$os" in
	linux*)
		# WSL reports Linux; treat as linux (same binaries)
		os="linux"
		;;
	darwin*)
		os="darwin"
		;;
	cygwin_nt* | mingw* | msys_nt*)
		os="windows"
		;;
	*)
		os="$os"
		;;
	esac
	printf '%s' "$os"
}

detect_arch() {
	arch="$(uname -m 2>/dev/null || echo unknown)"
	case "$arch" in
	x86_64 | x64 | amd64) arch="amd64" ;;
	i386 | i486 | i586 | i686 | i86pc | x86) arch="386" ;;
	aarch64 | arm64) arch="arm64" ;;
	armv7* | armv7l) arch="armv7" ;;
	armv6* | armv6l) arch="armv6" ;;
	esac
	printf '%s' "$arch"
}

is_wsl() {
	if [ -n "${WSL_DISTRO_NAME:-}" ] || [ -n "${WSL_INTEROP:-}" ]; then
		return 0
	fi
	if [ -f /proc/version ] && grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; then
		return 0
	fi
	return 1
}

# Reject HTML error pages (common when curl omits -f and hits a 404).
assert_not_html() {
	file="$1"
	if [ ! -s "$file" ]; then
		return 1
	fi
	first="$(head -c 64 "$file" 2>/dev/null || true)"
	case "$first" in
	'<!DOCTYPE'* | '<!doctype'* | '<html'* | '<HTML'* | *'404: Not Found'* )
		return 1
		;;
	esac
	return 0
}

http_get() {
	url="$1"
	out="$2"
	# Prefer curl; fall back to wget (some minimal WSL images)
	if command -v curl >/dev/null 2>&1; then
		# -f fail on HTTP errors, -L follow redirects, -S show errors with -s
		curl -fsSL --connect-timeout 30 --retry 3 --retry-delay 1 "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$out" "$url"
	else
		die "need curl or wget to download ${url}"
	fi
}

fetch_latest_version() {
	api_url="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
	json_file="${TMPDIR_INSTALL}/latest.json"
	if ! http_get "$api_url" "$json_file" 2>/dev/null; then
		return 1
	fi
	if ! assert_not_html "$json_file"; then
		return 1
	fi
	# Portable-ish JSON scrape (no jq required)
	tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$json_file" | head -n 1)"
	if [ -z "$tag" ]; then
		return 1
	fi
	printf '%s' "$tag"
}

# Try several asset naming conventions used by CI / release-build.sh
download_release() {
	os="$1"
	arch="$2"
	tag="$3"
	dest="$4"

	# Match release-build.sh (underscores + tar.gz) and GitHub Actions (hyphen targets, bare binary)
	bases="${BINARY}_${os}_${arch}
${BINARY}_${os}-${arch}
${BINARY}-${os}-${arch}"

	for base in $bases; do
		[ -n "$base" ] || continue
		for ext in tar.gz tgz zip bare; do
			case "$ext" in
			bare) url="https://${HOST}/${OWNER}/${REPO}/releases/download/${tag}/${base}" ;;
			*) url="https://${HOST}/${OWNER}/${REPO}/releases/download/${tag}/${base}.${ext}" ;;
			esac

			artifact="${TMPDIR_INSTALL}/artifact"
			rm -f "$artifact"
			if ! http_get "$url" "$artifact" 2>/dev/null; then
				continue
			fi
			if ! assert_not_html "$artifact"; then
				continue
			fi

			case "$ext" in
			bare)
				cp "$artifact" "$dest"
				;;
			tar.gz | tgz)
				need_cmd tar
				extract="${TMPDIR_INSTALL}/extract"
				rm -rf "$extract"
				mkdir -p "$extract"
				if ! tar -xzf "$artifact" -C "$extract" 2>/dev/null; then
					continue
				fi
				found="$(find "$extract" -type f -name "$BINARY" 2>/dev/null | head -n 1)"
				if [ -z "$found" ]; then
					found="$(find "$extract" -type f 2>/dev/null | head -n 1)"
				fi
				[ -n "$found" ] || continue
				cp "$found" "$dest"
				;;
			zip)
				if ! command -v unzip >/dev/null 2>&1; then
					continue
				fi
				extract="${TMPDIR_INSTALL}/extract"
				rm -rf "$extract"
				mkdir -p "$extract"
				if ! unzip -q "$artifact" -d "$extract" 2>/dev/null; then
					continue
				fi
				found="$(find "$extract" -type f -name "$BINARY" 2>/dev/null | head -n 1)"
				[ -n "$found" ] || continue
				cp "$found" "$dest"
				;;
			esac

			chmod 755 "$dest" 2>/dev/null || true
			if [ -f "$dest" ]; then
				log "downloaded ${url}"
				return 0
			fi
		done
	done
	return 1
}

build_from_source() {
	dest="$1"
	ref="${2:-main}"
	need_cmd git
	go_bin="${BETTERSCAN_INSTALL_GO:-}"
	if [ -n "$go_bin" ]; then
		:
	elif command -v go >/dev/null 2>&1; then
		go_bin="$(command -v go)"
	else
		die "Go is required to build from source (no release binary found). Install Go 1.22+ from https://go.dev/dl/ then re-run."
	fi

	src="${TMPDIR_INSTALL}/src"
	rm -rf "$src"
	log "cloning https://${HOST}/${OWNER}/${REPO}.git @ ${ref}"
	if ! git clone --depth 1 --branch "$ref" "https://${HOST}/${OWNER}/${REPO}.git" "$src" 2>/dev/null; then
		# branch might be a tag that needs full fetch, or default branch name differs
		git clone --depth 1 "https://${HOST}/${OWNER}/${REPO}.git" "$src" || die "git clone failed"
		(
			cd "$src"
			git fetch --depth 1 origin "$ref" 2>/dev/null || true
			git checkout "$ref" 2>/dev/null || git checkout "tags/${ref}" 2>/dev/null || true
		)
	fi

	build_dir=""
	for candidate in "$SOURCE_DIR" betterscan-core betterscan-cli betterscan; do
		if [ -f "${src}/${candidate}/main.go" ] || [ -f "${src}/${candidate}/go.mod" ]; then
			build_dir="${src}/${candidate}"
			break
		fi
	done
	if [ -z "$build_dir" ]; then
		if [ -f "${src}/main.go" ]; then
			build_dir="$src"
		else
			die "could not find Go package to build (looked for ${SOURCE_DIR}, betterscan-core, betterscan-cli)"
		fi
	fi

	log "building ${BINARY} from ${build_dir#"$src"/} (this may take a minute)"
	(
		cd "$build_dir"
		CGO_ENABLED=0 GO111MODULE=on "$go_bin" build -ldflags="-s -w" -o "$dest" .
	) || die "go build failed"
	chmod 755 "$dest"
}

path_contains() {
	case ":${PATH}:" in
	*":$1:"*) return 0 ;;
	*) return 1 ;;
	esac
}

profile_contains_dir() {
	file="$1"
	dir="$2"
	[ -f "$file" ] || return 1
	grep -Fq "$dir" "$file" 2>/dev/null
}

append_posix_path() {
	file="$1"
	dir="$2"
	mkdir -p "$(dirname "$file")"
	if profile_contains_dir "$file" "$dir"; then
		return 0
	fi
	{
		printf '\n# betterscan installer: ensure CLI is on PATH\n'
		printf 'export PATH="%s:$PATH"\n' "$dir"
	} >>"$file"
	log "updated ${file}"
}

configure_fish_path() {
	dir="$1"
	file="${HOME}/.config/fish/conf.d/betterscan.fish"
	mkdir -p "$(dirname "$file")"
	if [ -f "$file" ] && grep -Fq "$dir" "$file" 2>/dev/null; then
		return 0
	fi
	{
		printf '# betterscan installer: ensure CLI is on PATH\n'
		if command -v fish >/dev/null 2>&1 && fish -c 'functions fish_add_path' >/dev/null 2>&1; then
			printf 'fish_add_path -gm -- %s\n' "$dir"
		else
			printf 'set -gx PATH %s $PATH\n' "$dir"
		fi
	} >"$file"
	log "updated ${file}"
}

configure_shell_path() {
	dir="$1"
	shell_name="$(basename "${SHELL:-}")"

	if path_contains "$dir"; then
		return 0
	fi

	case "$shell_name" in
	zsh)
		append_posix_path "${HOME}/.zshrc" "$dir"
		;;
	bash)
		# WSL/Linux typically use .bashrc; macOS bash often uses .bash_profile
		if [ -f "${HOME}/.bashrc" ] || [ ! -f "${HOME}/.bash_profile" ]; then
			append_posix_path "${HOME}/.bashrc" "$dir"
		fi
		if [ -f "${HOME}/.bash_profile" ] || [ "$(uname -s 2>/dev/null)" = "Darwin" ]; then
			append_posix_path "${HOME}/.bash_profile" "$dir"
		fi
		# Always ensure login shells pick it up on Linux too
		append_posix_path "${HOME}/.profile" "$dir"
		;;
	fish)
		configure_fish_path "$dir"
		;;
	*)
		append_posix_path "${HOME}/.profile" "$dir"
		# common interactive shells
		if [ -f "${HOME}/.zshrc" ]; then
			append_posix_path "${HOME}/.zshrc" "$dir"
		fi
		if [ -f "${HOME}/.bashrc" ]; then
			append_posix_path "${HOME}/.bashrc" "$dir"
		fi
		;;
	esac
}

shell_setup_hint() {
	shell_name="$(basename "${SHELL:-}")"
	case "$shell_name" in
	zsh) printf '%s\n' "${HOME}/.zshrc" ;;
	bash)
		if [ -f "${HOME}/.bashrc" ]; then
			printf '%s\n' "${HOME}/.bashrc"
		else
			printf '%s\n' "${HOME}/.bash_profile"
		fi
		;;
	fish) printf '%s\n' "${HOME}/.config/fish/conf.d/betterscan.fish" ;;
	*) printf '%s\n' "${HOME}/.profile" ;;
	esac
}

# --- main ---

while getopts "b:v:h" opt; do
	case "$opt" in
	b) INSTALL_DIR="$OPTARG" ;;
	v) VERSION="$OPTARG" ;;
	h)
		usage
		exit 0
		;;
	*)
		usage >&2
		exit 1
		;;
	esac
done

if [ -z "$INSTALL_DIR" ]; then
	INSTALL_DIR="$DEFAULT_INSTALL_DIR"
fi

# Expand leading ~
case "$INSTALL_DIR" in
~/*) INSTALL_DIR="${HOME}/${INSTALL_DIR#~/}" ;;
~) INSTALL_DIR="${HOME}" ;;
esac

OS="$(detect_os)"
ARCH="$(detect_arch)"

case "$OS" in
darwin | linux) ;;
*)
	die "unsupported operating system: ${OS} (supported: macOS, Linux, WSL)"
	;;
esac

case "$ARCH" in
amd64 | arm64) ;;
*)
	die "unsupported architecture: ${ARCH} (supported: amd64, arm64)"
	;;
esac

make_tmpdir

platform_note="${OS}/${ARCH}"
if is_wsl; then
	platform_note="${platform_note} (WSL)"
fi

mkdir -p "$INSTALL_DIR" || die "cannot create install directory: ${INSTALL_DIR}"
DEST="${INSTALL_DIR%/}/${BINARY}"

log "installing ${BINARY} for ${platform_note} -> ${DEST}"

installed=0
if [ -z "$VERSION" ]; then
	if VERSION="$(fetch_latest_version 2>/dev/null)"; then
		log "latest release: ${VERSION}"
	else
		VERSION=""
		log "no GitHub release found; will build from source"
	fi
fi

if [ -n "$VERSION" ]; then
	log "trying release ${VERSION}"
	if download_release "$OS" "$ARCH" "$VERSION" "$DEST"; then
		installed=1
		log "installed ${BINARY} ${VERSION} from release"
	else
		log "release asset not found for ${OS}/${ARCH}; falling back to source build"
	fi
fi

if [ "$installed" -eq 0 ]; then
	ref="main"
	if [ -n "$VERSION" ]; then
		ref="$VERSION"
	fi
	build_from_source "$DEST" "$ref"
	log "installed ${BINARY} from source (${ref})"
fi

if [ ! -f "$DEST" ]; then
	die "installation failed: ${DEST} missing"
fi
chmod 755 "$DEST" 2>/dev/null || true

# Smoke check (binary may require flags; ignore non-zero)
if ! "$DEST" -h >/dev/null 2>&1 && ! "$DEST" --help >/dev/null 2>&1 && ! "$DEST" version >/dev/null 2>&1; then
	log "warning: installed binary did not respond to -h/--help/version (continuing)"
fi

log "success: ${DEST}"
configure_shell_path "$INSTALL_DIR"

if path_contains "$INSTALL_DIR"; then
	log "run: ${BINARY}"
else
	log "restart your shell, or run now:"
	log "  export PATH=\"${INSTALL_DIR}:\$PATH\" && ${BINARY}"
	profile="$(shell_setup_hint)"
	log "PATH updated in: ${profile}"
fi
