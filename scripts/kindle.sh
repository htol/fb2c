#!/usr/bin/env bash
# Convert a fixture to MOBI, copy it to a USB-connected Kindle, then eject the
# reader so it re-indexes the library — all without ever unplugging the cable.
#
# How the no-replug cycle works (verified on a Paperwhite-era e-ink Kindle):
#
#   attach:  if the volume is absent but the Kindle is still enumerated, its
#            SCSI disk sits at 0 B with the medium "removed" (the state its
#            firmware enters on host eject). The symmetric undo is another
#            START STOP UNIT — `sg_start --start --load` (LoEj=1, Start=1:
#            load medium) — which re-exposes the medium with NO USB
#            re-enumeration (verified: same device number stays on the bus).
#            The Kindle answers "Device not ready" to it while transitioning,
#            so sg_start's exit code is meaningless — the kernel capacity
#            event (the label reappearing) is the truth, hence the poll after.
#            Fallback: USBDEVFS_RESET ioctl re-runs enumeration; the gadget
#            comes back with the medium, like a cable re-plug without the
#            cable.
#
#            Both paths need rw device nodes, normally root-owned. The
#            one-time udev rules below give them to the user's group
#            (plugdev on Arch; the user must be in it) — or just run
#            `make kindle-udev`, which installs and reloads them idempotently:
#
#              echo 'SUBSYSTEM=="usb", ATTR{idVendor}=="1949", MODE="0660", GROUP="plugdev"' \
#                  | sudo tee /etc/udev/rules.d/99-kindle.rules
#              echo 'SUBSYSTEM=="block", ATTRS{vendor}=="Kindle*", MODE="0660", GROUP="plugdev"' \
#                  | sudo tee /etc/udev/rules.d/99-kindle-block.rules
#              sudo udevadm control --reload-rules && sudo udevadm trigger -s usb && sudo udevadm trigger -s block
#
#            Without the block rule sg_start needs sudo; without the usb rule
#            the reset fallback needs sudo.
#   eject:   udisks Drive.Eject over D-Bus (SCSI START STOP with LoEj). The
#            Kindle leaves USB-storage mode, shows the library and keeps
#            charging while staying on the bus. Never call Drive.PowerOff —
#            that cuts the port and the device drops off the bus entirely
#            (kernel: "USB disconnect"), after which only a cable re-plug
#            helps.
#
# The output name is derived from the input basename, never from the clock or
# a random id: re-running the script overwrites the same file on the device
# instead of piling up library duplicates (fb2c output is byte-deterministic,
# so the overwrite is also idempotent).
#
# Usage:
#   make kindle                           # default fixture: testdata/fb2/src_ref.fb2
#   ./scripts/kindle.sh path/to/book.fb2  # any other FB2 file
#   ./scripts/kindle.sh --reset           # re-attach storage only (no conversion)
#   KINDLE_WAIT=120 ./scripts/kindle.sh   # longer wait if a re-plug is needed

set -euo pipefail

MODE=deploy
case "${1:-}" in
    --reset|-r|reset) MODE=reset; shift ;;
esac
INPUT="${1:-testdata/fb2/src_ref.fb2}"
NAME="$(basename "$INPUT" .fb2)"
OUT="tmp/kindle/$NAME.mobi"
WAIT_SECS="${KINDLE_WAIT:-60}"

UDISKS_DEST="org.freedesktop.UDisks2"

if [ ! -x ./fb2c ]; then
    echo "✗ ./fb2c not found. Run: make build"
    exit 1
fi

kindle_disk() {
    lsblk -rno NAME,LABEL | awk '$2 == "Kindle" {print "/dev/" $1; exit}'
}

# USBDEVFS_RESET (_IO('U', 20)) on the Kindle usbfs node. Finds the node via
# sysfs (busnum/devnum), so it survives device-number changes after re-plugs.
usb_reset() {
    python3 - <<'EOF'
import fcntl, glob, os, sys

USBDEVFS_RESET = 21780
for f in glob.glob('/sys/bus/usb/devices/*/idVendor'):
    dev = os.path.dirname(f)
    if open(f).read().strip() != '1949':
        continue
    bus = int(open(dev + '/busnum').read())
    num = int(open(dev + '/devnum').read())
    node = '/dev/bus/usb/%03d/%03d' % (bus, num)
    try:
        fd = os.open(node, os.O_RDWR)
    except OSError as e:
        print('no rw access to %s (%s) — install the udev rule' % (node, e),
              file=sys.stderr)
        sys.exit(1)
    fcntl.ioctl(fd, USBDEVFS_RESET)
    sys.exit(0)
print('kindle not on the usb bus', file=sys.stderr)
sys.exit(1)
EOF
}

