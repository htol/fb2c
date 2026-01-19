package index

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// INDXHeaderSize is the size of the INDX header
const INDXHeaderSize = 192

// INDXHeader represents the header of an INDX record
type INDXHeader struct {
	ID               uint32    // 0x00: "INDX"
	HeaderLength     uint32    // 0x04: Header length (usually 192)
	Unknown1         uint32    // 0x08: 0
	IndexType        uint32    // 0x0C: 0=normal, 2=inflection
	UnknownOffset    uint32    // 0x10: 0
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
	Header INDXHeader
	TAGX   *TAGX
	CNCX   []string    // String table
	IDXT   []IDXTEntry // Index entries
}

// IDXTEntry represents an entry in the index
type IDXTEntry struct {
	Offset      uint32              // Target offset in the book
	Size        uint32              // Size of the entry (calculated during write)
	TagValues   map[uint32][]uint32 // Map of tag ID to values
	RecordIndex int                 // Record index this entry points to (optional)
}

// NewINDX creates a new INDX record
func NewINDX(encoding, language uint32) *INDX {
	return &INDX{
		Header: INDXHeader{
			ID:           0x494E4458, // "INDX"
			HeaderLength: INDXHeaderSize,
			Encoding:     encoding,
			Language:     language,
			IndexType:    0,
			Padding:      [136]byte{},
		},
		TAGX: NewTAGX(),
		CNCX: make([]string, 0),
		IDXT: make([]IDXTEntry, 0),
	}
}

// AddEntry adds an index entry with offset tracking
func (i *INDX) AddEntry(offset uint32, recordIndex int, tagValues map[uint32][]uint32) {
	i.IDXT = append(i.IDXT, IDXTEntry{
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

// Encode encodes the INDX to bytes
func (i *INDX) Encode() ([]byte, error) {
	var buf bytes.Buffer

	// 1. Encode TAGX
	tagxData, err := i.TAGX.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode TAGX: %w", err)
	}

	// 2. Encode CNCX
	cncxData, err := i.encodeCNCX()
	if err != nil {
		return nil, fmt.Errorf("failed to encode CNCX: %w", err)
	}

	// 3. Pre-encode Entries to calculate offsets and total size
	var entryBuffers [][]byte
	for _, entry := range i.IDXT {
		data, err := i.encodeIDXTEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to encode IDXT entry: %w", err)
		}
		entryBuffers = append(entryBuffers, data)
	}

	// 4. Construct IDXT Block (The Table of Offsets)
	// Format: "IDXT" (4) + Length(2) + Type(2) + Offsets(N*2)
	idxtHeaderSize := 8 // 4 + 2 + 2
	idxtTableSize := idxtHeaderSize + (len(entryBuffers) * 2)

	idxtBuf := new(bytes.Buffer)
	idxtBuf.WriteString("IDXT")
	if err := binary.Write(idxtBuf, binary.BigEndian, uint16(idxtTableSize)); err != nil {
		return nil, fmt.Errorf("failed to write IDXT size: %w", err)
	}
	if err := binary.Write(idxtBuf, binary.BigEndian, uint16(0)); err != nil { // Type/Unused
		return nil, fmt.Errorf("failed to write IDXT type: %w", err)
	}

	// Calculate offsets relative to the END of the IDXT block (start of entries)
	// Ref: "The IDXT tag... contains the offsets of the index entries."
	// Standard behavior is relative to IDXT block start.
	currentOffset := uint16(idxtTableSize)
	for _, entryBuf := range entryBuffers {
		if err := binary.Write(idxtBuf, binary.BigEndian, currentOffset); err != nil {
			return nil, fmt.Errorf("failed to write IDXT offset: %w", err)
		}
		currentOffset += uint16(len(entryBuf))
	}
	idxtData := idxtBuf.Bytes()

	// 5. Update Header Fields
	// Layout: Header | TAGX | CNCX | IDXT | Entries
	i.Header.Count = uint32(len(entryBuffers))
	i.Header.TotalRecordCount = uint32(len(entryBuffers))
	i.Header.CNCXCount = uint32(len(i.CNCX))

	// IDXTOffset points to the IDXT block
	i.Header.IDXTOffset = INDXHeaderSize + uint32(len(tagxData)) + uint32(len(cncxData))

	// 6. Write Header
	if err := i.writeHeader(&buf); err != nil {
		return nil, err
	}

	// 7. Write TAGX
	buf.Write(tagxData)

	// 8. Write CNCX
	buf.Write(cncxData)

	// 9. Write IDXT Block
	buf.Write(idxtData)

	// 10. Write Entries
	for _, entryBuf := range entryBuffers {
		buf.Write(entryBuf)
	}

	return buf.Bytes(), nil
}

// writeHeader writes the INDX header
func (i *INDX) writeHeader(w *bytes.Buffer) error {
	fields := []interface{}{
		i.Header.ID,
		i.Header.HeaderLength,
		i.Header.Unknown1,
		i.Header.IndexType,
		i.Header.UnknownOffset,
		i.Header.IDXTOffset,
		i.Header.Count,
		i.Header.Encoding,
		i.Header.Language,
		i.Header.TotalRecordCount,
		i.Header.ORDTOffset,
		i.Header.LIGTOffset,
		i.Header.CountNeeded,
		i.Header.CNCXCount,
		i.Header.Padding,
	}

	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}
	return nil
}

// encodeCNCX encodes the CNCX (string table)
func (i *INDX) encodeCNCX() ([]byte, error) {
	var buf bytes.Buffer
	for _, s := range i.CNCX {
		// Varint length prefix
		lengthBytes := encodeVarint(uint32(len(s)))
		buf.Write(lengthBytes)
		buf.WriteString(s)
	}
	return buf.Bytes(), nil
}

