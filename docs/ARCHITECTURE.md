# Architecture

`fb2c` is a pipeline-based converter that transforms FB2 (FictionBook 2.0) files into MOBI or EPUB formats. The core design philosophy is to use an intermediate representation (OPF/OEB) to decouple the input parsing from the output generation.

## High-Level Data Flow

1. **Input**: The process starts with an `.fb2` file (XML based, plain or ZIPped `.fbz`).
2. **Parsing (`fb2`)**: The file is parsed into a Go struct representation (`fb2.FictionBook`). This step handles XML unmarshalling and encoding normalization (`fb2/encoding`).
3. **Transformation (`fb2`)**: The structural FB2 data is transformed into HTML (`fb2.Transformer` + `fb2/renderer.go`), including an inline TOC and notes handling. The hierarchical TOC is extracted separately (`fb2/toc.go`).
4. **Intermediate Representation (`opf`)**: The metadata, HTML content, images, and TOC are assembled into an `opf.OEBBook` by `internal/mapper`. This structure mirrors the Open eBook (OEB) standard, which is the predecessor to both MOBI and EPUB 2.0.
5. **Output Generation**:
    * **EPUB (`epub`)**: The `OEBBook` is packaged directly into an EPUB container (ZIP structure with specific manifests).
    * **MOBI (`mobi`)**: The `OEBBook` is converted into MOBI records. This involves record building (PDB header, PalmDOC header, MOBI header), EXTH metadata, and the TOC index records (`mobi/index`). Compression is PalmDOC in theory, but currently disabled (known broken — see AGENTS.md).
    * **KF8 (`mobi/kf8`)**: KF8 (Kindle Format 8, EPUB-in-a-wrapper) writer with skeleton/flow/Fdst generation, including joint MOBI 6 + KF8 files.

Entry points: `cmd/fb2c` (CLI: `convert`, `metadata`, `dump`, `regen-testdata`) calls the `fb2c.Converter` facade (`converter.go`, package `fb2c` at repo root).

## Key Components

### `cmd/fb2c`

The command-line interface. It handles argument parsing, flag management, and orchestration of the conversion process using the `fb2c.Converter` facade. Subcommand files: `convert.go`, `metadata.go`, `dump.go`, `regen_testdata.go`.

### `converter.go` (package `fb2c`)

Facade over the whole pipeline: `Parser.ParseBytes → ExtractMetadata → Transformer.Transform → Parser.ExtractTOC → mapper.FromFB2 → write{MOBI6,KF8,Joint,EPUB}`. Output format is selected by output-file extension (`.epub` vs MOBI variants via `ConvertOptions.MobiType`).

### `fb2`

* **Parser** (`parser.go`): XML parsing, embedded-image extraction, FBZ (zipped FB2) support.
* **Transformer** (`transformer.go` + `renderer.go` + `inline.go`): FB2 → HTML, inline TOC, notes with backlinks.
* **Metadata** (`metadata.go`): extracts and normalizes book metadata.
* **TOC** (`toc.go`): flat list of `TOCEntry` with levels.

### `internal/mapper`

Translates FB2-specific structures (metadata, HTML, TOC, binaries) into the generic `opf.OEBBook`. The only package allowed to know both worlds.

### `opf`

Defines the `OEBBook` structure, which serves as the "universal" book format within the application. It contains:

* Standardized Metadata (Dublin Core)
* Manifest (list of all files)
* Spine (reading order) — currently **written but never consumed** by any writer (see Known issues)
* TOC structure

### `mobi`

Handles the complexity of the MOBI format. The authoritative format reference is `docs/MOBI6_SPECIFICATION.md`.

