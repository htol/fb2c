# PROJECT KNOWLEDGE BASE

**Generated:** 2026-01-16
**Commit:** 1558348
**Branch:** main

## OVERVIEW
FB2 to MOBI/EPUB converter in Go. Parses FictionBook 2.0 XML and generates e-book formats with TOC, metadata, and KF8 support.

## STRUCTURE
```
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
└── scripts/     # Validation against Calibre
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| FB2 parsing | `fb2/parser.go` | XML unmarshaling, metadata extraction |
| HTML conversion | `fb2/transformer.go` | FB2 → HTML with CSS, TOC |
| MOBI writing | `mobi/writer.go` | Full MOBI assembly |
| KF8 skeleton | `mobi/kf8/skeleton.go` | MOBI 6 + KF8 joint format |
| Encoding detection | `fb2encoding/detect.go` | Auto-detect Russian encodings |
| CLI usage | `cmd/fb2c/main.go` | convert/metadata commands |
| Validation | `scripts/validate.sh` | Compare output with Calibre |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| FictionBook | struct | fb2/parser.go | Root FB2 XML structure |
| Parser | struct | fb2/parser.go | FB2 parsing engine |
| Transformer | struct | fb2/transformer.go | FB2 → HTML converter |
| MOBIHeader | struct | mobi/header.go | MOBI 6 header (232 bytes) |
| MOBIWriter | struct | mobi/writer.go | MOBI file assembly |
| OEBBook | struct | opf/book.go | OPF package metadata |
| ConvertFileWithOptions | func | converter.go | Main conversion entry |
| ConvertFile | func | converter.go | Simplified conversion wrapper |

## CONVENTIONS
- **Standard Go layout**: cmd/ for binaries, flat packages otherwise
- **Testing**: _test.go files alongside source, testdata/ for fixtures
- **Build**: Makefile with build/test/validate/benchmark targets
- **Encoding**: FB2 files auto-detected (UTF-8, CP1251, KOI8-R), output UTF-8
- **MOBI format**: Default = old MOBI 6; KF8 via FB2C_MOBI_TYPE=both/new
- **Compression**: PalmDOC (type 2) default; disable via FB2C_COMPRESSION=0
- **Debug logging**: Use slog for structured debug output; enabled via --debug/-d flag

## DEBUG PATTERN (MANDATORY)

**Global Debug Flag**: `--debug` or `-d` (can be placed before or after any command)

### Required Debug Implementation Pattern

```go
// 1. Accept debug flag in WriteOptions/Options struct
type WriteOptions struct {
    debug bool
    // other fields...
}

// 2. Pass debug flag through all conversion layers
func NewPalmDBWriter(name string, debug bool) *PalmDBWriter {
    // debug flag passed to all components
}

// 3. Use slog for structured debug logging
import "log/slog"

if w.debug {
    slog.Debug("operation details", 
        "component", "PalmDBWriter",
        "operation", "Write",
        "dataOffset", dataOffset,
        "numRecords", len(w.records),
        "bytesProcessed", totalBytes,
    )
}
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

## ANTI-PATTERNS (THIS PROJECT)
- **Never**: Use fmt.Printf for debug output (use slog instead)
- **Never**: Print raw bytes without context (include size, offset, purpose)
- **Never**: Skip component/operation fields in debug logs

## UNIQUE STYLES
- **Dual MOBI output**: Generates joint MOBI 6 + KF8 files for Kindle compatibility
- **Encoding detection**: Russian-specific encoding detection with confidence scoring
- **Validation pipeline**: Shell script compares fb2c output against Calibre ebook-convert
- **No external deps**: Only golang.org/x/text for encoding support
- **Structured debug**: JSON-formatted debug logs with component, operation, and metrics

## COMMANDS
```bash
make build         # Build fb2c binary
make test          # Run all tests
make validate      # Build + validate against Calibre
make benchmark     # Performance comparison (fb2c vs Calibre)
make clean         # Remove build artifacts

# CLI usage with debug
fb2c [--debug|-d] convert input.fb2 output.mobi   # Convert with debug output
fb2c [--debug|-d] metadata input.fb2               # Extract metadata with debug
```

## NOTES
- **MOBI formats**: old (MOBI 6), new (KF8 only), both (joint MOBI 6 + KF8)
- **Environment vars**: FB2C_MOBI_TYPE, FB2C_COMPRESSION, FB2C_NO_TOC
- **Dependencies**: Requires Go 1.25.5; external tools (Calibre, mobitool) optional for validation
- **Compression**: PalmDOC (LZ77) applied to text records; HuffCDIC (17480) not implemented
- **TOC**: Inline TOC generated by default; disable via FB2C_NO_TOC=1
- **Images**: Embedded as base64 data URLs in MOBI; href references in EPUB
- **Debug**: Global --debug/-d flag enables JSON-formatted structured logging with slog
