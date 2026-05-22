#!/bin/sh
set -eu

# Greedy installer — single static binary, AI-native via MCP
#
# Installs the latest release, adds to PATH, and registers greedy
# as an MCP server in Claude Desktop, Cursor, and Windsurf automatically.
#
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
	register_all_mcp_tools

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

register_all_mcp_tools() {
	if ! command -v python3 >/dev/null 2>&1; then
		echo "python3 not found — skipping MCP registration"
		echo "Add this to your MCP config manually:"
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

	echo ""
	echo "Registering greedy as MCP server..."

	# Cursor
	if [ -d "${HOME}/.cursor" ]; then
		register_mcp_tool "${HOME}/.cursor/mcp.json" "Cursor"
	fi

	# Windsurf
	if [ -d "${HOME}/.windsurf" ]; then
		register_mcp_tool "${HOME}/.windsurf/mcp.json" "Windsurf"
	fi

	# Claude Desktop
	CLAUDE_CONFIG=""
	if [ "$(uname -s)" = "Darwin" ]; then
		CLAUDE_CONFIG="${HOME}/Library/Application Support/Claude/claude_desktop_config.json"
	else
		CLAUDE_CONFIG="${HOME}/.config/Claude/claude_desktop_config.json"
	fi
	if [ -f "$CLAUDE_CONFIG" ]; then
		register_mcp_tool "$CLAUDE_CONFIG" "Claude Desktop"
	fi

	# OpenCode (CLI-based registration)
	if command -v opencode >/dev/null 2>&1; then
		if opencode mcp list 2>/dev/null | grep -q greedy; then
			echo "greedy already registered in OpenCode"
		else
			opencode mcp add greedy -- "${INSTALL_DIR}/${BINARY}" mcp-serve 2>/dev/null && \
				echo "Registered greedy in OpenCode" || \
				echo "Could not register in OpenCode — run: opencode mcp add greedy -- ${INSTALL_DIR}/${BINARY} mcp-serve"
		fi
	fi

	echo ""
}

register_mcp_tool() {
	CONFIG="$1"
	TOOL="$2"

	if [ -f "$CONFIG" ] && grep -q '"greedy"' "$CONFIG" 2>/dev/null; then
		echo "  ${TOOL}: already registered"
		return
	fi

	python3 -c "
import json, os
cfg = {}
if os.path.exists('$CONFIG'):
    with open('$CONFIG') as f:
        try: cfg = json.load(f)
        except: pass
cfg.setdefault('mcpServers', {})['greedy'] = {
    'command': '$INSTALL_DIR/$BINARY',
    'args': ['mcp-serve']
}
with open('$CONFIG', 'w') as f:
    json.dump(cfg, f, indent=2)
" 2>/dev/null && echo "  ${TOOL}: registered" || \
		echo "  ${TOOL}: could not write config"
}

main "$@"
