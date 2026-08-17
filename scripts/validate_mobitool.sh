#!/usr/bin/env bash
# Validate fb2c MOBI output with mobitool (libmobi).
#
# mobitool is an independent, strict MOBI parser: it fully reconstructs the
# document (structure, INDX, resources) and exits nonzero on corrupted data.
# Calibre tolerates files that real Kindles reject; this check catches what
# Calibre would silently forgive. It is an external-tool gate, deliberately
# outside `go test` (docs/TESTING.md: no external tools in the test suite).
#
# Note: mobitool -e writes <name>.epub next to the input file and ignores -o,
# so it is always run on copies in the scratch directory, never on goldens.

set -u

WORKDIR="tmp/mobitool_check"
failed=0

if ! command -v mobitool > /dev/null 2>&1; then
    echo "✗ mobitool not found. Install with: yay -S libmobi"
    exit 1
fi

if [ ! -x ./fb2c ]; then
    echo "✗ ./fb2c not found. Run: make build"
    exit 1
fi

rm -rf "$WORKDIR" && mkdir -p "$WORKDIR"
echo "Validating fb2c MOBI output with mobitool (libmobi)..."

for fb2 in testdata/fb2/*.fb2; do
    name=$(basename "$fb2" .fb2)

    # Negative fixtures must fail conversion; they are covered by go test.
    [ -f "testdata/golden/negative/$name.txt" ] && continue

    if ! ./fb2c convert "$fb2" "$WORKDIR/$name.mobi" > /dev/null 2>&1; then
        echo "✗ $name: fb2c conversion failed"
        failed=1
        continue
    fi

    (
        cd "$WORKDIR" && mobitool -e "$name.mobi" > "$name.log" 2>&1
    )
    if [ $? -ne 0 ]; then
        echo "✗ $name: mobitool rejected the file:"
        head -5 "$WORKDIR/$name.log" | sed 's/^/    /'
        failed=1
    else
        echo "✓ $name"
    fi
done

if [ "$failed" -ne 0 ]; then
    echo "✗ mobitool validation failed"
    exit 1
fi

echo "✓ mobitool validation complete"
