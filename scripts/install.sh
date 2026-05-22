#!/bin/sh
set -eu

# Greedy installer — single static binary, no dependencies
# Usage: curl -fsSL https://raw.githubusercontent.com/antonygiomarxdev/greedy/main/scripts/install.sh | sh

REPO="antonygiomarxdev/greedy"
DEFAULT_VERSION="latest"
INSTALL_DIR="${HOME}/.local/bin"
BINARY="greedy"

main() {
	VERSION="${1:-$DEFAULT_VERSION}"

	if [ "$VERSION" = "latest" ]; then
		VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
		if [ -z "$VERSION" ]; then
			echo "Error: could not determine latest version" >&2
			exit 1
		fi
	fi

	OS=$(uname -s | tr '[:upper:]' '[:lower:]')
	ARCH=$(uname -m)

	case "$ARCH" in
		x86_64|amd64) ARCH="amd64" ;;
		aarch64|arm64) ARCH="arm64" ;;
		*) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
	esac

	case "$OS" in
		linux) EXT="tar.gz" ;;
		darwin) EXT="tar.gz" ;;
		*) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
	esac

	ARCHIVE="greedy_${VERSION}_${OS}_${ARCH}.${EXT}"
	URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

	echo "Downloading greedy ${VERSION} for ${OS}/${ARCH}..."
	TMPDIR=$(mktemp -d)
	trap 'rm -rf "$TMPDIR"' EXIT

	curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

	if [ "$EXT" = "tar.gz" ]; then
		tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
	else
		unzip -q "${TMPDIR}/${ARCHIVE}" -d "$TMPDIR"
	fi

	mkdir -p "$INSTALL_DIR"
	install "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

	echo ""
	echo "greedy ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
	echo ""

	if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
		echo "Add ${INSTALL_DIR} to your PATH:"
		echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
		echo ""
		echo "Or add it to your shell profile (~/.bashrc, ~/.zshrc):"
		echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.bashrc"
		echo ""
	fi

	echo "Run: greedy version"
}

main "$@"
