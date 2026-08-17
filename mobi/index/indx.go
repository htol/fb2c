package index

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/htol/fb2c/mobi/varint"
)

// INDXHeaderSize is the size of the INDX header
const INDXHeaderSize = 192

// INDXHeader represents the header of an INDX record
type INDXHeader struct {
	ID               uint32    // 0x00: "INDX"
	HeaderLength     uint32    // 0x04: Header length (usually 192)
	Unknown1         uint32    // 0x08: 0
	IndexType        uint32    // 0x0C: 0=Primary, 1=Secondary
	UnknownOffset    uint32    // 0x10: Unknown (2 for primary, 0 for secondary)
	IDXTOffset       uint32    // 0x14: Offset to IDXT table
	Count            uint32    // 0x18: Number of entries
	Encoding         uint32    // 0x1C: 1252 or 65001
	Language         uint32    // 0x20: Language
	TotalRecordCount uint32    // 0x24: Total entries
	ORDTOffset       uint32    // 0x28: ORDT offset
	LIGTOffset       uint32    // 0x2C: LIGT offset
	CountNeeded      uint32    // 0x30: Count required?
	CNCXCount        uint32    // 0x34: Number of CNCX entries
	Padding          [136]byte // Padding to 192 bytes
}

// INDX represents a MOBI INDX record
type INDX struct {
	Header    INDXHeader
	TAGX      *TAGX
	CNCX      []string    // String table
	IDXT      []IDXTEntry // Index entries
	RootTitle string      // Root title for NCX (author + title)
	TotalSize uint32      // Total size of the text content
}

// IDXTEntry represents an entry in the index
type IDXTEntry struct {
	Label       string              // Entry label/key text (required for MOBI index)
	Offset      uint32              // Target offset in the book
	Length      uint32              // Length of the chapter/section in bytes
	Size        uint32              // Size of the entry (calculated during write)
	TagValues   map[uint32][]uint32 // Map of tag ID to values
	RecordIndex int                 // Record index this entry points to (optional)
}

// NewINDX creates a new INDX record
func NewINDX(encoding, language uint32) *INDX {
	return &INDX{
		Header: INDXHeader{
			ID:            0x494E4458, // "INDX"
			HeaderLength:  INDXHeaderSize,
			Encoding:      encoding,
			Language:      language,
			IndexType:     0,
			UnknownOffset: 0,
			Padding:       [136]byte{},
		},
		TAGX: NewTAGX(),
		CNCX: make([]string, 0),
		IDXT: make([]IDXTEntry, 0),
	}
}

// AddEntry adds an index entry with offset tracking
func (i *INDX) AddEntry(label string, offset uint32, recordIndex int, tagValues map[uint32][]uint32) {
	i.IDXT = append(i.IDXT, IDXTEntry{
		Label:       label,
		Offset:      offset,
		TagValues:   tagValues,
		RecordIndex: recordIndex,
	})
	// Header counts are updated in Encode, but we update them here for immediate consistency
	i.Header.Count++
	i.Header.TotalRecordCount++
}

// AddString adds a string to CNCX (string table)
func (i *INDX) AddString(s string) int {
	// Simple deduplication could be added here
	index := len(i.CNCX)
	i.CNCX = append(i.CNCX, s)
	return index
}

// NCXIndexResult contains primary, secondary INDX and CNCX records
type NCXIndexResult struct {
	PrimaryINDX   []byte // Meta INDX record (IndexType=2)
	SecondaryINDX []byte // Data INDX record with actual entries
	CNCXRecord    []byte // String table with chapter names
	TotalEntries  int    // Number of TOC entries
}

