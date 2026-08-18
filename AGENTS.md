# PROJECT KNOWLEDGE BASE

## OVERVIEW

FB2 to MOBI/EPUB converter in Go. Parses FictionBook 2.0 XML and generates e-book formats with TOC, metadata, and KF8 support.

## STRUCTURE

```sh
./
├── cmd/fb2c/    # CLI entry point (main.go)
├── fb2/         # FB2 XML parsing and transformation
├── mobi/        # MOBI header, compression, indexing
│   ├── kf8/     # KF8 skeleton generation
│   └── index/   # MOBI index tables
├── opf/         # OPF metadata and NCX TOC
├── fb2encoding/ # Encoding detection (UTF-8, CP1251, KOI8-R)
├── b64/         # Base64 decoder
├── varint/      # Variable-width integer encoding
├── epub/        # EPUB format support
├── scripts/     # Validation against Calibre
└── testdata/    # Test data (src fb2 files for reference generation)
```

## WHERE TO LOOK

| Task | Location | Notes |
| ------ | ---------- | ------- |
| FB2 parsing | `fb2/parser.go` | XML unmarshaling, metadata extraction |
| HTML conversion | `fb2/transformer.go` | FB2 → HTML with CSS, TOC |
| MOBI writing | `mobi/writer.go` | Full MOBI assembly |
| KF8 skeleton | `mobi/kf8/skeleton.go` | MOBI 6 + KF8 joint format |
| Encoding detection | `fb2encoding/detect.go` | Auto-detect Russian encodings |
| CLI usage | `cmd/fb2c/main.go` | convert/metadata commands |
| Validation | `scripts/validate.sh` | Compare output with Calibre |

## CODE MAP

| Symbol | Type | Location | Role |
| -------- | ------ | ---------- | ------ |
| FictionBook | struct | fb2/parser.go | Root FB2 XML structure |
| Parser | struct | fb2/parser.go | FB2 parsing engine |
| Transformer | struct | fb2/transformer.go | FB2 → HTML converter |
| MOBIHeader | struct | mobi/header.go | MOBI 6 header (232 bytes) |
| MOBIWriter | struct | mobi/writer.go | MOBI file assembly |
| ReadDump | func | mobi/reader.go | Parse MOBI bytes into dump model |
| ExtractRawML | func | mobi/reader.go | Concatenate text records into book text |
| Diff | func | mobi/reader.go | Record-by-record comparison of two MOBI files |
| OEBBook | struct | opf/book.go | OPF package metadata |
| Converter | struct | converter.go | Conversion facade (Convert) |

## CONVENTIONS

- **Standard Go layout**: cmd/ for binaries, flat packages otherwise
- **Testing**: golden + round-trip tests via own reader, zero external tools (docs/TESTING.md); `_test.go` alongside source; fixtures in `testdata/fb2/`, goldens in `testdata/golden/`
- **Determinism**: two conversions of the same input must be byte-identical (fixed seeds/IDs, UUIDv5 for EPUB, zeroed timestamps). No randomness or wall-clock time in output
- **testdata/ is fully tracked** (inputs and goldens). Scratch and debug artifacts go to `./tmp/`, never testdata/
- **Build**: Makefile with build/test/lint/validate targets
- **Encoding**: FB2 files auto-detected (UTF-8, CP1251, KOI8-R), output UTF-8; corrupt binaries (bad base64) and empty bodies fail conversion
- **MOBI format**: Default = old MOBI 6; KF8. Use only old MOBI 6 unless explicitly instructed to use other format
- **Compression**: no compression for now. it's broken. TODO: fix
- **Debug logging**: Use slog for structured debug output; enabled via --debug/-d flag after app name; prioritize debug logging over creating debug scripts where possible

### Required Debug Implementation Pattern

```go
// 1. Pass slog.Logger to all components

// 2. Use slog for structured debug logging
import "log/slog"

    w.opts.Logger.Debug("operation details", 
        "component", "PalmDBWriter",
        "operation", "Write",
        "dataOffset", dataOffset,
        "numRecords", len(w.records),
        "bytesProcessed", totalBytes,
    )

```

### Debug Enrichment Requirements

**Mandatory debug fields to include**:

- `component`: Component name (e.g., "PalmDBWriter", "MOBIWriter", "Parser")
- `operation`: Current operation (e.g., "Write", "Parse", "Compress")
- `bytesProcessed`: Total bytes processed
- `recordCount`: Number of records/chunks
- `compressionRatio`: Compression efficiency (when applicable)

**Context-specific debug data**:

- FB2 parsing: `xmlElements`, `encoding`, `fileSize`
- MOBI writing: `headerSize`, `textRecords`, `indexEntries`
- Compression: `inputSize`, `outputSize`, `ratio`
- KF8 generation: `skeletonSize`, `htmlBlocks`

### Debug Output Format

```json
{"time":"2026-01-17T13:45:00.000Z","level":"DEBUG","msg":"MOBI file assembly","component":"MOBIWriter","operation":"Write","totalBytes":1234567,"recordCount":42}
```

## AGENT RULES

