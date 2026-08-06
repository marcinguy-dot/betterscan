#!/bin/sh
# BetterScan CLI installer
#
# Usage:
#   curl -fsSL https://github.com/betterscan-io/betterscan/raw/refs/heads/main/install.sh | sh
#   curl -fsSL https://github.com/betterscan-io/betterscan/raw/refs/heads/main/install.sh | sh -s -- -b /usr/local/bin
#   curl -fsSL https://github.com/betterscan-io/betterscan/raw/refs/heads/main/install.sh | sh -s -- -v v0.1.0
#
# Options:
#   -b DIR   Install directory (default: ~/.local/bin)
#   -v VER   Release tag to install (default: latest release, or build from source)
#   -h       Show help

set -e

OWNER="${BETTERSCAN_INSTALL_OWNER:-betterscan-io}"
REPO="${BETTERSCAN_INSTALL_REPO:-betterscan}"
HOST="${BETTERSCAN_INSTALL_HOST:-github.com}"
BINARY="${BETTERSCAN_INSTALL_BINARY:-betterscan}"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"
VERSION=""
INSTALL_DIR=""

usage() {
	cat <<EOF
BetterScan CLI installer

Usage:
  curl -fsSL https://${HOST}/${OWNER}/${REPO}/raw/branch/main/install.sh | sh
  curl -fsSL https://${HOST}/${OWNER}/${REPO}/raw/branch/main/install.sh | sh -s -- [options]

Options:
  -b DIR   Install directory (default: ${DEFAULT_INSTALL_DIR})
  -v VER   Release tag to install (default: latest)
  -h       Show this help

Environment:
  BETTERSCAN_INSTALL_OWNER   Repository owner (default: ${OWNER})
  BETTERSCAN_INSTALL_REPO    Repository name (default: ${REPO})
  BETTERSCAN_INSTALL_HOST     Forge host (default: ${HOST})
  BETTERSCAN_INSTALL_BINARY   Binary name (default: ${BINARY})
EOF
}

log() {
	printf 'betterscan-install: %s\n' "$1"
}

err() {
	printf 'betterscan-install: error: %s\n' "$1" >&2
}

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		err "required command not found: $1"
		exit 1
	fi
}

detect_os() {
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	case "$os" in
	cygwin_nt* | mingw* | msys_nt*) os="windows" ;;
	esac
	printf '%s' "$os"
}

detect_arch() {
	arch="$(uname -m)"
	case "$arch" in
	x86_64 | x64) arch="amd64" ;;
	i?86) arch="386" ;;
	aarch64 | arm64) arch="arm64" ;;
	armv7*) arch="armv7" ;;
	armv6*) arch="armv6" ;;
	esac
	printf '%s' "$arch"
}

fetch_latest_version() {
	need_cmd curl
	api_url="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
	json="$(curl -fsSL "$api_url" 2>/dev/null || true)"
	if [ -z "$json" ]; then
		return 1
	fi
	tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
	if [ -z "$tag" ]; then
		return 1
	fi
	printf '%s' "$tag"
}

download_release() {
	os="$1"
	arch="$2"
	tag="$3"
	dest="$4"

	base="https://${HOST}/${OWNER}/${REPO}/releases/download/${tag}/${BINARY}_${os}_${arch}"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	for suffix in "" ".tar.gz" ".zip"; do
		url="${base}${suffix}"
		if curl -fsSL "$url" -o "${tmp}/artifact${suffix}"; then
			case "$suffix" in
			"") cp "${tmp}/artifact" "$dest" ;;
			.tar.gz)
				need_cmd tar
				tar -xzf "${tmp}/artifact.tar.gz" -C "$tmp"
				find "$tmp" -name "$BINARY" -type f | head -n 1 | xargs -I{} cp {} "$dest"
				;;
			.zip)
				need_cmd unzip
				unzip -q "${tmp}/artifact.zip" -d "$tmp"
				find "$tmp" -name "$BINARY" -type f | head -n 1 | xargs -I{} cp {} "$dest"
				;;
			esac
			chmod 755 "$dest"
			return 0
		fi
	done

	return 1
}

build_from_source() {
	dest="$1"
	ref="${2:-main}"
	need_cmd git
	need_cmd go

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	log "building ${BINARY} from source (${HOST}/${OWNER}/${REPO}@${ref})"
	git clone --depth 1 --branch "$ref" "https://${HOST}/${OWNER}/${REPO}.git" "$tmp/src"
	(
		cd "$tmp/src/betterscan"
		GO111MODULE=on go build -ldflags="-s -w" -o "$dest" .
	)
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
		if [ -f "${HOME}/.bashrc" ]; then
			append_posix_path "${HOME}/.bashrc" "$dir"
		fi
		append_posix_path "${HOME}/.bash_profile" "$dir"
		;;
	fish)
		configure_fish_path "$dir"
		;;
	*)
		append_posix_path "${HOME}/.profile" "$dir"
		;;
	esac
}

shell_setup_hint() {
	dir="$1"
	shell_name="$(basename "${SHELL:-}")"

	case "$shell_name" in
	zsh)
		printf '%s\n' "${HOME}/.zshrc"
		;;
	bash)
		if [ -f "${HOME}/.bashrc" ]; then
			printf '%s\n' "${HOME}/.bashrc"
		else
			printf '%s\n' "${HOME}/.bash_profile"
		fi
		;;
	fish)
		printf '%s\n' "${HOME}/.config/fish/conf.d/betterscan.fish"
		;;
	*)
		printf '%s\n' "${HOME}/.profile"
		;;
	esac
}

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

OS="$(detect_os)"
ARCH="$(detect_arch)"

case "$OS" in
darwin | linux) ;;
*)
	err "unsupported operating system: ${OS}"
	exit 1
	;;
esac

case "$ARCH" in
amd64 | arm64) ;;
*)
	err "unsupported architecture: ${ARCH}"
	exit 1
	;;
esac

mkdir -p "$INSTALL_DIR"
DEST="${INSTALL_DIR%/}/${BINARY}"

log "installing ${BINARY} for ${OS}/${ARCH} to ${DEST}"

installed=0
if [ -z "$VERSION" ]; then
	VERSION="$(fetch_latest_version 2>/dev/null || true)"
fi

if [ -n "$VERSION" ]; then
	log "trying release ${VERSION}"
	if download_release "$OS" "$ARCH" "$VERSION" "$DEST"; then
		installed=1
		log "installed ${BINARY} ${VERSION} from release"
	fi
fi

if [ "$installed" -eq 0 ]; then
	if [ -n "$VERSION" ] && [ "$VERSION" != "main" ]; then
		build_from_source "$DEST" "$VERSION"
	else
		build_from_source "$DEST" "main"
	fi
	log "installed ${BINARY} from source"
fi

if [ -x "$DEST" ]; then
	if ! "$DEST" -h >/dev/null 2>&1 && ! "$DEST" --help >/dev/null 2>&1; then
		log "warning: installed binary did not respond to -h/--help (continuing anyway)"
	fi
else
	err "installation failed: ${DEST} is not executable"
	exit 1
fi

log "success"
configure_shell_path "$INSTALL_DIR"
if path_contains "$INSTALL_DIR"; then
	log "run: ${BINARY}"
else
	log "run: ${BINARY} (after restarting your shell)"
	log "or run now: export PATH=\"${INSTALL_DIR}:\$PATH\" && ${BINARY}"
	profile="$(shell_setup_hint "$INSTALL_DIR")"
	log "updated shell config: ${profile}"
fi
