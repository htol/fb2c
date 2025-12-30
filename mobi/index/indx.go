// Package index provides MOBI index generation (INDX, TAGX, CNCX).
package index

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/htol/fb2c/varint"
)

const (
	INDXHeaderSize = 192
)

// INDXHeader represents an INDX header
type INDXHeader struct {
	// First 4 bytes: offset to TAGX (from start of INDX)
	TagxOffset uint32

	// Header fields
	ID             uint32
	HeaderLength   uint32
	Unknown1       uint32
	IndexType      uint32
	IndexOffset    uint32
	RecordCount    uint32
	RecordSize     uint32
	Encoding       uint32
	Unknown2       [84]byte // Padding/reserved
	TotalRecordCount uint32
	OrtographicCount uint32
	IndexedCount    uint32
	Unknown3        [4]uint32 // More reserved fields
}

// INDX represents a complete INDX structure
type INDX struct {
	Header    INDXHeader
	TAGX      *TAGX
	IDXT      []IDXTEntry
	CNCX      []string
}

// TAGX represents tag table for index entries
type TAGX struct {
	Entries []TAGXEntry
}

// TAGXEntry represents a single tag definition
type TAGXEntry struct {
	TagID   uint32
	Count   uint32 // Number of values
	Control byte   // Control byte (bit flags)
}

// IDXTEntry represents an index entry with proper offset tracking
type IDXTEntry struct {
	Offset       uint32 // Offset in text records
	Size         uint32 // Size of entry data
	TagValues    map[uint32][]uint32 // Tag ID -> values
	RecordIndex  int    // Which text record this entry is in
	RecordOffset uint32 // Offset within that record
}

// NewINDX creates a new INDX structure
func NewINDX(indexType uint32) *INDX {
	return &INDX{
		Header: INDXHeader{
			HeaderLength: INDXHeaderSize,
			IndexType:    indexType,
			Encoding:     65001, // UTF-8
		},
		TAGX: NewTAGX(),
		IDXT: make([]IDXTEntry, 0),
		CNCX: make([]string, 0),
	}
}

// NewTAGX creates a new TAGX
func NewTAGX() *TAGX {
	return &TAGX{
		Entries: make([]TAGXEntry, 0),
	}
}

// AddTag adds a tag to the TAGX
func (t *TAGX) AddTag(tagID, count uint32, control byte) {
	t.Entries = append(t.Entries, TAGXEntry{
		TagID:  tagID,
		Count:  count,
		Control: control,
	})
}

// AddEntry adds an index entry with offset tracking
func (i *INDX) AddEntry(offset uint32, recordIndex int, tagValues map[uint32][]uint32) {
	i.IDXT = append(i.IDXT, IDXTEntry{
		Offset:      offset,
		TagValues:   tagValues,
		RecordIndex: recordIndex,
	})
	i.Header.RecordCount++
}

// AddString adds a string to CNCX (string table)
func (i *INDX) AddString(s string) int {
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

	// 3. Update header
	i.Header.TagxOffset = INDXHeaderSize
	i.Header.IndexOffset = INDXHeaderSize + uint32(len(tagxData))

	// 4. Write header
	if err := i.writeHeader(&buf); err != nil {
		return nil, err
	}

	// 5. Write TAGX
	buf.Write(tagxData)

	// 6. Write CNCX
	buf.Write(cncxData)

	// 7. Write IDXT entries
	for _, entry := range i.IDXT {
		entryData, err := i.encodeIDXTEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to encode IDXT entry: %w", err)
		}
		entry.Size = uint32(len(entryData))
		buf.Write(entryData)
	}

	return buf.Bytes(), nil
}