* **PalmDB** (`palmdb.go`): PDB container (78-byte header + record list + records).
* **Headers** (`header.go`, `exth.go`): MOBI header (232 bytes) and EXTH metadata records.
* **Writer** (`writer.go` + `writer_records.go`, `writer_toc.go`, `writer_helper.go`, `content.go`): full record layout assembly.
* **Reader** (`reader.go`, `dump.go`): parses PalmDB/MOBI/EXTH/INDX headers and records, extracts rawml, diffs two files; backs `fb2c dump` and the round-trip tests.
* **Index** (`mobi/index`): INDX/TAGX/CNCX index records for the native Kindle TOC.
* **Compression** (`compression.go`): PalmDOC LZ77 — known broken, disabled (`NoCompression`).
* **KF8** (`mobi/kf8`): skeleton chunking, flows, FDST, joint-file support.

### `epub`

A lightweight EPUB 2.0 writer that packages the `OEBBook` content into a valid EPUB zip file (mimetype/container/OPF/NCX/content/resources), plus a minimal reader (`reader.go`) rendering the archive's text members as a readable listing (the EPUB content golden).

## Testing infrastructure

Design and rationale: `docs/TESTING.md` (the full specification); decisions distilled in `docs/DECISIONS.md`.

* **Oracles**: byte goldens of our own output are the regression oracle (`golden_test.go`); self round-trip through our MOBI reader (`mobi.ExtractRawML`) is the validity oracle. Negative fixtures must fail with exactly the recorded error. No external tools in `go test` — the suite runs offline on a bare Go toolchain.
* **Corpus**: hand-written inputs in `testdata/fb2/` (one feature per fixture, plus the official FictionBook 2.1 sample and three negative fixtures); generated goldens in `testdata/golden/` (`mobi6/*.mobi+*.rawml`, `epub/*.epub+*.txt`, `negative/*.txt`).
* **Determinism** is the precondition: PalmDB `UniqueIDSeed = numRecords+1`, sequential record unique IDs, MOBI header `UniqueID` = FNV-1a(book name), EPUB identifier = UUIDv5(namespace, normalized title+authors), zeroed timestamps. Two conversions of the same input are byte-identical (guarded by `TestDeterministicOutput`).
* **Tooling**: `fb2c dump [--json|--rawml|--diff]` decodes a MOBI file via the reader; `fb2c regen-testdata` regenerates goldens only (inputs are never generated).
* **Legacy**: `scripts/validate.sh` (Calibre comparison) stays out of the test loop until the golden tests prove reliable; removal deferred, not cancelled.

## Directory Structure

* `cmd/fb2c/`: CLI entry point.
* `converter.go`: pipeline facade (package `fb2c`).
* `internal/mapper/`: FB2 → OEB translation layer.
* `epub/`: EPUB generation logic.
* `fb2/`: FB2 parsing and transformation.
* `fb2/encoding/`: character-encoding detection and conversion (CP1251, KOI8-R, …).
* `fb2/b64/`: base64 decoding for embedded images.
* `mobi/`: MOBI format generation.
* `mobi/index/`: INDX index tables.
* `mobi/kf8/`: KF8 skeleton/flow generation.
* `mobi/varint/`: variable-width integer encoding (MOBI).
* `opf/`: intermediate Open eBook representation.
* `scripts/`: validation tooling (`validate.sh` vs Calibre, `inspect_mobi.go`) — legacy, out of the test loop.
* `testdata/fb2/`: hand-written fixture inputs (tracked).
* `testdata/golden/`: generated goldens, mobi6/epub/negative (tracked).
* `tmp/`: scratch space for temporary artifacts (git-ignored).

## Known issues and technical debt

Cross-cutting findings from the 2026-08-17 review; the review's MOBI-package defect
list was worked off in the same campaign (fix commits of 2026-08-17). Fix agents:
verify each item against current code before acting.

### Architecture

1. **Writers mutate the input model.** `mobi/kf8/writer.go:111` (`w.book.Content = content`)
   rewrites the `OEBBook` handed to it. A second write of the same book (or parallel
   MOBI+EPUB output) produces corrupted output. Writers must not modify the book.
