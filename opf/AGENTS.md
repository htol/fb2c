# OPF PACKAGE GENERATION

## OVERVIEW
Open Packaging Format (OPF) metadata and NCX navigation for EPUB.

## STRUCTURE
```
./opf/
├── book.go      # OEBBook struct, metadata container
├── toc.go       # NCX navigation map generation
├── html.go      # HTML helper functions
├── metadata.go  # Metadata extraction utilities
└── book_test.go # Unit tests
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| OPF structure | `book.go` | OEBBook, metadata, manifest |
| NCX navigation | `toc.go` | buildNCXNavMap, buildNCXNavPoint |
| HTML helpers | `html.go` | Page breaks, HTML generation |
| Metadata | `metadata.go` | Dublin Core fields |

## CONVENTIONS
- **Dublin Core**: Standard metadata fields (dc:title, dc:creator, etc.)
- **NCX version**: Use 2005-1 format
- **Item IDs**: Sequential numbering (item1, item2, etc.)
- **Manifest order**: Follows reading order
- **Spine**: Linear = yes for all content files

## ANTI-PATTERNS
- Do not use NCX 3.0 (stick to 2005-1 for compatibility)
- Do not skip TOC generation in EPUB (unlike MOBI where it's optional)