// writeHeader writes the INDX header
func (i *INDX) writeHeader(w *bytes.Buffer) error {
	// Write offset to TAGX
	if err := binary.Write(w, binary.BigEndian, i.Header.TagxOffset); err != nil {
		return err
	}

	// Write other header fields
	fields := []interface{}{
		i.Header.ID,
		i.Header.HeaderLength,
		i.Header.Unknown1,
		i.Header.IndexType,
		i.Header.IndexOffset,
		i.Header.RecordCount,
		i.Header.RecordSize,
		i.Header.Encoding,
		i.Header.Unknown2,
		i.Header.TotalRecordCount,
		i.Header.OrtographicCount,
		i.Header.IndexedCount,
		i.Header.Unknown3[0],
		i.Header.Unknown3[1],
		i.Header.Unknown3[2],
		i.Header.Unknown3[3],
	}

	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}

	// Fill remaining bytes with zeros
	remaining := INDXHeaderSize - 4 - (len(fields) * 4)
	padding := make([]byte, remaining)
	w.Write(padding)

	return nil
}

// encodeCNCX encodes the CNCX (string table)
func (i *INDX) encodeCNCX() ([]byte, error) {
	var buf bytes.Buffer

	// CNCX format: length prefix (VWI) + string for each entry
	for _, s := range i.CNCX {
		// Write length as varint
		lengthBytes := varint.EncodeForward(uint32(len(s)))
		buf.Write(lengthBytes)

		// Write string bytes
		buf.WriteString(s)
	}

	return buf.Bytes(), nil
}

// encodeIDXTEntry encodes a single IDXT entry
func (i *INDX) encodeIDXTEntry(entry IDXTEntry) ([]byte, error) {
	var buf bytes.Buffer

	// Write offset (varint)
	offsetBytes := varint.EncodeForward(entry.Offset)
	buf.Write(offsetBytes)

	// Write tag values in order of TAGX entries
	for _, tag := range i.TAGX.Entries {
		values, ok := entry.TagValues[tag.TagID]
		if !ok {
			continue
		}

		// Write each value as varint
		for _, val := range values {
			valBytes := varint.EncodeForward(val)
			buf.Write(valBytes)
		}
	}

	return buf.Bytes(), nil
}

// Encode encodes the TAGX to bytes
func (t *TAGX) Encode() ([]byte, error) {
	var buf bytes.Buffer

	// Write header (4 bytes: number of tags)
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(t.Entries))); err != nil {
		return nil, err
	}

	// Write tag entries (each: 1 byte control + 3 bytes tag ID)
	for _, entry := range t.Entries {
		// Write control byte
		buf.WriteByte(entry.Control)

		// Write tag ID (24-bit, big-endian)
		tagID := make([]byte, 4)
		binary.BigEndian.PutUint32(tagID, entry.TagID)
		buf.Write(tagID[1:4]) // Skip first byte, use last 3
	}

	return buf.Bytes(), nil
}

// TOCIndexBuilder builds a TOC index with proper offset tracking
type TOCIndexBuilder struct {
	indx         *INDX
	entries      []TOCEntry
	textRecords  [][]byte      // Text records for offset calculation
	recordSizes  []int        // Size of each text record
	totalLength  int          // Total uncompressed text length
}

// TOCEntry represents a TOC entry with position info
type TOCEntry struct {
	Label       string
	Href        string
	Level       int
	ParentIndex int
	Offset      uint32 // Byte offset in text
	Length      uint32 // Length of section
}

// NewTOCIndexBuilder creates a new TOC index builder
func NewTOCIndexBuilder() *TOCIndexBuilder {
	return &TOCIndexBuilder{
		indx:    NewINDX(0), // INDX type for TOC
		entries: make([]TOCEntry, 0),
		textRecords: make([][]byte, 0),
		recordSizes: make([]int, 0),
	}
}

// SetTextRecords sets the text records for offset calculation
func (b *TOCIndexBuilder) SetTextRecords(records [][]byte) {
	b.textRecords = records
	b.recordSizes = make([]int, len(records))
	b.totalLength = 0
	for i, rec := range records {
		b.recordSizes[i] = len(rec)
		b.totalLength += len(rec)
	}
}