// EncodeNCXIndex creates the three-record INDX structure for native NCX TOC
// Returns primary (meta), secondary (data), and CNCX (strings) records
func (i *INDX) EncodeNCXIndex() (*NCXIndexResult, error) {
	if len(i.IDXT) == 0 {
		return nil, fmt.Errorf("no INDX entries to encode")
	}

	// 1. Encode CNCX (string table) first
	cncxData, err := i.encodeCNCXRecord()
	if err != nil {
		return nil, fmt.Errorf("failed to encode CNCX: %w", err)
	}

	// 2. Encode secondary INDX (actual data)
	secondaryData, err := i.encodeSecondaryINDX()
	if err != nil {
		return nil, fmt.Errorf("failed to encode secondary INDX: %w", err)
	}

	// Calculate true total entries (including root entry in secondary INDX if present)
	totalEntries := len(i.IDXT)
	if i.RootTitle != "" {
		totalEntries++
	}

	// 3. Encode primary INDX (meta) that points to secondary
	primaryData, err := i.encodePrimaryINDX(totalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode primary INDX: %w", err)
	}

	return &NCXIndexResult{
		PrimaryINDX:   primaryData,
		SecondaryINDX: secondaryData,
		CNCXRecord:    cncxData,
		TotalEntries:  totalEntries,
	}, nil
}

// encodeCNCXRecord creates the CNCX string table record
func (i *INDX) encodeCNCXRecord() ([]byte, error) {
	var buf bytes.Buffer

	// Write root title first if set
	if i.RootTitle != "" {
		// Length as forward varint
		buf.Write(varint.EncodeForward(uint32(len(i.RootTitle)))) //nolint:gosec // Length fits
		buf.WriteString(i.RootTitle)
	}

	// Write chapter names with length prefix
	for _, s := range i.CNCX {
		buf.Write(varint.EncodeForward(uint32(len(s)))) //nolint:gosec // Length fits
		buf.WriteString(s)
	}

	// Spec §9: the CNCX table is terminated by a NUL byte (a zero where a
	// length prefix would start).
	buf.WriteByte(0)
	return buf.Bytes(), nil
}

// encodePrimaryINDX creates the primary/meta INDX record (IndexType=2)
// This record contains TAGX definitions and points to total entry count
func (i *INDX) encodePrimaryINDX(totalEntries int) ([]byte, error) {
	var buf bytes.Buffer

	// Create primary TAGX with NCX tag definitions
	// These match Calibre's output: tags 1,2,3,4 with terminator
	primaryTAGX := NewTAGX()
	primaryTAGX.AddTag(1, 1, 0x01) // Tag 1: Offset
	primaryTAGX.AddTag(2, 1, 0x02) // Tag 2: Size/Length
	primaryTAGX.AddTag(3, 1, 0x04) // Tag 3: Label offset
	primaryTAGX.AddTag(4, 1, 0x08) // Tag 4: Depth

	tagxData, err := primaryTAGX.EncodeWithTerminator()
	if err != nil {
		return nil, fmt.Errorf("failed to encode primary TAGX: %w", err)
	}

	// Create a single entry that represents the whole secondary index
	// Entry label is the count in hex format (e.g., "0E" for 14 entries)
	entryLabel := fmt.Sprintf("%02X", totalEntries-1) // 0-based max index
	entryData := []byte{byte(len(entryLabel))}
	entryData = append(entryData, []byte(entryLabel)...)
	entryData = append(entryData, 0x00) // Control byte (no tags for meta entry)

	// Total count must be encoded as a standard VWI (Protobuf-style/Little Endian Varint)
	// where MSB 1 means "more bytes" and MSB 0 means "stop".
	//
	// Reference (15): 0x0F (MSB 0 -> stop. Value 15).
	// Our Failure (131): 0x83. Reader saw MSB 1, read next byte 0x00 (padding).
	//                    0x83 & 0x7F = 3. 0x00 << 7 = 0. Total = 3.
	//
	// Solution: Encode 131 as 0x83 0x01 (131 = 3 + 128).
	val := uint32(totalEntries) //nolint:gosec // Count fits in uint32
	for val >= 128 {
		entryData = append(entryData, byte(val&0x7F)|0x80) // Set continuation bit
		val >>= 7
	}
	entryData = append(entryData, byte(val))

	entryData = append(entryData, 0x00, 0x00, 0x00) // Padding to match reference

	// IDXT block
	idxtBuf := new(bytes.Buffer)
	idxtBuf.WriteString("IDXT")
	// Offset to entry (after header + TAGX)
	entryOffset := uint16(INDXHeaderSize + len(tagxData)) //nolint:gosec // Fits in uint16
	if err := binary.Write(idxtBuf, binary.BigEndian, entryOffset); err != nil {
		return nil, err
	}
	// Padding to align
	idxtBuf.Write([]byte{0x00, 0x00})

	// Calculate layout
	// Header (192) | TAGX | Entry | IDXT
	idxtOffset := INDXHeaderSize + uint32(len(tagxData)) + uint32(len(entryData)) //nolint:gosec // Offset fits

	// Build header for primary INDX
	header := INDXHeader{
		ID:               0x494E4458, // "INDX"
		HeaderLength:     INDXHeaderSize,
		Unknown1:         0,
		IndexType:        0, // Primary/meta index (0 matches Reference 364)
		UnknownOffset:    2, // Unknown field (2 matches Reference 364)
		IDXTOffset:       idxtOffset,
		Count:            1, // One entry in this record
		Encoding:         i.Header.Encoding,
		Language:         0xFFFFFFFF,           // No specific language (match reference)
		TotalRecordCount: uint32(totalEntries), //nolint:gosec // Count fits
		ORDTOffset:       0,                    // Match reference (0, not 0xFFFFFFFF)
		LIGTOffset:       0,
		CountNeeded:      0,
		CNCXCount:        1, // One secondary INDX record
		Padding:          [136]byte{},
	}

	// Write header
	if err := writeINDXHeader(&buf, header); err != nil {
		return nil, err
	}

	// Write TAGX
	buf.Write(tagxData)

	// Write entry
	buf.Write(entryData)

	// Write IDXT
	buf.Write(idxtBuf.Bytes())

	return buf.Bytes(), nil
}

