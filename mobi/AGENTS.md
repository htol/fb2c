# MOBI GENERATION

## OVERVIEW
MOBI file generation with header, compression, indexing, and KF8 support.

## STRUCTURE
```
./mobi/
├── header.go      # MOBI 6 header (232 bytes)
├── writer.go      # Full MOBI assembly
├── compression.go # PalmDOC LZ77 compression
├── palmdb.go      # PalmDB header structure
├── exth.go        # EXTH metadata extension
├── validate.go    # MOBI validation utilities
└── kf8/           # KF8 skeleton generation
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| MOBI header | `header.go` | MOBI 6 structure, constants |
| Full assembly | `writer.go` | PalmDB + MOBI + KF8 + indexes |
| Compression | `compression.go` | PalmDOC LZ77 (type 2) |
| PalmDB header | `palmdb.go` | Database record structure |
| EXTH metadata | `exth.go` | Extended metadata fields |
| KF8 skeleton | `kf8/skeleton.go` | Modern Kindle format |

## CONVENTIONS
- **MOBI 6 + KF8**: Joint format by default (set via FB2C_MOBI_TYPE)
- **Record size**: Standard 4096 for MOBI 6; bit mask 0x10000000 for KF8
- **Encoding**: UTF-8 (65001) output only
- **Compression**: PalmDOC (type 2) applied; HuffCDIC (17480) not implemented
- **Indexes**: INDX, TAGX, INDX generated for TOC navigation

## ANTI-PATTERNS
- Do not set compression to 17480 (HuffCDIC) - not implemented
- Do not use record sizes other than 4096 for MOBI 6
