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
| OEBBook | struct | opf/book.go | OPF package metadata |
| ConvertFileWithOptions | func | converter.go | Main conversion entry |
| ConvertFile | func | converter.go | Simplified conversion wrapper |

## CONVENTIONS

- **Standard Go layout**: cmd/ for binaries, flat packages otherwise
- **Testing**: _test.go files alongside source, testdata/ for fixtures
- **Build**: Makefile with build/test/lint/validate targets
- **Encoding**: FB2 files auto-detected (UTF-8, CP1251, KOI8-R), output UTF-8
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
- **Use**: ./tmp/ for temporary artifacts
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

- **Dependencies**: Requires Go 1.25.5; external tools (Calibre, mobitool) optional for validation