- **Never enable compression**: The compression implementation in `mobi` package is broken and produces garbage output. Always use `NoCompression` (default) for MOBI generation. Do not attempt to fix or enable `PalmDOCCompression` unless explicitly instructed to debug it *and* verification proves it works.
- **Use**: ./tmp/ for temporary artifacts; NEVER testdata/ (inputs are hand-written, goldens come from `fb2c regen-testdata` only)
- **Never**: edit goldens by hand — if output legitimately changed, run `fb2c regen-testdata`, review the diff, commit
- **Never**: add randomness, wall-clock time or unsorted map iteration to output — it breaks byte goldens
- **Never**: use 'git push' without explicit permission
- **Never**: use 'git reset --hard' without explicit permission
- **Prioritize**: mobi specification over assumptions and reverse engineering
- **Prioritize**: KF8 specification over assumptions and reverse engineering
- **Prioritize**: using make targets over direct commands
- **Explain**: decisions with facts from specifications
- **Reread**: AGENTS.md after each change
- **Update**: AGENTS.md with new significant information before commit
- **Check**: `docs/ARCHITECTURE.md` and `docs/DECISIONS.md` before starting tasks
- **Update**: `docs/ARCHITECTURE.md` and `docs/DECISIONS.md` if the task changes architecture or decisions

## ANTI-PATTERNS

- **Never**: Set compression to 17480 (HuffCDIC) - not implemented
- **Never**: Use record sizes other than 4096 for MOBI 6
- **Never**: Use fmt.Printf for debug output (use slog instead)
- **Never**: Print raw bytes without context (include size, offset, purpose)
- **Never**: Skip component/operation fields in debug logs
- **Never**: Use 'git push' without explicit permission
- **Never**: Use 'git reset --hard' without explicit permission
- **Never**: Make useless obvious comments. Comments must add value, not just repeat code. Comment why not how.

## UNIQUE STYLES

- **Encoding detection**: Russian-specific encoding detection with confidence scoring
- **Testing**: byte goldens of own output + round-trip through own reader; Calibre comparison (legacy) lives only in `scripts/`, outside `go test`
- **No external deps**: Only golang.org/x/text for encoding support
- **Structured debug**: JSON-formatted debug logs with component, operation, and metrics

## COMMANDS

```bash
make build         # Build fb2c binary
make test          # Run all tests (offline, no Calibre/mobitool)
make validate      # Legacy: validate against Calibre
make test-validate-by-mobitool  # Validate MOBI output with mobitool (independent strict parser)
make preview       # Render corpus with Kindle Previewer (closest to a real device; xvfb-run on headless)
make kindle        # Convert a fixture and deploy it to a USB-connected Kindle (kindle.sh ejects the device)
make kindle-reset  # Re-attach Kindle storage after an eject (USB reset, no conversion)
make kindle-udev   # One-time udev rules for cable-free attach/eject (sudo; idempotent)
make benchmark     # Performance comparison (fb2c vs Calibre)
make clean         # Remove build artifacts

# CLI usage with debug
fb2c [--debug|-d] convert input.fb2 output.mobi   # Convert with debug output
fb2c [--debug|-d] metadata input.fb2              # Extract metadata with debug
fb2c dump [--json] file.mobi                      # Decode PalmDB/MOBI/EXTH/INDX headers
fb2c dump --rawml file.mobi                       # Extract book text
fb2c dump --diff a.mobi b.mobi                    # Record-by-record diff (exit 1 on difference)
fb2c regen-testdata                               # Regenerate testdata/golden (never inputs)
```

## NOTES

- **Kindle USB behaviour** (verified 2026-08-18, e-ink Kindle, whole-disk FAT labelled `Kindle`): `Drive.Eject` over D-Bus (no root) makes the Kindle leave USB-storage mode — it shows the library and keeps charging while staying on the bus; its SCSI disk survives as a 0 B device with the medium removed. Re-attach = `sg_start --start --load` on that 0 B disk (SCSI load medium, NO re-enumeration; the LUN answers "Device not ready" while transitioning, so its exit code is meaningless — poll for the label). Fallback: `USBDEVFS_RESET` ioctl on the Kindle usbfs node (re-enumerates the gadget). Both need one-time udev rules granting the user's group rw on the nodes — see the `scripts/kindle.sh` header. `Block.Rescan` is NOT reliable: the firmware ignores it unpredictably (verified failing 1 of 2 identical attempts). NEVER call `Drive.PowerOff` on the Kindle: it cuts the port, the device drops off the bus (`usb X-Y: USB disconnect` in the kernel log) and only a cable re-plug recovers it. `scripts/kindle.sh` implements the cycle (attach → copy → eject); `make kindle-reset` re-attaches without converting; `KINDLE_WAIT` (default 60 s) is only the re-plug fallback
- **Dependencies**: Requires Go 1.25.5; external tools (Calibre, mobitool) optional for validation
- **Kindle app ≠ MOBI validation target** (verified 2026-08-17): current Android Kindle (8.154) accepts only PDF/EPUB sideload; old builds (8.51) upload MOBI via cloud Send-to-Kindle which the server now rejects (HTTP 400). Validate MOBI with `make test-validate-by-mobitool` and `make preview`; e-ink devices via USB are the only real-device MOBI consumers. Current Kindle app remains valid for local EPUB import tests.