// encodeSecondaryINDX creates the secondary INDX record with actual TOC entries
func (i *INDX) encodeSecondaryINDX() ([]byte, error) { //nolint:gocyclo
	var buf bytes.Buffer

	// Pre-calculate CNCX byte offsets for each string
	// Note: CNCX offsets are calculated from position 0
	cncxOffsets := make([]uint32, len(i.CNCX))
	currentCNCXOffset := uint32(0)

	// Add root title at CNCX offset 0
	if i.RootTitle != "" {
		vLen := varint.EncodeForward(uint32(len(i.RootTitle)))    //nolint:gosec // Length fits
		currentCNCXOffset += uint32(len(vLen) + len(i.RootTitle)) //nolint:gosec // Offset fits
	}

	for idx, s := range i.CNCX {
		cncxOffsets[idx] = currentCNCXOffset
		vLen := varint.EncodeForward(uint32(len(s)))    //nolint:gosec // Length fits
		currentCNCXOffset += uint32(len(vLen) + len(s)) //nolint:gosec // Offset fits
	}

	// Secondary INDX has no TAGX, just entries
	// Pre-encode all entries with correct CNCX offsets
	var entryBuffers [][]byte

	// Add root entry (entry 00) pointing to root title at CNCX offset 0
	// This matches reference format where document root is entry 0
	if i.RootTitle != "" {
		// Root entry should cover the preamble (from 0 to first chapter offset)
		// If it covers TotalSize, it overlaps with all chapters, which breaks the index
		rootLength := i.TotalSize
		if len(i.IDXT) > 0 {
			rootLength = i.IDXT[0].Offset
		}

		// Root entry covers the preamble (Offset 0, Length = rootLength)
		rootEntry := i.encodeSecondaryEntryWithOffset("00", 0, rootLength, 0, 0)
		entryBuffers = append(entryBuffers, rootEntry)
	}

	// Add chapter entries starting at entry 01 (or 00 if no root title)
	for idx, entry := range i.IDXT {
		// Create entry with label - offset by 1 if root entry exists
		entryIdx := idx
		if i.RootTitle != "" {
			entryIdx = idx + 1
		}
		entryLabel := fmt.Sprintf("%02X", entryIdx)

		// Find CNCX offset for this entry's label
		cncxOffset := uint32(0)
		if idx < len(cncxOffsets) {
			cncxOffset = cncxOffsets[idx]
		}

		// Get depth from tag values
		depth := uint32(0)
		if vals, ok := entry.TagValues[4]; ok && len(vals) > 0 {
			depth = vals[0]
		}

		// Get length from tag values (not entry.Length which may be unset)
		length := entry.Length
		if vals, ok := entry.TagValues[2]; ok && len(vals) > 0 {
			length = vals[0]
		}

		data := i.encodeSecondaryEntryWithOffset(entryLabel, entry.Offset, length, cncxOffset, depth)
		entryBuffers = append(entryBuffers, data)
	}

	// Calculate layout: Header (192) | Entries | IDXT
	entriesStartOffset := uint32(INDXHeaderSize)
	var totalEntriesSize uint32
	for _, eb := range entryBuffers {
		totalEntriesSize += uint32(len(eb)) //nolint:gosec // Size fits
	}
	idxtOffset := entriesStartOffset + totalEntriesSize

	// Build IDXT block
	idxtBuf := new(bytes.Buffer)
	idxtBuf.WriteString("IDXT")
	currentOffset := entriesStartOffset
	for _, eb := range entryBuffers {
		if err := binary.Write(idxtBuf, binary.BigEndian, uint16(currentOffset)); err != nil { //nolint:gosec // Must fit in uint16
			return nil, err
		}
		currentOffset += uint32(len(eb)) //nolint:gosec // Fits
	}

	// Build header for secondary INDX
	header := INDXHeader{
		ID:               0x494E4458, // "INDX"
		HeaderLength:     INDXHeaderSize,
		Unknown1:         0,
		IndexType:        1, // Secondary/data index (1 matches Reference 365)
		UnknownOffset:    0, // Unknown field (0 matches Reference 365)
		IDXTOffset:       idxtOffset,
		Count:            uint32(len(entryBuffers)), //nolint:gosec // Count fits
		Encoding:         0xFFFFFFFF,                // No encoding for secondary (match reference)
		Language:         0xFFFFFFFF,                // No language for secondary (match reference)
		TotalRecordCount: 0,                         // Not used in secondary
		ORDTOffset:       0,                         // Match reference (0, not 0xFFFFFFFF)
		LIGTOffset:       0,                         // Match reference
		CountNeeded:      0,
		CNCXCount:        0,
		Padding:          [136]byte{},
	}

	// Write header
	if err := writeINDXHeader(&buf, header); err != nil {
		return nil, err
	}

	// Write entries
	for _, eb := range entryBuffers {
		buf.Write(eb)
	}

	// Write IDXT
	buf.Write(idxtBuf.Bytes())

	return buf.Bytes(), nil
}

