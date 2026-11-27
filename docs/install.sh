#!/bin/sh
#
# Installs bin (https://github.com/marcosnils/bin).
#
#   curl -fsSL https://marcosnils.github.io/bin/install.sh | sh
#
# Environment variables:
#   BIN_VERSION      release to install, e.g. 0.30.0 or v0.30.0 (default: latest)
#   BIN_INSTALL_DIR  directory to install into (default: first writable
#                    directory on PATH among /usr/local/bin, ~/.local/bin and
#                    ~/bin, otherwise /usr/local/bin with sudo/doas, otherwise
#                    ~/.local/bin)

set -eu

REPO="marcosnils/bin"

# Everything runs from main, called on the last line, so a truncated
# download never executes a partial script.
main() {
	need_downloader
	detect_platform

	tag=$(resolve_tag)
	version=${tag#v}
	asset="bin_${version}_${os}_${arch}${ext}"
	base="https://github.com/$REPO/releases/download/$tag"

	tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t bin-install)
	trap 'rm -rf "$tmpdir"' EXIT
	trap 'exit 1' INT TERM

	echo "Downloading bin $tag for $os/$arch"
	download "$base/$asset" "$tmpdir/$asset" ||
		fail "could not download $base/$asset
bin $tag may not have a release for $os/$arch, see https://github.com/$REPO/releases/tag/$tag"
	verify_checksum "$base/checksums.txt" "$tmpdir" "$asset"
	chmod 0755 "$tmpdir/$asset"

	choose_install_dir
	dest="$dir/bin$ext"
	# Copy next to the destination and rename it into place, so an existing
	# bin that is running is replaced instead of overwritten.
	$sudo mkdir -p "$dir"
	$sudo cp "$tmpdir/$asset" "$dest.new"
	$sudo mv -f "$dest.new" "$dest"

	echo "Installed bin $tag to $dest"
	if ! "$dest" --version >/dev/null 2>&1; then
		echo "warning: $dest does not run on this machine" >&2
	fi
	case ":$PATH:" in
	*":$dir:"*)
		echo "Run 'bin install github.com/$REPO' to let bin manage its own updates."
		;;
	*)
		echo
		echo "$dir is not on your PATH. Add it with:"
		echo "  export PATH=\"$dir:\$PATH\""
		;;
	esac
}

fail() {
	echo "error: $*" >&2
	exit 1
}

need_downloader() {
	if command -v curl >/dev/null 2>&1; then
		downloader=curl
	elif command -v wget >/dev/null 2>&1; then
		downloader=wget
	else
		fail "curl or wget is required"
	fi
}

download() {
	if [ "$downloader" = curl ]; then
		curl -fsSL -o "$2" "$1"
	else
		wget -q -O "$2" "$1"
	fi
}

detect_platform() {
	case "$(uname -s)" in
	Linux)
		os=linux
		if [ "$(uname -o 2>/dev/null)" = Android ]; then
			os=android
		fi
		;;
	Darwin) os=darwin ;;
	MINGW* | MSYS* | CYGWIN*) os=windows ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
	esac

	# A shell running under Rosetta reports x86_64 on Apple silicon.
	if [ "$os" = darwin ] && [ "$arch" = amd64 ] &&
		[ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = 1 ]; then
		arch=arm64
	fi

	ext=""
	if [ "$os" = windows ]; then
		ext=.exe
	fi
}

# Resolves the release tag from the /releases/latest redirect instead of the
# GitHub API, which rate-limits anonymous requests and returns JSON.
resolve_tag() {
	if [ -n "${BIN_VERSION:-}" ]; then
		echo "v${BIN_VERSION#v}"
		return
	fi
	url="https://github.com/$REPO/releases/latest"
	if [ "$downloader" = curl ]; then
		effective=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$url") || effective=""
	else
		effective=$(wget -S --spider "$url" 2>&1 | tr -d '\r' | sed -n 's/^ *[Ll]ocation: *\([^ ]*\).*/\1/p' | tail -n 1) || effective=""
	fi
	case "$effective" in
	*/releases/tag/*) echo "${effective##*/}" ;;
	*) fail "could not determine the latest bin release from $url" ;;
	esac
}

verify_checksum() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256="sha256sum"
	elif command -v shasum >/dev/null 2>&1; then
		sha256="shasum -a 256"
	else
		echo "warning: sha256sum or shasum not found, skipping checksum verification" >&2
		return
	fi
	download "$1" "$2/checksums.txt" || fail "could not download $1"
	expected=$(awk -v f="$3" '$2 == f { print $1 }' "$2/checksums.txt")
	[ -n "$expected" ] || fail "$3 is not listed in $1"
	actual=$(cd "$2" && $sha256 "$3" | awk '{ print $1 }')
	[ "$expected" = "$actual" ] || fail "checksum mismatch for $3: expected $expected, got $actual"
}

on_path() {
	case ":$PATH:" in
	*":$1:"*) return 0 ;;
	*) return 1 ;;
	esac
}

# A directory is usable when it already is writable, or when it does not
# exist yet and can be created.
usable() {
	if [ -d "$1" ]; then
		[ -w "$1" ]
	else
		mkdir -p "$1" 2>/dev/null
	fi
}

choose_install_dir() {
	sudo=""
	if [ -n "${BIN_INSTALL_DIR:-}" ]; then
		dir=$BIN_INSTALL_DIR
		if ! usable "$dir"; then
			find_sudo || fail "$dir is not writable"
		fi
		return
	fi
	for dir in /usr/local/bin "$HOME/.local/bin" "$HOME/bin"; do
		if on_path "$dir" && usable "$dir"; then
			return
		fi
	done
	if [ -n "${PREFIX:-}" ] && on_path "$PREFIX/bin" && usable "$PREFIX/bin"; then
		dir=$PREFIX/bin
		return
	fi
	dir=/usr/local/bin
	if on_path "$dir" && find_sudo; then
		echo "Installing to $dir requires $sudo"
		return
	fi
	dir=$HOME/.local/bin
}

find_sudo() {
	if [ "$(id -u)" = 0 ]; then
		return 1
	elif command -v sudo >/dev/null 2>&1; then
		sudo=sudo
	elif command -v doas >/dev/null 2>&1; then
		sudo=doas
	else
		return 1
	fi
}

main "$@"
