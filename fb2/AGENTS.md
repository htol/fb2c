# FB2 PARSING & TRANSFORMATION

## OVERVIEW
FictionBook 2.0 XML parsing, metadata extraction, and HTML transformation.

## STRUCTURE
```
./fb2/
├── parser.go       # XML unmarshaling, FB2 structures
├── transformer.go  # FB2 → HTML conversion
├── toc.go          # Table of contents generation
├── metadata.go     # Metadata extraction utilities
└── parser_test.go  # Unit tests
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| XML structures | `parser.go` | FictionBook, Description, Body, Binary |
| HTML conversion | `transformer.go` | Transformer struct, CSS embedding |
| TOC generation | `toc.go` | Extract sections, build nav map |
| Metadata extraction | `metadata.go` | Title, author, publisher, genre |

## CONVENTIONS
- **Encoding**: Auto-detect UTF-8, CP1251, KOI8-R via fb2encoding package
- **HTML output**: Minimalist HTML for MOBI; richer for EPUB
- **Images**: Embedded as base64 data URLs (MOBI) or href references (EPUB)
- **CSS**: Inline styles only; no external stylesheets
- **TOC**: Generated inline by default; disable via FB2C_NO_TOC

## ANTI-PATTERNS
- Do not assume UTF-8 encoding - always detect first
- Do not generate external CSS references for MOBI