# Re-attach the Kindle medium: SCSI load first (no re-enumeration), then the
# heavier USB reset. Returns 0 once the labelled volume is back.
attach_kindle() {
    local disk device
    if command -v sg_start > /dev/null 2>&1; then
        for disk in $(lsblk -rno NAME,TRAN,SIZE | awk '$2 == "usb" && $3 == "0B" {print $1}'); do
            sg_start --start --load "/dev/$disk" > /dev/null 2>&1 || true
        done
        for _ in $(seq 10); do
            sleep 1
            device="$(kindle_disk)"
            [ -n "$device" ] && return 0
        done
    fi
    usb_reset || return 1
    for _ in $(seq 15); do
        sleep 1
        device="$(kindle_disk)"
        [ -n "$device" ] && return 0
    done
    return 1
}

if [ "$MODE" = reset ]; then
    DEVICE="$(kindle_disk)"
    if [ -n "$DEVICE" ]; then
        echo "✓ Storage already attached: $DEVICE"
        exit 0
    fi
    if ! lsusb | grep -q 1949:; then
        echo "✗ Kindle not on the USB bus. Only a cable re-plug recovers this."
        exit 1
    fi
    echo "Kindle ejected but on the bus — re-attaching storage..."
    attach_kindle || { echo "✗ Storage did not re-attach."; exit 1; }
    DEVICE="$(kindle_disk)"
    echo "✓ Storage attached: $DEVICE"
    exit 0
fi

DEVICE="$(kindle_disk)"
if [ -z "$DEVICE" ] && lsusb | grep -q 1949:; then
    echo "Kindle ejected but on the bus — re-attaching storage..."
    attach_kindle
    DEVICE="$(kindle_disk)"
fi
if [ -z "$DEVICE" ]; then
    echo "✗ Kindle not on USB. Re-plug the cable."
    echo "  Waiting up to ${WAIT_SECS}s..."
    for _ in $(seq "$WAIT_SECS"); do
        sleep 1
        DEVICE="$(kindle_disk)"
        [ -n "$DEVICE" ] && break
    done
fi
if [ -z "$DEVICE" ]; then
    echo "✗ Kindle did not appear. Re-plug the cable and run again."
    exit 1
fi
echo "✓ Found Kindle: $DEVICE"

MOUNTPOINT="$(lsblk -rno MOUNTPOINT "$DEVICE")"
if [ -z "$MOUNTPOINT" ]; then
    udisksctl mount -b "$DEVICE" --no-user-interaction > /dev/null
    MOUNTPOINT="$(lsblk -rno MOUNTPOINT "$DEVICE")"
fi
if [ ! -d "$MOUNTPOINT/documents" ]; then
    echo "✗ $MOUNTPOINT has no documents/ — not a Kindle volume?"
    exit 1
fi

mkdir -p tmp/kindle
./fb2c convert "$INPUT" "$OUT"
echo "✓ Converted: $INPUT → $OUT"

cp "$OUT" "$MOUNTPOINT/documents/"
sync
echo "✓ Copied: $NAME.mobi → $MOUNTPOINT/documents/"

udisksctl unmount -b "$DEVICE" --no-user-interaction

# Eject the medium (not the port!) so the Kindle leaves storage mode and
# re-indexes the library while staying on the bus for the next run.
DRIVE="$(udisksctl info -b "$DEVICE" | grep -o "/org/freedesktop/UDisks2/drives/[^']*" | head -1)"
if [ -z "$DRIVE" ]; then
    echo "✗ Could not find the udisks drive object for $DEVICE"
    exit 1
fi
gdbus call --system --dest "$UDISKS_DEST" \
    --object-path "$DRIVE" \
    --method "$UDISKS_DEST.Drive.Eject" '{}' > /dev/null
echo "✓ Kindle ejected — library re-indexes on the device; cable can stay plugged."