// encodeSecondaryEntryWithOffset encodes a single entry with explicit CNCX offset
func (i *INDX) encodeSecondaryEntryWithOffset(label string, offset, length, cncxOffset, depth uint32) []byte {
	var buf bytes.Buffer

	// Write label
	buf.WriteByte(byte(len(label)))
	buf.WriteString(label)

	// Control byte - indicates which tags are present
	// For NCX: offset (0x01), length (0x02), label (0x04), depth (0x08)
	controlByte := byte(0x0F) // All four tags present
	buf.WriteByte(controlByte)

	// Tag 1: Offset (position in text)
	buf.Write(encodeVarint(offset))

	// Tag 2: Length (size of chapter)
	if length == 0 {
		length = 1000 // Default fallback
	}
	buf.Write(encodeVarint(length))

	// Tag 3: Label offset in CNCX (byte offset into CNCX record)
	buf.Write(encodeVarint(cncxOffset))

	// Tag 4: Depth
	buf.Write(encodeVarint(depth))

	return buf.Bytes()
}

// EncodeWithTerminator encodes TAGX with a terminator entry
func (t *TAGX) EncodeWithTerminator() ([]byte, error) {
	// Header: "TAGX" (4) + Length(4) + Control(4)
	// Entries: N * 4 bytes + Terminator (4 bytes)
	headerLen := 12
	entriesLen := (len(t.Entries) + 1) * 4 // +1 for terminator
	totalLen := headerLen + entriesLen

	var buf bytes.Buffer
	buf.WriteString("TAGX")
	if err := binary.Write(&buf, binary.BigEndian, uint32(totalLen)); err != nil { //nolint:gosec // Length fits
		return nil, fmt.Errorf("failed to write header length: %w", err)
	}
	buf.Write([]byte{0, 0, 0, 1}) // 1 Control Byte

	for _, entry := range t.Entries {
		buf.WriteByte(entry.TagID)
		buf.WriteByte(entry.NValues)
		buf.WriteByte(entry.Mask)
		buf.WriteByte(0) // End bit = 0
	}

	// Terminator entry: 0, 0, 0, 1
	buf.Write([]byte{0, 0, 0, 1})

	return buf.Bytes(), nil
}

