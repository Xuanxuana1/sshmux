#!/bin/bash
set -e

REPO="Xuanxuana1/sshmux"
INSTALL_DIR="$HOME/bin"

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
  BINARY="sshmux-darwin-arm64"
else
  BINARY="sshmux-darwin-amd64"
fi

URL="https://github.com/$REPO/releases/latest/download/$BINARY"

echo "Downloading sshmux ($ARCH)..."
mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" -o "$INSTALL_DIR/sshmux"
chmod 755 "$INSTALL_DIR/sshmux"
echo "Installed to $INSTALL_DIR/sshmux"

# Add ~/bin to PATH if not already present
if [[ ":$PATH:" != *":$HOME/bin:"* ]]; then
  echo ""
  echo "Add the following line to your ~/.zshrc to make sshmux available:"
  echo "  export PATH=\"\$HOME/bin:\$PATH\""
fi

# Import SSH hosts
echo ""
echo "Importing SSH hosts from ~/.ssh/config ..."
"$INSTALL_DIR/sshmux" import-hosts 2>/dev/null && echo "SSH hosts imported." || echo "(skipped — no ~/.ssh/config found)"

echo ""
echo "Done. Run: sshmux"