// AddEntry adds a TOC entry with offset
func (b *TOCIndexBuilder) AddEntry(label, href string, level, offset uint32) {
	entry := TOCEntry{
		Label:  label,
		Href:   href,
		Level:  int(level),
		Offset: offset,
	}

	// Find parent (last entry with lower level)
	parentIndex := -1
	for i := len(b.entries) - 1; i >= 0; i-- {
		if b.entries[i].Level < int(level) {
			parentIndex = i
			break
		}
	}
	entry.ParentIndex = parentIndex

	b.entries = append(b.entries, entry)
}

// CalculateRecordOffset calculates which record a byte offset falls into
func (b *TOCIndexBuilder) CalculateRecordOffset(offset uint32) (recordIndex int, recordOffset uint32) {
	if len(b.recordSizes) == 0 {
		return 0, 0
	}

	running := uint32(0)
	for i, size := range b.recordSizes {
		recordEnd := running + uint32(size)
		if offset < recordEnd {
			return i, offset - running
		}
		running = recordEnd
	}

	// If offset is beyond all records, put in last record
	return len(b.recordSizes) - 1, offset - running
}

// Build builds the INDX structure with proper offsets
func (b *TOCIndexBuilder) Build() (*INDX, error) {
	// Add TAGX tags for TOC
	b.indx.TAGX.AddTag(1, 1, 0x01) // Name/label (string reference)
	b.indx.TAGX.AddTag(2, 1, 0x01) // Offset/position
	b.indx.TAGX.AddTag(3, 1, 0x01) // Level
	b.indx.TAGX.AddTag(4, 1, 0x01) // Parent index

	// Add each entry with record tracking
	for _, entry := range b.entries {
		// Calculate which record this entry appears in
		recordIndex, _ := b.CalculateRecordOffset(entry.Offset)

		// Add label to CNCX
		labelIndex := b.indx.AddString(entry.Label)

		// Build tag values
		tagValues := map[uint32][]uint32{
			1: {uint32(labelIndex)},
			2: {entry.Offset},
			3: {uint32(entry.Level)},
			4: {uint32(entry.ParentIndex)},
		}

		b.indx.AddEntry(entry.Offset, recordIndex, tagValues)
	}

	return b.indx, nil
}

// GetEntries returns the TOC entries
func (b *TOCIndexBuilder) GetEntries() []TOCEntry {
	return b.entries
}

// SortTOC sorts TOC entries by offset
func SortTOC(entries []TOCEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Offset < entries[j].Offset
	})
}

// CalculateOffsetsFromHTML scans HTML for TOC anchors and calculates offsets
func (b *TOCIndexBuilder) CalculateOffsetsFromHTML(html string) error {
	// Find all anchor tags with IDs
	// This is a simplified implementation
	// In production, would use proper HTML parser

	anchorPattern := `<[^>]+id=['"]([^'"]+)['"]`
	re := regexp.MustCompile(anchorPattern)
	matches := re.FindAllStringSubmatchIndex(html, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		// Calculate offset (position of the anchor)
		offset := uint32(match[0])

		// Extract ID (match[2] and match[3] are the start and end of the capture group)
		id := html[match[2]:match[3]]

		// Update existing entry or add new one
		found := false
		for i, entry := range b.entries {
			if entry.Href == "#"+id {
				b.entries[i].Offset = offset
				found = true
				break
			}
		}

		if !found {
			// Add new entry with this ID
			b.AddEntry(id, "#"+id, 1, offset)
		}
	}

	return nil
}

// FindOffsetForHref finds the byte offset of a given href in the HTML
func (b *TOCIndexBuilder) FindOffsetForHref(html, href string) uint32 {
	// Remove # prefix if present
	targetID := href
	if strings.HasPrefix(href, "#") {
		targetID = href[1:]
	}

	// Pattern to match id="..." or id='...'
	patterns := []string{
		fmt.Sprintf(`<[^>]+id=['"]%s['"]`, regexp.QuoteMeta(targetID)),
		fmt.Sprintf(`<[^>]+name=['"]%s['"]`, regexp.QuoteMeta(targetID)),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringIndex(html)
		if match != nil {
			return uint32(match[0])
		}
	}

	// If not found, return 0 (beginning of file)
	return 0
}
