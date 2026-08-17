# Testing Infrastructure Specification

Status: implemented 2026-08-17 (commits through "cleanup + docs")
Date: 2026-08-17
Scope: reference oracles, deterministic output, debug tooling, fixture corpus, git hygiene

This document records the design agreed in a planning session (Q1–Q17). It is the
source of truth for the test-infrastructure work. When the work lands, the
essence moves to `docs/DECISIONS.md` and `AGENTS.md`; this file stays as the full
specification.

## Problem

The repo had no reliable reference for tests and no real debug tooling:

- `go test ./...` was smoke tests only ("output not empty", "magic bytes at
  offset 60"), with no content-level comparison against any oracle.
- fb2c output was non-deterministic, so byte-level goldens were impossible.
- MOBI reader code lived only in throwaway scripts (`scripts/inspect_mobi.go`,
  `scripts/analyze_mobi.py`), not in the `mobi` package.
- `testdata/` was a graveyard of ~60 debug artifacts (`test_cover_v3..v26.mobi/.rawml`,
  `test_*.mobi`, extraction dirs) produced by "debug by renaming".
- The only validation pipeline (`scripts/validate.sh`) compared against Calibre,
  which breaks permanently once we support anything Calibre does not.

## Decisions

### Oracle (Q1, Q7, Q14)

- **Golden files of our own output** are the regression oracle: convert fixture,
  compare against committed golden, fail on any diff.
- **Self round-trip** through our own MOBI reader is the validity oracle: write
  MOBI, read it back, extracted content must match expectations. The reader is
  the first real reader in the `mobi` package and is shared with `fb2c dump`.
- **Zero external tools in `go test`**. No Calibre, no mobitool, nothing. The
  test suite must run on a bare Go toolchain (fresh clone, offline).
- **Calibre is out of the test loop entirely** (Q1 = b): once we implement
  anything Calibre lacks, a Calibre comparison would be permanently red.
  `scripts/` + `make validate` + `make benchmark` stay untouched as legacy
  until the new tests prove reliable; removal is deferred, not cancelled (Q14).

### Comparison layers (Q2)

Both, and content is primary:

- **Byte-wise golden**: whole generated file vs committed golden bytes.
- **Content golden**: extracted rawml (MOBI) / archive members (EPUB) as
  readable text. When a test fails, the content golden is what a human diffs.

### Determinism (Q3)

Two runs on the same input must be byte-identical.

- **MOBI**: replace `generateRandomUniqueIDSeed()` (`mobi/utils.go`) with the
  fixed value `numRecords + 1` (spec requires seed > all unique IDs). Sequence
  record unique IDs deterministically (the second observed diff at file offset
  238–241 is record-info unique IDs; make them sequential).
- **EPUB**: replace the two random UUIDs (`epub/writer.go:35–36`,
  `generateUUID` at :500) with **UUIDv5 = SHA-1(namespace + normalized
  title+authors)**: stable across conversions of the same book, distinct
  between books. Implementation via `crypto/sha1`, no external deps.
- PalmDB timestamps are already zeroed (`mobi/palmdb.go:49`) — verified.

Verified facts (2026-08-17 session):

- MOBI: two runs differed exactly in `UniqueIDSeed` (file offset 68–71) and
  record 0 unique ID (offset 238–241). Confirmed by `cmp -l`.
- EPUB: two runs differed in `dc:identifier urn:uuid:...` in `content.opf` and
  the same UUID in `toc.ncx`. Confirmed by unpacking and diffing.

### Debug tooling (Q4, Q12, Q13)

- New subcommand `fb2c dump <file>`: record listing, decoded PalmDB/MOBI/EXTH/
  INDX headers, rawml extraction.
- One internal dump model (Go structs), two serializations: human-readable text
  (default) and `--json` (Q12: both — same structure, different serializer).
- `fb2c dump --diff a.mobi b.mobi`: record-by-record comparison, first
  divergence with file offset.
- New subcommand `fb2c regen-testdata` (Q13 = c): regenerates **goldens only**;
  hand-written fixture inputs are never regenerated (see Q16).
- `fb2c dump` reads EPUB too? — decided: MOBI first; dump is reader-based and
  the reader lives in `mobi`; EPUB dumping (zip listing/OPF) can reuse the
  pattern later. Not required for the initial implementation.

### Fixture corpus (Q5, Q15, Q16, Q17)

Hand-written inputs (Q16 = a) under `testdata/fb2/`, goldens generated from
them under `testdata/golden/`. Inputs are authored, not generated: if the
generator breaks, a hand-written input exposes it instead of co-evolving with
the golden. Author: the implementing agent, based on existing seeds
(`simple.fb2`, `with_coverpage.fb2`, `test_cover.jpg`); CP1251/KOI8-R variants
are one-shot `iconv` conversions committed as static files.

Happy path (one feature per fixture, minimal size):

| Fixture | Feature |
|---|---|
| `minimal.fb2` | title + one paragraph (baseline) |
| `cover.fb2` | `<coverpage>` + base64 JPEG |
| `images.fb2` | 2–3 inline `<image l:href>` in body |
| `footnotes.fb2` | `<body name="notes">` + `<a l:href>` links |
| `encoding_cp1251.fb2` | CP1251 encoding detection |
| `encoding_koi8r.fb2` | KOI8-R encoding detection |
| `toc_deep.fb2` | 3–4 level sections, titles longer than TOC limit |
| `poetry_tables.fb2` | `<poem>`, `<stanza>`, `<table>` rendering |
| `src_ref.fb2` | official FictionBook 2.1 sample, the one realistic document |

Negative path (Q15 = b; conversion must fail with the expected error; expected
error message is itself a text golden):

| Fixture | Expected failure |
|---|---|
| `broken_xml.fb2` | truncated tag — XML parse error |
| `bad_base64.fb2` | invalid base64 in binary — decode error |
| `empty_body.fb2` | valid XML, empty body — conversion error |

### Git hygiene (Q8, Q9)

- `testdata/` **removed from `.gitignore`** and tracked fully: `testdata/fb2/`
  (inputs), `testdata/golden/mobi6/` and `testdata/golden/epub/` (binary +
  text goldens). Binary goldens in git are accepted (~KBs each).
- All scratch output goes to `./tmp/` (already ignored). AGENTS.md gets an
  explicit instruction: debug artifacts belong in `./tmp/`, never `testdata/`.
- Deleted as part of the cleanup: ~60 `test_cover_v*.mobi/.rawml`, `test_*.mobi`,
  extraction dirs in `testdata/`, and root-level `debug_records/`, `ref_records/`,
  `final_v2_records/` (all currently untracked).

### Formats in scope (Q6)

MOBI 6 uncompressed + EPUB now; KF8 later. Golden/dump infrastructure is
format-neutral (per-format golden dirs) so KF8 slots in without rework.
Compression stays off per AGENTS.md.

## Implementation plan (commits)

1. **Determinism**: `mobi/utils.go`, `mobi/palmdb.go` (record IDs), `epub/writer.go`
   (UUIDv5); new test: two conversions byte-identical.
2. **Reader + dump**: MOBI reader in package `mobi`; `cmd/fb2c`: `dump`
   (text/`--json`/`--diff`).
3. **Corpus + layout**: `testdata/fb2/` (11 fixtures), `.gitignore` update,
   `fb2c regen-testdata`.
4. **Tests**: golden tests (MOBI6 + EPUB, happy + negative), round-trip tests;
   `integration_test.go` reworked onto the new layout.
5. **Cleanup + docs**: Q8 garbage removal, AGENTS.md (testdata tracked, scratch
   → `./tmp/`, new commands, test philosophy), `docs/ARCHITECTURE.md`,
   `docs/DECISIONS.md` (this file distilled there; this file remains the spec).

## What is explicitly NOT done now

- Removing `scripts/`, `make validate`, `make benchmark` (deferred until the new
  tests prove reliable — Q14).
- KF8 goldens (Q6).
- EPUB dump mode.