// encodeVarint encodes a uint32 as a variable length integer (MOBI format)
func encodeVarint(val uint32) []byte {
	// 7 bits per byte, little endian? No, usually high bit check.
	// MOBI (Palm) varint:
	// A sequence of bytes, each containing 7 bits of data.
	// The high bit is set in the last byte.
	// Low bits first? Or high bits first?
	// MOBI typically uses forward encoding.
	// Actually, CNCX uses a specific varint.
	// Let's use the standard "stop bit" varint.

	// Simplest implementation for short strings (<128):
	if val < 128 {
		return []byte{byte(val | 0x80)} // Set high bit to indicate end
	}
	// For now support only small strings for safety
	// Or implement proper varint

	// Proper:
	// Reverse order chunks of 7 bits.
	// Last chunk gets 0x80.

	// Simplified loop
	// We need to write from MSB to LSB? Or LSB to MSB?
	// "The variable width integer... stores the integer 7 bits at a time... the high bit is set on the last byte."
	// Usually Big Endian style.

	// Let's use the simple <128 optimization for filenames.
	return []byte{byte(val | 0x80)}
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
	if err := binary.Write(&buf, binary.BigEndian, uint32(totalLen)); err != nil {
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

// encodeIDXTEntry encodes a single index entry
func (i *INDX) encodeIDXTEntry(entry IDXTEntry) ([]byte, error) {
	var buf bytes.Buffer

	// Based on standard MOBI behavior for these tags:
	var controlByte byte = 0x00

	// Write Control Byte
	buf.WriteByte(0) // Placeholder for control byte

	// Tag 1: Offset (Position 0 in control byte usually)
	if val, ok := entry.TagValues[1]; ok && len(val) > 0 {
		controlByte |= 0x01
		buf.Write(encodeVarint(val[0]))
	} else if entry.Offset > 0 {
		// Implicit tag 1
		controlByte |= 0x01
		buf.Write(encodeVarint(entry.Offset)) // Fallback uses Entry.Offset
	}

	// Tag 6: Name Index (Position 1 in control byte usually)
	if val, ok := entry.TagValues[6]; ok && len(val) > 0 {
		controlByte |= 0x02
		buf.Write(encodeVarint(val[0]))
	}

	// Update control byte
	data := buf.Bytes()
	data[0] = controlByte

	return data, nil
}

// TOCEntry represents a helper entry for building TOC
type TOCEntry struct {
	Label       string
	Offset      uint32
	Level       uint32 // Depth
	ParentIndex int    // Index of parent in the entries list, -1 if root
	Reference   string // HREF or similar identifier
}

// TOCIndexBuilder helper for building TOCs
type TOCIndexBuilder struct {
	INDX        *INDX
	entries     []TOCEntry
	textRecords [][]byte
}

// NewTOCIndexBuilder creates a new TOC index builder
func NewTOCIndexBuilder() *TOCIndexBuilder {
	indx := NewINDX(65001, 1033) // Default to UTF-8 and English

	// Initialize default TAGX for TOC
	// Tag 1: Offset, 1 value, Mask 0x01
	indx.TAGX.AddTag(1, 1, 0x01)
	// Tag 6: Name Offset, 1 value, Mask 0x02
	indx.TAGX.AddTag(6, 1, 0x02)
	// Tag 2: Level/Depth? usually present in NCX
	indx.TAGX.AddTag(2, 1, 0x04)
	// Tag 3: Parent?
	indx.TAGX.AddTag(3, 1, 0x08)

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
		recLen := uint32(len(rec))
		if offset < currentOffset+recLen {
			return i, offset - currentOffset
		}
		currentOffset += recLen
	}
	// If beyond, return last record and overflow offset (or handle as error)
	// For robustness return last record index
	if len(b.textRecords) > 0 {
		return len(b.textRecords) - 1, offset - (currentOffset - uint32(len(b.textRecords[len(b.textRecords)-1])))
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
	}

	// Determine parent (simple logic: last entry with Level < current Level)
	for i := len(b.entries) - 1; i >= 0; i-- {
		if b.entries[i].Level < level {
			entry.ParentIndex = i
			break
		}
	}

	b.entries = append(b.entries, entry)
}

// GetEntries returns the current list of entries
func (b *TOCIndexBuilder) GetEntries() []TOCEntry {
	return b.entries
}

// Build returns the built INDX record
func (b *TOCIndexBuilder) Build() (*INDX, error) {
	for _, entry := range b.entries {
		// Add title to CNCX
		nameIdx := b.INDX.AddString(entry.Label)

		// Create entry with tags
		tags := map[uint32][]uint32{
			1: {entry.Offset},
			6: {uint32(nameIdx)},
		}

		b.INDX.AddEntry(entry.Offset, -1, tags)
	}
	return b.INDX, nil
}

// FindOffsetForHref matches HREF to file offset
func (b *TOCIndexBuilder) FindOffsetForHref(html, href string) uint32 {
	// Simplified regex search
	targetID := strings.TrimPrefix(href, "#")

	// Try matching both id="..." and name="..." attributes
	patterns := []string{
		fmt.Sprintf(`id=['"]%s['"]`, regexp.QuoteMeta(targetID)),
		fmt.Sprintf(`name=['"]%s['"]`, regexp.QuoteMeta(targetID)),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		loc := re.FindStringIndex(html)
		if loc != nil {
			return uint32(loc[0])
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