2. **Options are translated across three layers.** `cmd/fb2c/convert.go:16` builds
   `mobi.WriteOptions` → repacks into `fb2c.ConvertOptions` (`convert.go:40–45`) →
   `converter.go:126–147` unpacks back into `mobi.WriteOptions`. The CLI knows mobi-package
   internals for no reason. One options type at the top layer is enough.
3. **CLI hides engine capabilities.** `ConvertOptions` supports `MobiType`
   (old/new/both), `Title`, `Authors`, `CoverImage`; the CLI exposes none of it and hardcodes
   `MobiType: "old"` (`cmd/fb2c/convert.go:52`). The help text in `cmd/fb2c/main.go`
   ("joint MOBI 6 + KF8 by default") contradicts both the code and AGENTS.md.
4. **`OEBBook.Spine` is dead API.** No writer reads it; both outputs are built from the
   monolithic `book.Content` (`AddToSpine` has zero production callers). Either use it or
   remove it.
5. **Metadata extracted twice.** `converter.go:183` calls `fb2.ExtractMetadata`; then
   `Transformer.Transform` calls it again (`fb2/transformer.go:56`) on the same document.
   `Transformer` also duplicates output in fields and in `TransformResult`.

### Correctness (non-mobi)

6. **Three inconsistent section-ID schemes.** For sections without an explicit `id`:
   inline TOC uses per-slice indices (`fb2/transformer.go:221`), HTML anchors use a global
   counter (`fb2/renderer.go:69`), NCX/INDX TOC uses entry count (`fb2/toc.go:50`). They
   coincide only for flat books; nested sections get colliding/wrong anchors. ID generation
   must live in one place and be shared by all three consumers.
7. **EPUB cover `meta` points to a non-existent manifest item.** `epub/writer.go:202`
   writes `content="cover-<CoverID>"` but items are emitted with `id="res-<id>"`
   (`epub/writer.go:232`). Readers cannot find the cover.
8. **EPUB navPoints all lead to the top of the document.** `convertToXHTML`
   (`epub/writer.go:~430–445`) injects all `toc-N` anchor spans *before* the body content.
   Fix by placing anchors at the actual section positions.
9. **`fb2.Parser` state leaks between books.** The parser is created once per `Converter`
    (`converter.go:59`) and `imageData`/`imageTypes` (`fb2/parser.go:41–42`) are never reset
    in `ParseBytes`. A second `Convert` on the same `Converter` inherits the previous book's
    images. Reset maps per parse (or create a parser per conversion).

### Hygiene

10. **Dead code:** `sanitizeFilename` (`fb2/parser.go:188`), `CalculateRecordCount` /
    `SortManifestIDs` / `ConvertStream` (`mobi/writer.go:47–75`, `converter.go:98`),
    `CompressRecord` / `DecompressPalmDOC` / `splitTextRecords`
    (`mobi/compression.go`, `mobi/content.go:120`), unused option fields
    (`Parser.NoInlineTOC`, `Parser.ProcessCSS`, `Transformer.ProcessCSS`),
    `joinNonEmpty` (`opf/book.go:231` — reinvents `strings.Join`).
    (`epub.Writer.uuid` was removed with the deterministic-UUID work.)
12. **Test coverage gaps** (re-checked 2026-08-17, testing-infra landed): `epub` now has
    byte + content goldens via the corpus, and `mobi` has a reader with tests;
    still 0%: `internal/mapper`, `cmd`. Priorities: mapper unit tests; items 8–9
    (EPUB cover manifest id, navPoint anchors) are locked into current goldens and
    remain open bugs.
13. **Docs drift:** AGENTS.md still names `fb2encoding/` (actual: `fb2/encoding/`) and
    `ConvertFile`/`ConvertFileWithOptions` (actual: `Converter.Convert`).
    (Fixed in AGENTS.md with the testing-infra docs update, 2026-08-17.)
14. **Workspace residue:** `debug_records/`, `final_v2_records/`, `ref_records/`,
    built `fb2c` binary are untracked and not in `.gitignore`.
    (Resolved 2026-08-17: debug dirs and legacy testdata artifacts deleted; `tmp/` ignored.)
