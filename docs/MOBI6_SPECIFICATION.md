# MOBI 6 Format Specification

Curated reference for the legacy Mobipocket format ("MOBI 6", also called MOBI 7 / KF7-era
MOBI, as opposed to KF8/AZW3). Focused on what a writer needs; DRM, HUFF/CDIC compression
and KF8 are out of scope.

Sources (all byte layouts below trace to these):

- MobileRead wiki, *MOBI* page (current revision): <https://wiki.mobileread.com/wiki/MOBI>
- KindleUnpack source, header field tables: `lib/mobi_header.py` (`mobi6_header` dict)
  and `lib/mobi_index.py` (INDX/TAGX parsing): <https://github.com/kevinhendricks/KindleUnpack>
- Calibre PDB container writer: `src/calibre/ebooks/pdb/header.py` (`PdbHeaderBuilder`):
  <https://github.com/kovidgoyal/calibre/blob/master/src/calibre/ebooks/pdb/header.py>
- Calibre MOBI 6 writer (EXTH flags value, record layout, INDX fields):
  `src/calibre/ebooks/mobi/writer2/main.py` and `.../writer2/index.py`:
  <https://github.com/kovidgoyal/calibre/tree/master/src/calibre/ebooks/mobi/writer2>
- PalmDOC compressor (token stream of §11): `palmdoc.c` in gpsbabel-flytec:
  <https://github.com/twpayne/gpsbabel-flytec/blob/master/palmdoc.c>
- Byte-level reference: `testdata/reference.mobi` in this repo (Calibre-generated;
  §9 field values, §12 record order, EXTH flags 0x50)
- Local practice: `mobi/palmdb.go`, `mobi/header.go`, `mobi/index/indx.go`

All multi-byte integers are **big-endian** unless stated otherwise.

## Contents