// writeINDXHeader writes an INDX header to a buffer
func writeINDXHeader(w *bytes.Buffer, h INDXHeader) error {
	fields := []interface{}{
		h.ID,
		h.HeaderLength,
		h.Unknown1,
		h.IndexType,
		h.UnknownOffset,
		h.IDXTOffset,
		h.Count,
		h.Encoding,
		h.Language,
		h.TotalRecordCount,
		h.ORDTOffset,
		h.LIGTOffset,
		h.CountNeeded,
		h.CNCXCount,
		h.Padding,
	}

	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}
	return nil
}

// encodeVarint encodes a uint32 as a variable-width integer for index entry
// tag values. Spec §10: index entry tag values are forward-encoded VWI
// (MSB set on the last byte), like the CNCX string lengths. Only trailing-entry
// sizes use backward encoding.
func encodeVarint(val uint32) []byte {
	return varint.EncodeForward(val)
}

// TAGXEntry represents a single tag definition
type TAGXEntry struct {
	TagID   uint8
	NValues uint8
	Mask    uint8
}

// TAGX represents the tag definition table
type TAGX struct {
	Entries []TAGXEntry
}

// NewTAGX creates a new TAGX
func NewTAGX() *TAGX {
	return &TAGX{
		Entries: make([]TAGXEntry, 0),
	}
}

// AddTag adds a tag definition
func (t *TAGX) AddTag(tagID, nValues, mask uint8) {
	t.Entries = append(t.Entries, TAGXEntry{
		TagID:   tagID,
		NValues: nValues,
		Mask:    mask,
	})
}

// Encode encodes the TAGX to bytes
func (t *TAGX) Encode() ([]byte, error) {
	// Header: "TAGX" (4) + Length(4) + Control(4)
	// Entries: N * 4 bytes
	headerLen := 12
	entriesLen := len(t.Entries) * 4
	totalLen := headerLen + entriesLen

	var buf bytes.Buffer
	buf.WriteString("TAGX")
	if err := binary.Write(&buf, binary.BigEndian, uint32(totalLen)); err != nil { //nolint:gosec // Length fits
		return nil, fmt.Errorf("failed to write header length: %w", err)
	}
	buf.Write([]byte{0, 0, 0, 1}) // 1 Control Byte

	for _, entry := range t.Entries {
		buf.WriteByte(entry.TagID)
		buf.WriteByte(entry.NValues)
		buf.WriteByte(entry.Mask)
		buf.WriteByte(0) // End bit or reserved? usually 0 for simple tags
	}

	return buf.Bytes(), nil
}

