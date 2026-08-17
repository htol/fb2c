#!/usr/bin/env bash
# Render the fixture corpus with Kindle Previewer (official Amazon tool using
# the same rendering engine as modern Kindle firmware). This is the closest
# automatable check to "does it open on a real Kindle"; Calibre accepting a
# file guarantees nothing there.
#
# Kindle Previewer must be installed manually from Amazon KDP (login
# required), so this target is optional: without the binary it fails with an
# install hint. On headless Linux run it under xvfb-run — the tool is a Qt
# application:
#
#   xvfb-run make preview
#
# Flags follow Amazon's documented CLI: -silent -convert -o <outdir>.
# Nonzero exit is the machine gate; Kindle Previewer also emits warnings
# (e.g. "no enhanced typesetting" for MOBI 6) into the per-file logs —
# review tmp/preview_check/*.log before trusting a green run.

set -u

WORKDIR="tmp/preview_check"
OUTDIR="$WORKDIR/out"
failed=0

if command -v kindlepreviewer > /dev/null 2>&1; then
    KP=kindlepreviewer
elif command -v kindle-preview > /dev/null 2>&1; then
    KP=kindle-preview
else
    echo "✗ Kindle Previewer not found. Install it from Amazon KDP (login required)."
    echo "  On headless Linux run this target under xvfb-run."
    exit 1
fi

if [ ! -x ./fb2c ]; then
    echo "✗ ./fb2c not found. Run: make build"
    exit 1
fi

rm -rf "$WORKDIR" && mkdir -p "$OUTDIR"
echo "Rendering fb2c output with Kindle Previewer ($KP)..."

for fb2 in testdata/fb2/*.fb2; do
    name=$(basename "$fb2" .fb2)

    # Negative fixtures must fail conversion; they are covered by go test.
    [ -f "testdata/golden/negative/$name.txt" ] && continue

    for ext in mobi epub; do
        if ! ./fb2c convert "$fb2" "$WORKDIR/$name.$ext" > /dev/null 2>&1; then
            echo "✗ $name.$ext: fb2c conversion failed"
            failed=1
            continue
        fi

        # One input file per run; output dir must already exist.
        (
            cd "$WORKDIR" && "$KP" -silent -convert -o out "$name.$ext" \
                > "$name.$ext.log" 2>&1
        )
        if [ $? -ne 0 ]; then
            echo "✗ $name.$ext: Kindle Previewer rejected the file (see $WORKDIR/$name.$ext.log)"
            failed=1
        else
            echo "✓ $name.$ext"
        fi
    done
done

if [ "$failed" -ne 0 ]; then
    echo "✗ Kindle Previewer validation failed"
    exit 1
fi

echo "✓ Kindle Previewer validation complete — still review the logs in $WORKDIR/ for warnings"
