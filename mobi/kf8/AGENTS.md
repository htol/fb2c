# KF8 SKELETON GENERATION

## OVERVIEW
Kindle Format 8 skeleton for modern Kindle devices (KF8).

## STRUCTURE
```
./kf8/
├── skeleton.go      # Main KF8 skeleton generation
├── structures.go    # KF8 chunk, chapter structures
└── skeleton_test.go # Unit tests
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Skeleton generation | `skeleton.go` | GenerateKF8Skeleton, chunk building |
| KF8 structures | `structures.go` | Chunk, Fragment, Chapter types |

## CONVENTIONS
- **Joint format**: Always generated alongside MOBI 6 (set via FB2C_MOBI_TYPE=both)
- **Fragmentation**: Splits HTML into chunks for better rendering
- **Resource types**: Separate chunks for HTML, CSS, images
- **Navigation**: Maintains MOBI 6 TOC compatibility
- **Record size**: Bit mask 0x10000000 in parent MOBI header

## ANTI-PATTERNS
- Do not generate KF8-only files (always use joint format)
- Do not ignore MOBI 6 compatibility in skeleton