// TOCEntry represents a helper entry for building TOC
type TOCEntry struct {
	Label       string
	Offset      uint32
	Length      uint32 // Size of this chapter in bytes (calculated from next entry offset)
	Level       uint32 // Depth (0=chapter, 1=section, 2=subsection, etc.)
	ParentIndex int    // Index of parent in the entries list, -1 if root
	FirstChild  int    // Index of first child, -1 if no children
	LastChild   int    // Index of last child, -1 if no children
	Reference   string // HREF or similar identifier
}

// TOCIndexBuilder helper for building TOCs
type TOCIndexBuilder struct {
	INDX          *INDX
	entries       []TOCEntry
	textRecords   [][]byte
	totalTextSize uint32 // Total size of uncompressed text for length calculation
}

// NewTOCIndexBuilder creates a new TOC index builder
func NewTOCIndexBuilder() *TOCIndexBuilder {
	// Encoding 65001 (UTF-8): the CNCX strings are UTF-8 and the reference
	// meta record declares 65001. Language 0xFFFFFFFF matches the reference.
	indx := NewINDX(65001, 0xFFFFFFFF)

	// Initialize NCX TAGX tags for TOC navigation
	// Match Reference (364) exactly: Tags 1, 2, 3, 4 only.
	indx.TAGX.AddTag(1, 1, 0x01)  // Tag 1: Offset/Position
	indx.TAGX.AddTag(2, 1, 0x02)  // Tag 2: Length
	indx.TAGX.AddTag(3, 1, 0x04)  // Tag 3: Name index in CNCX
	indx.TAGX.AddTag(4, 1, 0x08)  // Tag 4: Depth/Level
	indx.TAGX.AddTag(5, 1, 0x10)  // Tag 5: Parent index
	indx.TAGX.AddTag(21, 1, 0x20) // Tag 21: First child index
	indx.TAGX.AddTag(22, 1, 0x40) // Tag 22: Last child index

	return &TOCIndexBuilder{
		INDX:    indx,
		entries: make([]TOCEntry, 0),
	}
}

// SetTextRecords sets the text records for offset calculation
func (b *TOCIndexBuilder) SetTextRecords(records [][]byte) {
	b.textRecords = records
}

// CalculateRecordOffset calculates record index and relative offset from a linear text offset
func (b *TOCIndexBuilder) CalculateRecordOffset(offset uint32) (int, uint32) {
	currentOffset := uint32(0)
	for i, rec := range b.textRecords {
		recLen := uint32(len(rec)) //nolint:gosec // Length fits
		if offset < currentOffset+recLen {
			return i, offset - currentOffset
		}
		currentOffset += recLen
	}
	// If beyond, return last record and overflow offset (or handle as error)
	// For robustness return last record index
	if len(b.textRecords) > 0 {
		return len(b.textRecords) - 1, offset - (currentOffset - uint32(len(b.textRecords[len(b.textRecords)-1]))) //nolint:gosec // Length fits
	}
	return 0, 0
}

// AddEntry adds a TOC entry
func (b *TOCIndexBuilder) AddEntry(label, href string, level uint32, offset uint32) {
	entry := TOCEntry{
		Label:       label,
		Offset:      offset,
		Level:       level,
		Reference:   href,
		ParentIndex: -1,
		FirstChild:  -1,
		LastChild:   -1,
	}

	currentIdx := len(b.entries)

	// Determine parent (simple logic: last entry with Level < current Level)
	for i := len(b.entries) - 1; i >= 0; i-- {
		if b.entries[i].Level < level {
			entry.ParentIndex = i
			// Update parent's child tracking
			if b.entries[i].FirstChild == -1 {
				b.entries[i].FirstChild = currentIdx
			}
			b.entries[i].LastChild = currentIdx
			break
		}
	}

	b.entries = append(b.entries, entry)
}

// GetEntries returns the current list of entries
func (b *TOCIndexBuilder) GetEntries() []TOCEntry {
	return b.entries
}

// SetTotalTextSize sets the total text size for length calculation
func (b *TOCIndexBuilder) SetTotalTextSize(size uint32) {
	b.totalTextSize = size
	if b.INDX != nil {
		b.INDX.TotalSize = size
	}
}