1. [Container: Palm Database (PDB)](#1-container-palm-database-pdb)
2. [Record 0: PalmDOC header](#2-record-0-palmdoc-header)
3. [Record 0: MOBI header](#3-record-0-mobi-header)
4. [Record 0: EXTH header](#4-record-0-exth-header)
5. [Record 0: remainder (full name)](#5-record-0-remainder-full-name)
6. [Text records](#6-text-records)
7. [Image records](#7-image-records)
8. [Magic and compilation records](#8-magic-and-compilation-records)
9. [Index records (INDX)](#9-index-records-indx)
10. [Variable-width integers](#10-variable-width-integers)
11. [PalmDOC compression](#11-palmdoc-compression)
12. [Typical record layout of a MOBI 6 book](#12-typical-record-layout-of-a-mobi-6-book)

---

## 1. Container: Palm Database (PDB)

A MOBI file is a Palm database: a 78-byte header, a record info list, then all records
back to back. Records are numbered from 0; every reference in the MOBI header ("record
number of ...") is an index into this list.

| offset | bytes | content              | comments                                              |
|--------|-------|----------------------|-------------------------------------------------------|
| 0x00   | 32    | Database name        | Book title, ASCII/CP1252, NUL-padded; max 31 usable   |
| 0x20   | 2     | Attributes (flags)   | 0 for books                                           |
| 0x22   | 2     | Version              | 0                                                     |
| 0x24   | 4     | Creation date        | Palm epoch seconds (1904-01-01); 0 = unset           |
| 0x28   | 4     | Modification date    | same encoding                                         |
| 0x2C   | 4     | Last backup date     | 0                                                     |
| 0x30   | 4     | Modification number  | 0                                                     |
| 0x34   | 4     | App info ID          | 0                                                     |
| 0x38   | 4     | Sort info ID         | 0                                                     |
| 0x3C   | 4     | Type                 | `BOOK`                                                |
| 0x40   | 4     | Creator              | `MOBI`                                                |
| 0x44   | 4     | Unique ID seed       | random nonzero value; readers use it as the file's ID |
| 0x48   | 4     | Next record list ID  | 0 (single list)                                       |
| 0x4C   | 2     | Number of records    | N                                                     |

Record info list — N entries of 8 bytes each, starting at offset 78 (0x4E):

| offset | bytes | content      | comments                                     |
|--------|-------|--------------|----------------------------------------------|
| +0     | 4     | Record offset| absolute file offset of the record data      |
| +4     | 1     | Attributes   | 0 (bit 2 `0x08` = dirty, normally unused)    |
| +5     | 3     | Unique ID    | sequential; conventionally `2 * record index`|

Two zero filler bytes follow the record list (Calibre and KindleGen place them after the
list; the Palm OS spec's "gap to word boundary" placement after `numRecords` is also seen).
First record data then starts at `80 + 8*N` in files that include the filler.

> **Implementation note (fb2c):** `mobi/palmdb.go` omits the filler and puts record data at
> `78 + 8*N`. This is self-consistent — readers locate records through the offsets in the
> record list, not by computation — and output validates against Calibre. Reference files
> from Calibre/KindleGen use `80 + 8*N`.

The last record's length is "to end of file"; there is no length field per record.

## 2. Record 0: PalmDOC header

Record 0 starts with a 16-byte PalmDOC-compatible header:

| offset | bytes | content         | comments                                                                                     |
|--------|-------|-----------------|----------------------------------------------------------------------------------------------|
| 0x00   | 2     | Compression     | 1 = none, 2 = PalmDOC LZ77, 17480 (0x4448) = HUFF/CDIC (out of scope)                        |
| 0x02   | 2     | Unused          | 0                                                                                            |
| 0x04   | 4     | Text length     | uncompressed size of the whole book text (concatenation of all text records)                 |
| 0x08   | 2     | Record count    | number of records used for text (excluding record 0)                                         |
| 0x0A   | 2     | Record size     | maximum uncompressed size of a text record: **4096**                                         |
| 0x0C   | 2     | Encryption type | 0 = none, 1 = old Mobipocket, 2 = current Mobipocket (out of scope)                          |
| 0x0E   | 2     | Unused          | 0                                                                                            |

## 3. Record 0: MOBI header

Follows the PalmDOC header at offset 16. Variable length: 232 (0xE8) is the standard for
files produced since Mobipocket 6; older files use 0xE4. `header length` counts from the
`MOBI` magic (i.e. it is the length of everything below minus the first 16 bytes).

Offsets below are from the start of record 0 (add 16 when reading from the `MOBI` magic).

| offset | hex    | bytes | content                        | comments                                                                                        |
|--------|--------|-------|--------------------------------|--------------------------------------------------------------------------------------------------|
| 16     | 0x10   | 4     | Identifier                     | `MOBI`                                                                                            |
| 20     | 0x14   | 4     | Header length                  | typically 232 (0xE8)                                                                              |
| 24     | 0x18   | 4     | Mobi type                      | 2 = Mobipocket Book, 3 = PalmDoc Book, 4 = Audio, 232 = kindlegen 1.2, 257 = News, 258 = News Feed, 259 = News Magazine, 513 = PICS, 514 = WORD, 515 = XLS, 516 = PPT, 517 = TEXT, 518 = HTML |
| 28     | 0x1C   | 4     | Text encoding                  | 1252 = CP1252, 65001 = UTF-8                                                                      |
| 32     | 0x20   | 4     | Unique-ID                      | random; echoed in EXTH 204–207 convention                                                         |
| 36     | 0x24   | 4     | File version                   | Mobipocket format version: **6** for MOBI 6                                                       |
| 40     | 0x28   | 4     | Orthographic index             | dictionary orth index record number; 0xFFFFFFFF if none                                           |
| 44     | 0x2C   | 4     | Inflection index               | dictionary inflection index record number; 0xFFFFFFFF if none                                     |
| 48     | 0x30   | 4     | Index names                    | 0xFFFFFFFF if none                                                                                |
| 52     | 0x34   | 4     | Index keys                     | 0xFFFFFFFF if none                                                                                |
| 56     | 0x38   | 4     | Extra index 0                  | 0xFFFFFFFF if none                                                                                |
| 60     | 0x3C   | 4     | Extra index 1                  | 0xFFFFFFFF if none                                                                                |
| 64     | 0x40   | 4     | Extra index 2                  | 0xFFFFFFFF if none                                                                                |
| 68     | 0x44   | 4     | Extra index 3                  | 0xFFFFFFFF if none                                                                                |
| 72     | 0x48   | 4     | Extra index 4                  | 0xFFFFFFFF if none                                                                                |
| 76     | 0x4C   | 4     | Extra index 5                  | 0xFFFFFFFF if none                                                                                |
| 80     | 0x50   | 4     | First Non-book index           | first record number that is not book text (images, FLIS, FCIS, ...)                               |
| 84     | 0x54   | 4     | Full Name offset               | offset **in record 0** of the full book name (see §5)                                             |
| 88     | 0x58   | 4     | Full Name length               | length in bytes of the name                                                                       |
| 92     | 0x5C   | 4     | Locale                         | low byte = language (09 = English), next byte = dialect (04 = US, 08 = UK); en-US = 1033, en-GB = 2057, ru = 1049 |
| 96     | 0x60   | 4     | Input language                 | dictionary input language                                                                         |
| 100    | 0x64   | 4     | Output language                | dictionary output language                                                                        |
| 104    | 0x68   | 4     | Min version                    | minimum Mobipocket version needed (6)                                                             |
| 108    | 0x6C   | 4     | First Image index              | first record number containing an image; images are sequential; 0xFFFFFFFF if none                 |
| 112    | 0x70   | 4     | Huffman record offset          | HUFF/CDIC only; 0 otherwise                                                                       |
| 116    | 0x74   | 4     | Huffman record count           | HUFF/CDIC only; 0 otherwise                                                                       |
| 120    | 0x78   | 4     | Huffman table offset           | HUFF/CDIC only                                                                                    |
| 124    | 0x7C   | 4     | Huffman table length           | HUFF/CDIC only                                                                                    |
| 128    | 0x80   | 4     | EXTH flags                     | bit 6 (0x40) set = EXTH header present; writers typically emit 0x50 (bit 4 set too — undocumented, see §4) |
| 132    | 0x84   | 32    | Unknown                        | zeros                                                                                             |
| 164    | 0xA4   | 4     | Unknown                        | 0xFFFFFFFF (KindleUnpack: `unknown0`)                                                             |
| 168    | 0xA8   | 4     | DRM offset                     | 0xFFFFFFFF if no DRM                                                                              |
| 172    | 0xAC   | 4     | DRM count                      | 0xFFFFFFFF if no DRM                                                                              |
| 176    | 0xB0   | 4     | DRM size                       | 0 if no DRM                                                                                       |
| 180    | 0xB4   | 4     | DRM flags                      | 0 if no DRM                                                                                       |
| 184    | 0xB8   | 8     | Unknown                        | zeros; if header length ≥ 228 these two words are "bytes to end of MOBI header" in old files       |
| 192    | 0xC0   | 2     | First content record number    | normally 1 (first text record after record 0)                                                     |
| 194    | 0xC2   | 2     | Last content record number     | last image record, or last text record if no images; includes Image, DATP, HUFF, DRM records       |
| 196    | 0xC4   | 4     | Unknown                        | 0x00000001 (in KF8 headers this slot is the FDST offset — not used in MOBI 6)                     |
| 200    | 0xC8   | 4     | FCIS record number             | see §8                                                                                            |
| 204    | 0xCC   | 4     | FCIS record count              | 0x00000001                                                                                        |
| 208    | 0xD0   | 4     | FLIS record number             | see §8                                                                                            |
| 212    | 0xD4   | 4     | FLIS record count              | 0x00000001                                                                                        |
| 216    | 0xD8   | 8     | Unknown                        | zeros                                                                                             |
| 224    | 0xE0   | 4     | Unknown                        | 0xFFFFFFFF (KindleUnpack: `srcs_offset`)                                                          |
| 228    | 0xE4   | 4     | First compilation data section | 0x00000000 (KindleUnpack: `srcs_count`)                                                           |
| 232    | 0xE8   | 4     | Number of compilation sections | 0xFFFFFFFF (KindleUnpack: `unknown3`)                                                             |
| 236    | 0xEC   | 4     | Unknown                        | 0xFFFFFFFF (KindleUnpack: `unknown4`)                                                             |
| 240    | 0xF0   | 2     | Unknown                        | 0 (KindleUnpack: `fill5`)                                                                         |
| 242    | 0xF2   | 2     | **Extra Record Data Flags**    | trailing-entry flags for text records, see §6 (KindleUnpack: `traildata_flags`)                   |
| 244    | 0xF4   | 4     | INDX record offset             | record number of the first INDX record (the flat/NCX TOC index); 0xFFFFFFFF if none               |
| 248    | 0xF8   | 4     | Unknown                        | 0xFFFFFFFF (KF8: fragment index; 0xFFFFFFFF in MOBI 6)                                            |
| 252    | 0xFC   | 4     | Unknown                        | 0xFFFFFFFF (KF8: skeleton index; 0xFFFFFFFF in MOBI 6)                                            |
| 256    | 0x100  | 4     | Unknown                        | 0xFFFFFFFF (KindleUnpack: `datp_offset`)                                                          |
| 260    | 0x104  | 4     | Unknown                        | 0xFFFFFFFF (KF8: guide index)                                                                     |

Notes:

- Only fields up to `16 + header length` are present. A 232-byte MOBI header ends at
  0xF8, so it includes the INDX record offset (0xF4) but nothing after it; offsets 248+
  exist only in longer headers (e.g. the 256-byte variant used by KindleGen for dual
  MOBI6+KF8 files).
- The wiki historically documents offset 240 (0xF0) as a 4-byte "Extra Record Data Flags".
  KindleUnpack and Calibre read/write it as a 16-bit field at **0xF2** (`fill5` H at 0xF0,
  `traildata_flags` H at 0xF2). Use the 16-bit reading.
- MOBI 6 files from KindleGen are often part of a joint file where a second (KF8) header
  record follows a `BOUNDARY` record; MOBI Record Offset (EXTH 121) marks it. A pure
  MOBI 6 writer does not create it.

## 4. Record 0: EXTH header

Present when EXTH flags bit 6 (0x40) is set. Follows immediately after the MOBI header.

> **EXTH flags in practice:** Calibre always writes `0b1010000` (0x50) here — bit 6 plus
> undocumented bit 4 — and adds bit 3 for periodicals and bit 12 for embedded fonts
> (`calibre/ebooks/mobi/writer2/main.py`; the comment there says the purpose of the other
> bits is unknown). Readers test bit 6 only, but writers set 0x50; the Calibre-generated
> reference file carries 0x50.

| bytes | content            | comments                                                                     |
|-------|--------------------|-------------------------------------------------------------------------------|
| 4     | Identifier         | `EXTH`                                                                         |
| 4     | Header length      | includes the 8 bytes above, excludes the final padding                         |
| 4     | Record count       | number of EXTH records following                                              |
| —     | *records*          | repeated `<type:4><length:4><data>`; `length` counts the 8 header bytes + data |
| p     | Padding            | NUL bytes so the padded length is a multiple of 4; not counted in the length   |

Record types relevant to MOBI 6 books (strings use the record-0 text encoding):

| type  | usual length | name             | comments                                                            |
|-------|--------------|------------------|----------------------------------------------------------------------|
| 1–4   | var          | drm server/commerce/ebookbase ids | DRM, out of scope                                     |
| 100   | var          | author           | may repeat; OPF `dc:creator`                                        |
| 101   | var          | publisher        |                                                                      |
| 102   | var          | imprint          |                                                                      |
| 103   | var          | description      |                                                                      |
| 104   | var          | ISBN             |                                                                      |
| 105   | var          | subject          | may repeat                                                           |
| 106   | var          | publishing date  |                                                                      |
| 107   | var          | review           |                                                                      |
| 108   | var          | contributor      | may repeat                                                           |
| 109   | var          | rights           |                                                                      |
| 110   | var          | subject code     |                                                                      |
| 111   | var          | type             |                                                                      |
| 112   | var          | source           |                                                                      |
| 113   | var          | ASIN             |                                                                      |
| 114   | var          | version number   |                                                                      |
| 115   | 4            | sample           | 1 if content is a sample                                             |
| 116   | 4            | start reading    | offset to open at; 0xFFFFFFFF = unset                               |
| 117   | 3            | adult            | "yes"                                                                |
| 118   | var          | retail price     | text, e.g. "4.99"                                                    |
| 119   | var          | price currency   | text, e.g. "USD"                                                     |
| 121   | 4            | KF8 boundary     | record offset of the KF8 part in joint files; MOBI 6 only: absent    |
| 125   | 4            | resource count   | count of embedded resources                                          |
| 200   | 3            | dict short name | dictionaries                                                         |
| 201   | 4            | cover offset     | add to First Image index → record with cover image                   |
| 202   | 4            | thumb offset     | add to First Image index → record with cover thumbnail               |
| 203   | 4            | has fake cover   |                                                                      |
| 204   | 4            | creator software | 1 = mobigen, 2 = Mobipocket Creator, 200/201/202 = kindlegen win/linux/mac |
| 205   | 4            | creator major    |                                                                      |
| 206   | 4            | creator minor    |                                                                      |
| 207   | 4            | creator build    |                                                                      |
| 208   | var          | watermark       |                                                                      |
| 209   | var          | tamper proof keys | Kindle PID generation                                               |
| 300   | var          | font signature   | dictionaries                                                         |
| 401   | 1            | clipping limit   | percent of text clippable, usually 10                                |
| 402   | 1            | publisher limit  |                                                                      |
| 404   | 1            | TTS flag         | 1 = text-to-speech disabled                                          |
| 501   | 4            | cde type         | `PDOC` (personal doc), `EBOK` (ebook), `EBSP` (sample)               |
| 502   | var          | last update time |                                                                      |
| 503   | var          | updated title    | title used by Kindle devices (overrides Full Name)                   |
| 504   | var          | ASIN (CDE key)   | duplicate of 113                                                     |
| 506   | var          | title language   |                                                                      |
| 507–523 | var        | title/author/publisher collation metadata (language, direction, pronunciation, collation per field) | Japanese-oriented |
| 524   | var          | language         | OPF `dc:language`                                                    |
| 525   | var          | writing mode     | e.g. `horizontal-lr`                                                 |
| 526   | var          | NCX ingested by | software marker                                                      |
| 527   | var          | page progression | `rtl` / `ltr`                                                        |
| 528   | var          | override fonts   |                                                                      |

## 5. Record 0: remainder (full name)

After the EXTH header (or after the MOBI header when there is no EXTH):

- possibly some bytes of unknown use (usually zeros);
- the **full name** of the book, at `Full Name offset`, `Full Name length` bytes;
- two NUL bytes, then NUL padding to a 4-byte boundary (padding not counted in the length);
- more unknown/NUL data to the end of record 0.

Record 0 is typically padded to a 4-byte multiple overall.

## 6. Text records

The book text (a single HTML stream in Mobipocket markup) is split into records following
record 0:

- maximum **4096 bytes uncompressed** per record — the limit applies to the text
  portion only; trailing entries (below) are appended after it, so the stored record can
  exceed 4096 bytes when they are present;
- the split must not break a multi-byte UTF-8 character *inside* the text portion; when
  Extra Data Flags bit 0 is set the overlap is carried in a trailing entry instead;
- with compression = 2 each record is independently LZ77-compressed (§11).

### Trailing entries

When `Extra Record Data Flags` (MOBI+0xF2) is nonzero, each text record is followed by
one trailing entry per set bit, in bit order (bit 0 first, immediately after the text).
Entries for bits ≥ 1 have the form `<data><size>` where `<size>` is the size of the whole
entry (including the size byte itself) as a **backward-encoded** variable-width integer
(§10).

| bit   | meaning                          | format                                             |
|-------|----------------------------------|----------------------------------------------------|
| 0x01  | multi-byte character overlap     | 0–3 overlap bytes + 1 count byte (N in bits 0–1)   |
| 0x02  | indexing data (TBS)              | disables `<reference type="start">` guide handling |
| 0x04  | uncrossable breaks               |                                                     |

For an uncompressed, TBS-less book (the fb2c default) the flags can be 0 and no trailing
entries are written. The overlap bytes of bit 0x01 are *not* compressed but *are* counted
for encryption; the overlapped bytes reappear as normal content at the start of the next
record.

## 7. Image records

Images follow the text records, one image per record, in the order referenced by
`recindex="00000"`-based `<img>` attributes (first image = recindex 00001). The 4096-byte
record limit does **not** apply to image records. Supported formats: GIF, JPEG (JPeg),
PNG, BMP; Kindle devices additionally read images embedded in a HD `CONT` container (out
of scope). First record number of the run is in MOBI+0x6C; the cover is located via
EXTH 201/202.

## 8. Magic and compilation records

### FLIS record (32+ bytes, mostly fixed)

| offset | bytes | value          |
|--------|-------|----------------|
| 0      | 4     | `FLIS`         |
| 4      | 4     | 8              |
| 8      | 2     | 65             |
| 10     | 2     | 0              |
| 12     | 4     | 0              |
| 16     | 4     | 0xFFFFFFFF     |
| 20     | 2     | 1              |
| 22     | 2     | 3              |
| 24     | 4     | 3              |
| 28     | 4     | 1              |
| 32     | 4     | 0xFFFFFFFF     |

### FCIS record (44 bytes, text length dependent)

| offset | bytes | value                                    |
|--------|-------|------------------------------------------|
| 0      | 4     | `FCIS`                                   |
| 4      | 4     | 20                                       |
| 8      | 4     | 16                                       |
| 12     | 4     | 1                                        |
| 16     | 4     | 0                                        |
| 20     | 4     | text length (== PalmDOC+0x04 value)      |
| 24     | 4     | 0                                        |
| 28     | 4     | 32                                       |
| 32     | 4     | 8                                        |
| 36     | 2     | 1                                        |
| 38     | 2     | 1                                        |
| 40     | 4     | 0                                        |

### End-of-file record

Exactly 4 bytes: `0xE9 0x8E 0x0D 0x0A`. Always the last record of the file.

### SRCS / CMET records (KindleGen only)

`SRCS` = embedded zip of the sources (12-byte mini-header then zip data); `CMET` =
compilation log. A standalone MOBI 6 writer does not create them.

## 9. Index records (INDX)

TOC (and dictionary) indexes are stored as a sequence of INDX records. The MOBI header
field INDX record offset (0xF4) points at the first one. Structure:

```
[INDX meta record: INDX header + TAGX]   ← what 0xF4 points to
[INDX entry record(s): INDX header + entries + IDXT]
[optional CNCX/CTOC string-table record(s)]
```

### INDX header (192 = 0xC0 bytes in practice)

| offset | hex  | bytes | content            | comments                                                      |
|--------|------|-------|--------------------|----------------------------------------------------------------|
| 0x00   | 0x00 | 4     | Identifier         | `INDX`                                                          |
| 0x04   | 0x04 | 4     | Header length      | 192                                                             |
| 0x08   | 0x08 | 4     | Unknown            | 0                                                              |
| 0x0C   | 0x0C | 4     | Index type         | 0 = normal index, 1 = normal index entry (data) record, 2 = inflection (dictionaries). Observed in the Calibre reference: meta record type 0, entry record type 1 |
| 0x10   | 0x10 | 4     | Unknown            | 2 in the meta record, 0 in entry records (KindleUnpack: `gen`)   |
| 0x14   | 0x14 | 4     | IDXT offset        | offset of the IDXT table within *this* record                   |
| 0x18   | 0x18 | 4     | Count              | meta record: number of entry records that follow; entry record: number of entries in this record |
| 0x1C   | 0x1C | 4     | Encoding           | 1252 or 65001 in the meta record; 0xFFFFFFFF in entry records   |
| 0x20   | 0x20 | 4     | Language           | wiki: language code of the index; the Calibre reference writes 0xFFFFFFFF in both the meta and entry records (KindleUnpack ignores the field) |
| 0x24   | 0x24 | 4     | Total entry count  | total entries across all entry records (meta record only; 0 in entry records) |
| 0x28   | 0x28 | 4     | ORDT offset        | within-record offset of ORDT (sort) table, else 0               |
| 0x2C   | 0x2C | 4     | LIGT offset        | within-record offset of LIGT (ligature) table                   |
| 0x30   | 0x30 | 4     | LIGT count         |                                                                 |
| 0x34   | 0x34 | 4     | CNCX count         | number of CNCX/CTOC records following the entry records         |
| 0x38   | 0x38 | ...   | zero padding       | to 192 bytes                                                    |
| 0xA4   | 0xA4 | 20    | ORDT descriptor    | 5 words: count, entries, ORDT1 offset, ORDT2 offset, TAGX offset (dictionaries only; zero otherwise) |

### TAGX section (in the meta record, at `header length`)

| offset | bytes | content              | comments                                    |
|--------|-------|----------------------|----------------------------------------------|
| 0      | 4     | Identifier           | `TAGX`                                        |
| 4      | 4     | Header length        | 12 + 4 × number of tag entries               |
| 8      | 4     | Control byte count   | number of control bytes per entry            |
| 12     | 4×n   | Tag table            | entries of 4 bytes, see below                |

Each tag table entry is 4 bytes:

| byte | content          | comments                                                       |
|------|------------------|-----------------------------------------------------------------|
| 0    | Tag number       | meaning is index-kind-specific, see tag table below             |
| 1    | Values per entry | how many VWI values one occurrence of the tag carries           |
| 2    | Mask             | bit(s) in the control byte that signal the tag's presence       |
| 3    | End flag         | 0x01 = end of control byte group; all other bytes then 0        |

### Entry records

Each entry record repeats the INDX header (Count = entries in this record, IDXT offset
points into this record) and then contains, back to back:

```
<entry>  ::= <label length: 1><label bytes><control bytes: CBC><tag value bytes>
```

- `label length` is a raw byte (max 255);
- `control bytes` is `Control byte count` bytes from the TAGX section;
- for every tag whose control-byte bits are nonzero, the tag's VWI values follow after
  the control bytes, in tag-table order;
- special case: if the masked value equals the mask and the mask has more than one bit,
  a single VWI *byte count* follows the control bytes, and then that many bytes of VWI
  values.

At `IDXT offset` the record ends with the IDXT table: the 4 ASCII bytes `IDXT`, then
`Count` 16-bit big-endian offsets, each pointing at the start of an entry (relative to
the record start). The end of the last entry is the IDXT offset itself.

### CNCX / CTOC string table

`CNCX count` records after the entry records. Each is a sequence of
`<forward-encoded VWI length><string bytes>`, terminated by a NUL byte. Entries reference
strings by byte offset into this table (tag 3, see below).

### Tag numbers used by book TOC indexes

Reverse-engineered convention (KindleUnpack, Calibre, and `mobi/index/indx.go` agree):

| tag | meaning                     | notes                                    |
|-----|-----------------------------|------------------------------------------|
| 1   | offset in text              | file position of the entry target        |
| 2   | length                      | size of the section in bytes             |
| 3   | label index                 | byte offset into the CNCX string table   |
| 4   | depth                       | hierarchy level, 0-based (Calibre and fb2c write 0 for top level; the flat TOC in the reference file has 0 for every entry) |
| 5   | parent index                | index (0-based) of parent entry          |
| 6   | first child index           |                                          |
| 21  | first child index (alt)     |                                          |
| 22  | last child index            |                                          |

Masks are not fixed by the format: they are whatever the file's own TAGX table defines;
a typical assignment is 0x01, 0x02, 0x04, 0x08, 0x10 for tags 1–5.

## 10. Variable-width integers

Big-endian, 7 bits per byte, bit 8 used as a terminator.

- **Forward-encoded** (index entry values, CNCX lengths): the *last* (least significant)
  byte has bit 8 set.
  Example — 0x11111 = `04 22 91`.
- **Backward-encoded** (trailing-entry sizes): the *first* (most significant) byte has
  bit 8 set.
  Example — 0x11111 = `84 22 11`.

Decode (forward): `value = (value << 7) | (b & 0x7F)` until a byte with `b & 0x80`.

## 11. PalmDOC compression

Only relevant when record 0 compression = 2. Each text record is compressed independently
after appending trailing entries (trailing-entry bytes themselves are stored uncompressed
at the end). Token stream:

| byte(s)   | meaning                                                             |
|-----------|----------------------------------------------------------------------|
| 0x00      | one space (0x20)                                                      |
| 0x01–0x08 | next 1–8 bytes are literals                                           |
| 0x09–0x7F | literal                                                               |
| 0x80–0xBF | LZ77 pair, see below                                                  |
| 0xC0–0xFF | one space + one literal ASCII byte (`byte & 0x7F`, always ≥ 0x40)      |

The LZ77 pair stores a 14-bit compound `((b0 & 0x3F) << 8) | b1`: the top 11 bits are
the distance, the low 3 bits the length:

```
distance = ((b0 & 0x3F) << 5) | (b1 >> 3)
length   = (b1 & 0x07) + 3        ; 3..10
```

The pair copies `length` bytes starting `distance` bytes back in the already-decoded
output of the current record; `distance` may be smaller than `length` (overlapping copy,
used as run-length for repeated bytes).

> fb2c currently writes compression = 1 (none): the PalmDOC compressor is known broken
> in the `mobi` package. See AGENTS.md.

## 12. Typical record layout of a MOBI 6 book

```
0          record 0  (PalmDOC + MOBI + EXTH + full name)
1..T       text records (≤ 4096 B uncompressed each, + trailing entries per flags)
T+1        INDX meta record            ← MOBI+0xF4 and First Non-book index point here
T+2..T+K   INDX entry record(s)
T+K+1..    CNCX string-table record(s)
...        image records (one per image; may be absent)
...        FLIS
...        FCIS
last       EOF record (E9 8E 0D 0A)
```

This is the order both Calibre and fb2c emit (verified against
`testdata/reference.mobi`: records 364–366 INDX/CNCX, 367–368 images, 369 FLIS,
370 FCIS, 371 EOF). The order itself is **not normative** — readers locate every block
through the MOBI header fields (INDX record offset, First Image index, FCIS/FLIS record
numbers) — so a writer may lay records out differently.

Wiring between headers and records that a writer must keep consistent:

- PalmDOC record count = number of text records (not images/other records);
- First Non-book index (MOBI+0x50) = first record after the text records (in the
  reference and in fb2c this is the INDX meta record)
- First Image index (MOBI+0x6C) = first image record number;
- First/Last content record (MOBI+0xC0/0xC2) span the text records plus images and
  DATP/HUFF/DRM records (per the wiki); i.e. everything up to the first book content end.
- FCIS text length (§8) equals PalmDOC text length;
- Full Name offset (MOBI+0x54) points inside record 0 after the padded EXTH.
