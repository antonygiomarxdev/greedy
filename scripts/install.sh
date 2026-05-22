#!/bin/sh
set -eu

# Greedy installer — single static binary, AI-native via MCP
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

	add_to_path
	register_claude_desktop

	echo "Ready. Try: greedy version"
}

add_to_path() {
	if echo "$PATH" | grep -q "$INSTALL_DIR"; then
		return
	fi

	SHELL_PROFILE=""
	case "$SHELL" in
		*/zsh) SHELL_PROFILE="${HOME}/.zshrc" ;;
		*/bash) SHELL_PROFILE="${HOME}/.bashrc" ;;
		*/fish) SHELL_PROFILE="${HOME}/.config/fish/config.fish" ;;
	esac

	if [ -n "$SHELL_PROFILE" ]; then
		echo "export PATH=\"${INSTALL_DIR}:\$PATH\"" >> "$SHELL_PROFILE"
		echo "Added ${INSTALL_DIR} to PATH in ${SHELL_PROFILE}"
		export PATH="${INSTALL_DIR}:$PATH"
	else
		echo "Add ${INSTALL_DIR} to your PATH manually"
	fi
}

register_claude_desktop() {
	CLAUDE_CONFIG=""
	if [ "$(uname -s)" = "Darwin" ]; then
		CLAUDE_CONFIG="${HOME}/Library/Application Support/Claude/claude_desktop_config.json"
	else
		CLAUDE_CONFIG="${HOME}/.config/Claude/claude_desktop_config.json"
	fi

	if [ ! -f "$CLAUDE_CONFIG" ]; then
		echo "Claude Desktop config not found at ${CLAUDE_CONFIG} — skipping MCP registration"
		echo "To connect greedy to Claude manually, add this to your claude_desktop_config.json:"
		echo ""
		echo '  "mcpServers": {'
		echo '    "greedy": {'
		echo "      \"command\": \"${INSTALL_DIR}/${BINARY}\","
		echo '      "args": ["mcp-serve"]'
		echo '    }'
		echo '  }'
		echo ""
		return
	fi

	if grep -q '"greedy"' "$CLAUDE_CONFIG"; then
		echo "greedy already registered in Claude Desktop config"
		return
	fi

	TMPFILE=$(mktemp)
	python3 -c "
import json, sys
with open('$CLAUDE_CONFIG') as f:
    cfg = json.load(f)
cfg.setdefault('mcpServers', {})['greedy'] = {
    'command': '$INSTALL_DIR/$BINARY',
    'args': ['mcp-serve']
}
json.dump(cfg, sys.stdout, indent=2)
" > "$TMPFILE" 2>/dev/null && mv "$TMPFILE" "$CLAUDE_CONFIG" && echo "Registered greedy as MCP server in Claude Desktop" || {
		rm -f "$TMPFILE"
		echo "Could not auto-register in Claude Desktop config (python3 not found)"
		echo "Add manually to ${CLAUDE_CONFIG}:"
		echo ""
		echo '  "mcpServers": {'
		echo '    "greedy": {'
		echo "      \"command\": \"${INSTALL_DIR}/${BINARY}\","
		echo '      "args": ["mcp-serve"]'
		echo '    }'
		echo '  }'
		echo ""
	}
}

main "$@"