// SetRootTitle sets the root title (author + book title) for NCX CNCX
func (b *TOCIndexBuilder) SetRootTitle(title string) {
	b.INDX.RootTitle = title
}

// BuildWithTotalSize builds the INDX with automatic length calculation
func (b *TOCIndexBuilder) BuildWithTotalSize(totalSize uint32) (*INDX, error) {
	b.totalTextSize = totalSize
	return b.Build()
}

// Build returns the built INDX record with full NCX tag values
func (b *TOCIndexBuilder) Build() (*INDX, error) {
	// Calculate chapter lengths based on next entry offset or total text size
	for i := range b.entries {
		if i < len(b.entries)-1 {
			// Length = next entry offset - this entry offset
			b.entries[i].Length = b.entries[i+1].Offset - b.entries[i].Offset
		} else {
			// Last entry: length = total text size - this entry offset
			if b.totalTextSize > 0 && b.totalTextSize > b.entries[i].Offset {
				b.entries[i].Length = b.totalTextSize - b.entries[i].Offset
			} else {
				// Fallback: use a reasonable default if totalTextSize not set
				b.entries[i].Length = 1000
			}
		}
	}

	for i, entry := range b.entries {
		// Add title to CNCX
		nameIdx := b.INDX.AddString(entry.Label)

		// Create entry with full NCX tags
		// Convert 1-based level to 0-based NCX depth
		depth := entry.Level
		if depth > 0 {
			depth--
		}
		tags := map[uint32][]uint32{
			1: {entry.Offset},    // Tag 1: Offset
			2: {entry.Length},    // Tag 2: Length
			3: {uint32(nameIdx)}, //nolint:gosec // Tag 3: Name index in CNCX
			4: {depth},           // Tag 4: Depth (0-based)
		}

		// Tag 5: Parent index (only if has parent)
		if entry.ParentIndex >= 0 {
			tags[5] = []uint32{uint32(entry.ParentIndex)} //nolint:gosec // Index fits
		}

		// Tag 21: First child index (only if has children)
		if entry.FirstChild >= 0 {
			tags[21] = []uint32{uint32(entry.FirstChild)} //nolint:gosec // Index fits
		}

		// Tag 22: Last child index (only if has children)
		if entry.LastChild >= 0 {
			tags[22] = []uint32{uint32(entry.LastChild)} //nolint:gosec // Index fits
		}

		// Use sequential numeric label (standard MOBI format: "0000", "0001", etc.)
		entryLabel := fmt.Sprintf("%04d", i)
		b.INDX.AddEntry(entryLabel, entry.Offset, -1, tags)
	}
	return b.INDX, nil
}

// FindOffsetForHref matches HREF to file offset
func (b *TOCIndexBuilder) FindOffsetForHref(html, href string) uint32 {
	// Simplified regex search
	targetID := strings.TrimPrefix(href, "#")

	// Match id="..." or name="..." attributes
	patterns := []string{
		fmt.Sprintf(`id=['"]%s['"]`, regexp.QuoteMeta(targetID)),
		fmt.Sprintf(`name=['"]%s['"]`, regexp.QuoteMeta(targetID)),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		loc := re.FindStringIndex(html)
		if loc != nil {
			return uint32(loc[0]) //nolint:gosec // Index fits
		}
	}
	return 0
}

// CalculateOffsetsFromHTML scans HTML to find offsets for existing entries
func (b *TOCIndexBuilder) CalculateOffsetsFromHTML(html string) error {
	for i := range b.entries {
		if b.entries[i].Reference != "" {
			offset := b.FindOffsetForHref(html, b.entries[i].Reference)
			if offset > 0 {
				b.entries[i].Offset = offset
			}
		}
	}

	// Re-sort entries by offset to ensure TOC order matches reading order
	SortTOC(b.entries)

	return nil
}

// SortTOC sorts entries by offset
func SortTOC(entries []TOCEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Offset < entries[j].Offset
	})
}
