#!/usr/bin/env bash
# Install the udev rules that give the user's group rw access to the Kindle
# usbfs node and the Kindle block device, so scripts/kindle.sh needs no sudo:
# the sg_start attach path opens the block node, the USBDEVFS_RESET fallback
# opens the usbfs node.
#
# Idempotent: when both rule files already match, it reports and exits without
# touching anything (and without asking for a password). Otherwise it writes
# the files and reloads udev — run `make kindle-udev` from a terminal for the
# sudo password prompt.

set -euo pipefail

USB_RULES_FILE=/etc/udev/rules.d/99-kindle.rules
USB_RULES='SUBSYSTEM=="usb", ATTR{idVendor}=="1949", MODE="0660", GROUP="plugdev"'

BLOCK_RULES_FILE=/etc/udev/rules.d/99-kindle-block.rules
BLOCK_RULES='SUBSYSTEM=="block", ATTRS{vendor}=="Kindle*", MODE="0660", GROUP="plugdev"'

GROUP=plugdev
if ! id -nG "$USER" | tr ' ' '\n' | grep -qx "$GROUP"; then
    echo "✗ $USER is not in $GROUP. Run: sudo usermod -aG $GROUP $USER — then re-login."
    exit 1
fi

if [ -f "$USB_RULES_FILE" ] && [ "$(cat "$USB_RULES_FILE")" = "$USB_RULES" ] \
   && [ -f "$BLOCK_RULES_FILE" ] && [ "$(cat "$BLOCK_RULES_FILE")" = "$BLOCK_RULES" ]; then
    echo "✓ udev rules already installed and up to date."
    exit 0
fi

if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
else
    SUDO="sudo"
fi

printf '%s\n' "$USB_RULES"   | $SUDO tee "$USB_RULES_FILE"   > /dev/null
printf '%s\n' "$BLOCK_RULES" | $SUDO tee "$BLOCK_RULES_FILE" > /dev/null
$SUDO udevadm control --reload-rules
$SUDO udevadm trigger -s usb
$SUDO udevadm trigger -s block
echo "✓ Installed:"
echo "  $USB_RULES_FILE"
echo "  $BLOCK_RULES_FILE"
