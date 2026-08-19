// Package mobi: MOBI file reading.
//
// The reader parses files produced by any conforming MOBI 6 writer, but only
// implements the features fb2c itself emits: uncompressed text records
// (compression type 1) and no trailing per-record data (ExtraRecordFlags 0).
// Anything else fails with an explicit error instead of producing garbage.

package mobi

import (
	"encoding/binary"
	"fmt"
)

// Dump is the decoded model of a MOBI file: the single structure behind the
// text and JSON serializations of `fb2c dump`.
type Dump struct {
	PalmDB  PalmDBDump   `json:"palmdb"`
	MOBI    *MOBIDump    `json:"mobi,omitempty"`
	EXTH    *EXTHDump    `json:"exth,omitempty"`
	Records []RecordDump `json:"records"`
}

// PalmDBDump holds the decoded PalmDB header fields.
type PalmDBDump struct {
	Name             string `json:"name"`
	Attributes       uint16 `json:"attributes"`
	Version          uint16 `json:"version"`
	CreationDate     uint32 `json:"creationDate"`
	ModificationDate uint32 `json:"modificationDate"`
	UniqueIDSeed     uint32 `json:"uniqueIdSeed"`
	NextRecordListID uint32 `json:"nextRecordListId"`
	NumRecords       uint16 `json:"numRecords"`
	Type             string `json:"type"`
	Creator          string `json:"creator"`
}

// MOBIDump holds the decoded MOBI header fields (record 0).
type MOBIDump struct {
	Compression          uint16 `json:"compression"`
	CompressionName      string `json:"compressionName"`
	UncompressedTextSize uint32 `json:"uncompressedTextSize"`
	RecordCount          uint16 `json:"recordCount"`
	RecordSize           uint16 `json:"recordSize"`
	TextEncoding         uint32 `json:"textEncoding"`
	UniqueID             uint32 `json:"uniqueId"`
	FileVersion          uint32 `json:"fileVersion"`
	Locale               uint32 `json:"locale"`
	FirstImageIndex      uint32 `json:"firstImageIndex"`
	FirstNonBookIndex    uint32 `json:"firstNonBookIndex"`
	FirstContentRec      uint16 `json:"firstContentRec"`
	LastContentRec       uint16 `json:"lastContentRec"`
	FCISIndex            uint32 `json:"fcisIndex"`
	FLISIndex            uint32 `json:"flisIndex"`
	INDXRecordOffset     uint32 `json:"indxRecordOffset"`
	EXTHFlags            uint32 `json:"exthFlags"`
	ExtraRecordFlags     uint32 `json:"extraRecordFlags"`
	FullName             string `json:"fullName"`
}

// EXTHDump holds the decoded EXTH header and its records.
type EXTHDump struct {
	HeaderLength uint32           `json:"headerLength"`
	RecordCount  uint32           `json:"recordCount"`
	Records      []EXTHRecordDump `json:"records"`
}

// EXTHRecordDump is one EXTH metadata record.
type EXTHRecordDump struct {
	Type   uint32 `json:"type"`
	Name   string `json:"name,omitempty"`
	Length uint32 `json:"length"`
	Text   string `json:"text,omitempty"`
	Hex    string `json:"hex,omitempty"`
}

// RecordDump describes one record: its index entry plus derived kind.
type RecordDump struct {
	Index      uint32    `json:"index"`
	Offset     uint32    `json:"offset"`
	Length     uint32    `json:"length"`
	Attributes uint8     `json:"attributes"`
	UniqueID   uint32    `json:"uniqueId"`
	Kind       string    `json:"kind"`
	INDX       *INDXDump `json:"indx,omitempty"`
}

// INDXDump holds the decoded INDX header of an index record.
type INDXDump struct {
	HeaderLength     uint32 `json:"headerLength"`
	IndexType        uint32 `json:"indexType"`
	IDXTOffset       uint32 `json:"idxtOffset"`
	Count            uint32 `json:"count"`
	Encoding         uint32 `json:"encoding"`
	Language         uint32 `json:"language"`
	TotalRecordCount uint32 `json:"totalRecordCount"`
	CNCXCount        uint32 `json:"cncxCount"`
}

// exthTypeNames maps well-known EXTH record types to readable names.
var exthTypeNames = map[uint32]string{
	EXTHAuthor:          "author",
	EXTHPublisher:       "publisher",
	EXTHImprint:         "imprint",
	EXTHDescription:     "description",
	EXTHISBN:            "isbn",
	EXTHSubject:         "subject",
	EXTHPublishedDate:   "publishedDate",
	EXTHReview:          "review",
	EXTHContributor:     "contributor",
	EXTHRights:          "rights",
	EXTHSubjectCode:     "subjectCode",
	EXTHSource:          "source",
	EXTHASIN:            "asin",
	EXTHVersion:         "version",
	EXTHSample:          "sample",
	EXTHStartReading:    "startReading",
	EXTHAdultRating:     "adultRating",
	EXTHRetailPrice:     "retailPrice",
	EXTHCurrency:        "currency",
	EXTHKF8Bounded:      "kf8Boundary",
	EXTHResourceCount:   "resourceCount",
	EXTHCreatorSoftware: "creatorSoftware",
	EXTHCoverOffset:     "coverOffset",
	EXTHThumbOffset:     "thumbnailOffset",
	EXTHHasFakeCover:    "hasFakeCover",
	EXTHK8CoverImage:    "kf8CoverImage",
	EXTHTitle:           "title",
	EXTHLanguage:        "language",
	EXTHType:            "cdeType",
	EXTHNCXOffset:       "ncxOffset",
	EXTHNCXChunkCount:   "ncxChunkCount",
	EXTHNCXFlowCount:    "ncxFlowCount",
	EXTHNCXTotalSize:    "ncxTotalSize",
}

// compressionNames maps compression type values to readable names.
var compressionNames = map[uint16]string{
	1:     "none",
	2:     "palmdoc",
	17480: "huffcdic",
}

// ReadDump parses a MOBI file into the dump model.
func ReadDump(data []byte) (*Dump, error) {
	pdb, records, err := parsePalmDB(data)
	if err != nil {
		return nil, err
	}

	dump := &Dump{PalmDB: *pdb, Records: records}

	if len(records) > 0 {
		mobiHeader, exth, err := parseRecord0(data, records[0])
		if err != nil {
			return nil, err
		}
		dump.MOBI = mobiHeader
		dump.EXTH = exth
	}

	// Classify records once the MOBI header (content record range) is known.
	for i := range dump.Records {
		dump.Records[i].Kind = classifyRecord(data, dump.Records[i], dump.MOBI)
		if dump.Records[i].Kind == "indx" {
			dump.Records[i].INDX = parseINDXHeader(recordData(data, dump.Records[i]))
		}
	}

	return dump, nil
}

// ExtractRawML returns the book text (rawml) reconstructed from the text
// records: the content the writer split across records, concatenated back.
func ExtractRawML(data []byte) (string, error) {
	_, records, err := parsePalmDB(data)
	if err != nil {
		return "", err
	}

	mobiHeader, _, err := parseRecord0(data, records[0])
	if err != nil {
		return "", err
	}

	if mobiHeader.Compression != NoCompression {
		return "", fmt.Errorf("unsupported compression type %d (%s): fb2c reader supports uncompressed records only",
			mobiHeader.Compression, mobiHeader.CompressionName)
	}
	if mobiHeader.ExtraRecordFlags != 0 {
		return "", fmt.Errorf("unsupported extra record flags 0x%X: fb2c files carry no trailing record data",
			mobiHeader.ExtraRecordFlags)
	}

	first := int(mobiHeader.FirstContentRec)
	// LastContentRec spans text PLUS image records (spec §3/§12), so the
	// text end comes from the header's text record count instead.
	last := first + int(mobiHeader.RecordCount) - 1
	if first < 1 || last >= len(records) || first > last {
		return "", fmt.Errorf("invalid content record range %d..%d for %d records",
			first, last, len(records))
	}

	rawml := make([]byte, 0, mobiHeader.UncompressedTextSize)
	for i := first; i <= last; i++ {
		rawml = append(rawml, recordData(data, records[i])...)
	}

	return string(rawml), nil
}

// DiffReport is a record-by-record comparison of two MOBI files.
type DiffReport struct {
	NumRecordsA     int          `json:"numRecordsA"`
	NumRecordsB     int          `json:"numRecordsB"`
	Records         []RecordDiff `json:"records"`
	FirstDivergence *Divergence  `json:"firstDivergence,omitempty"`
}

// RecordDiff compares record i of both files. Missing records are flagged
// via Length -1.
type RecordDiff struct {
	Index      int    `json:"index"`
	LengthA    int    `json:"lengthA"`
	LengthB    int    `json:"lengthB"`
	UniqueIDA  uint32 `json:"uniqueIdA"`
	UniqueIDB  uint32 `json:"uniqueIdB"`
	BytesEqual bool   `json:"bytesEqual"`
}

// Divergence points at the first byte that differs between two files.
// RecordA/RecordB are the record indices containing the offset (-1 when the
// byte lies in the PalmDB header or record index area).
type Divergence struct {
	Offset  uint32 `json:"offset"`
	RecordA int    `json:"recordA"`
	RecordB int    `json:"recordB"`
}

// Diff compares two MOBI files record by record and locates the first
// diverging byte.
func Diff(a, b []byte) (*DiffReport, error) {
	_, recordsA, err := parsePalmDB(a)
	if err != nil {
		return nil, fmt.Errorf("file A: %w", err)
	}
	_, recordsB, err := parsePalmDB(b)
	if err != nil {
		return nil, fmt.Errorf("file B: %w", err)
	}

	report := &DiffReport{
		NumRecordsA: len(recordsA),
		NumRecordsB: len(recordsB),
	}

	n := max(len(recordsA), len(recordsB))
	report.Records = make([]RecordDiff, n)
	for i := 0; i < n; i++ {
		rd := RecordDiff{Index: i, LengthA: -1, LengthB: -1}
		if i < len(recordsA) {
			rd.LengthA = int(recordsA[i].Length)
			rd.UniqueIDA = recordsA[i].UniqueID
		}
		if i < len(recordsB) {
			rd.LengthB = int(recordsB[i].Length)
			rd.UniqueIDB = recordsB[i].UniqueID
		}
		rd.BytesEqual = i < len(recordsA) && i < len(recordsB) &&
			bytesEqual(recordData(a, recordsA[i]), recordData(b, recordsB[i]))
		report.Records[i] = rd
	}

	if aLen, bLen := len(a), len(b); aLen != bLen || !bytesEqual(a, b) {
		report.FirstDivergence = findDivergence(a, b, recordsA, recordsB)
	}

	return report, nil
}

// parsePalmDB decodes the PalmDB header and the record index.
func parsePalmDB(data []byte) (*PalmDBDump, []RecordDump, error) {
	const pdbHeaderSize = 78
	if len(data) < pdbHeaderSize {
		return nil, nil, fmt.Errorf("file too short for PalmDB header: %d bytes", len(data))
	}

	pdb := &PalmDBDump{
		Name:             cstring(data[0:32]),
		Attributes:       binary.BigEndian.Uint16(data[32:34]),
		Version:          binary.BigEndian.Uint16(data[34:36]),
		CreationDate:     binary.BigEndian.Uint32(data[36:40]),
		ModificationDate: binary.BigEndian.Uint32(data[40:44]),
		UniqueIDSeed:     binary.BigEndian.Uint32(data[68:72]),
		NextRecordListID: binary.BigEndian.Uint32(data[72:76]),
		NumRecords:       binary.BigEndian.Uint16(data[76:78]),
		Type:             string(data[60:64]),
		Creator:          string(data[64:68]),
	}

	if pdb.Type != PalmDBType || pdb.Creator != PalmDBCreator {
		return nil, nil, fmt.Errorf("not a BOOKMOBI file: type %q creator %q", pdb.Type, pdb.Creator)
	}

	numRecords := int(pdb.NumRecords)
	indexEnd := pdbHeaderSize + numRecords*8
	if len(data) < indexEnd {
		return nil, nil, fmt.Errorf("file too short for %d record index entries: need %d bytes, have %d",
			numRecords, indexEnd, len(data))
	}

	records := make([]RecordDump, numRecords)
	for i := 0; i < numRecords; i++ {
		base := pdbHeaderSize + i*8
		records[i] = RecordDump{
			Index:      uint32(i),
			Offset:     binary.BigEndian.Uint32(data[base : base+4]),
			Attributes: data[base+4],
			UniqueID:   uint32(data[base+5])<<16 | uint32(data[base+6])<<8 | uint32(data[base+7]),
		}
	}

	// Record length runs to the next record's offset (file end for the last).
	for i := 0; i < numRecords; i++ {
		end := uint32(len(data))
		if i+1 < numRecords {
			end = records[i+1].Offset
		}
		if end < records[i].Offset || int(end) > len(data) {
			return nil, nil, fmt.Errorf("record %d has invalid offset %d (next %d, file size %d)",
				i, records[i].Offset, end, len(data))
		}
		records[i].Length = end - records[i].Offset
	}

	return pdb, records, nil
}

// parseRecord0 decodes the PalmDOC+MOBI header and, when present, the EXTH
// header from record 0.
func parseRecord0(data []byte, rec0 RecordDump) (*MOBIDump, *EXTHDump, error) {
	rec := recordData(data, rec0)
	if len(rec) < 16+4+4 { // PalmDOC header + "MOBI" + header length
		return nil, nil, fmt.Errorf("record 0 too short for MOBI header: %d bytes", len(rec))
	}
	if string(rec[16:20]) != MOBIIdentifier {
		return nil, nil, fmt.Errorf("record 0 has no MOBI marker at offset 0x10: %q", rec[16:20])
	}

	headerLength := binary.BigEndian.Uint32(rec[20:24])
	// Header length counts from the "MOBI" magic and must reach at least the
	// EXTH flags field (0x80 + 4 bytes).
	if headerLength < 0x84 {
		return nil, nil, fmt.Errorf("implausible MOBI header length %d", headerLength)
	}
	mobiEnd := 16 + int(headerLength)
	if mobiEnd > len(rec) {
		return nil, nil, fmt.Errorf("MOBI header length %d overruns record 0 (%d bytes)", headerLength, len(rec))
	}

	// Field reads are bounded by the MOBI header end: fields beyond
	// HeaderLength do not exist — an old file's record 0 physically ends with
	// its (shorter) header — and read as zero instead of slicing past the
	// record and panicking.
	u16 := func(off int) uint16 {
		if off+2 > mobiEnd {
			return 0
		}
		return binary.BigEndian.Uint16(rec[off : off+2])
	}
	u32 := func(off int) uint32 {
		if off+4 > mobiEnd {
			return 0
		}
		return binary.BigEndian.Uint32(rec[off : off+4])
	}

	h := &MOBIDump{
		Compression:          u16(0x00),
		UncompressedTextSize: u32(0x04),
		RecordCount:          u16(0x08),
		RecordSize:           u16(0x0A),
		TextEncoding:         u32(0x1C),
		UniqueID:             u32(0x20),
		FileVersion:          u32(0x24),
		Locale:               u32(0x5C),
		FirstImageIndex:      u32(0x6C),
		FirstNonBookIndex:    u32(0x50),
		FirstContentRec:      u16(0xC0),
		LastContentRec:       u16(0xC2),
		FCISIndex:            u32(0xC8),
		FLISIndex:            u32(0xD0),
		INDXRecordOffset:     u32(0xF4),
		EXTHFlags:            u32(0x80),
		// 0xF2, trailing-entry flags; 0xF0 (fill5) is always zero. Both sit
		// beyond the minimum 0x84 header and read as 0 when absent.
		ExtraRecordFlags: uint32(u16(0xF2)),
	}
	h.CompressionName = compressionNames[h.Compression]

	// Full name lives at FullNameOffset within record 0.
	if off, length := u32(0x54), u32(0x58); length > 0 {
		if int(off)+int(length) <= len(rec) {
			h.FullName = decodeMobiString(rec[off:off+length], h.TextEncoding)
		}
	}

	var exth *EXTHDump
	if h.EXTHFlags&0x40 != 0 && mobiEnd+12 <= len(rec) && string(rec[mobiEnd:mobiEnd+4]) == EXTHIdentifier {
		exth = parseEXTH(rec[mobiEnd:])
	}

	return h, exth, nil
}

// parseEXTH decodes the EXTH header and its records from record 0 data
// starting at the "EXTH" magic.
func parseEXTH(rec []byte) *EXTHDump {
	if len(rec) < 12 {
		return nil
	}
	exth := &EXTHDump{
		HeaderLength: binary.BigEndian.Uint32(rec[4:8]),
		RecordCount:  binary.BigEndian.Uint32(rec[8:12]),
	}

	pos := 12
	for i := uint32(0); i < exth.RecordCount; i++ {
		if pos+8 > len(rec) {
			break
		}
		recType := binary.BigEndian.Uint32(rec[pos : pos+4])
		recLen := binary.BigEndian.Uint32(rec[pos+4 : pos+8])
		if recLen < 8 || pos+int(recLen) > len(rec) {
			break
		}
		value := rec[pos+8 : pos+int(recLen)]

		entry := EXTHRecordDump{
			Type:   recType,
			Name:   exthTypeNames[recType],
			Length: recLen - 8,
		}
		if isPrintableASCII(value) {
			entry.Text = string(value)
		} else {
			entry.Hex = fmt.Sprintf("%x", value)
		}
		exth.Records = append(exth.Records, entry)

		pos += int(recLen)
	}

	return exth
}

// parseINDXHeader decodes the INDX header of an index record.
func parseINDXHeader(rec []byte) *INDXDump {
	if len(rec) < 0x38 { // through CNCXCount at 0x34
		return nil
	}
	u32 := func(off int) uint32 { return binary.BigEndian.Uint32(rec[off : off+4]) }
	return &INDXDump{
		HeaderLength:     u32(0x04),
		IndexType:        u32(0x0C),
		IDXTOffset:       u32(0x14),
		Count:            u32(0x18),
		Encoding:         u32(0x1C),
		Language:         u32(0x20),
		TotalRecordCount: u32(0x24),
		CNCXCount:        u32(0x34),
	}
}

// classifyRecord derives a human-readable kind for a record.
func classifyRecord(data []byte, rec RecordDump, h *MOBIDump) string {
	body := recordData(data, rec)
	if len(body) >= 4 {
		switch string(body[:4]) {
		case "FLIS":
			return "flis"
		case "FCIS":
			return "fcis"
		case "INDX":
			return "indx"
		case "FONT":
			return "font"
		}
		if len(body) >= 8 && string(body[:8]) == "BOUNDARY" {
			return "boundary"
		}
		if len(body) >= 4 && body[0] == 0xE9 && body[1] == 0x8E && body[2] == 0x0D && body[3] == 0x0A {
			return "eof"
		}
		if len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF {
			return "image-jpeg"
		}
		if len(body) >= 4 && body[1] == 'P' && body[2] == 'N' && body[3] == 'G' {
			return "image-png"
		}
		if len(body) >= 4 && string(body[:4]) == "GIF8" {
			return "image-gif"
		}
	}
	if h != nil {
		if rec.Index == 0 {
			return "mobi-header"
		}
		if rec.Index >= uint32(h.FirstContentRec) && rec.Index <= uint32(h.LastContentRec) {
			return "text"
		}
	}
	return "unknown"
}

// recordData returns the raw bytes of a record.
func recordData(data []byte, rec RecordDump) []byte {
	return data[rec.Offset : rec.Offset+rec.Length]
}

// findDivergence locates the first differing byte and the records containing it.
func findDivergence(a, b []byte, recordsA, recordsB []RecordDump) *Divergence {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			continue
		}
		return &Divergence{
			Offset:  uint32(i),
			RecordA: recordAtOffset(recordsA, uint32(i)),
			RecordB: recordAtOffset(recordsB, uint32(i)),
		}
	}
	// One file is a prefix of the other: divergence is the first extra byte.
	return &Divergence{
		Offset:  uint32(n),
		RecordA: recordAtOffset(recordsA, uint32(n)),
		RecordB: recordAtOffset(recordsB, uint32(n)),
	}
}

// recordAtOffset returns the index of the record containing the offset,
// or -1 when it falls into the header/index area (or past the end).
func recordAtOffset(records []RecordDump, offset uint32) int {
	for i, rec := range records {
		if offset >= rec.Offset && offset < rec.Offset+rec.Length {
			return i
		}
	}
	return -1
}

// cstring returns the null-terminated string within b.
func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// decodeMobiString decodes a MOBI string according to the header text encoding.
func decodeMobiString(b []byte, encoding uint32) string {
	if encoding == Latin1Encoding {
		runes := make([]rune, len(b))
		for i, c := range b {
			runes[i] = rune(c)
		}
		return string(runes)
	}
	return string(b)
}

// isPrintableASCII reports whether b is printable ASCII (safe to show as text).
func isPrintableASCII(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return len(b) > 0
}

// bytesEqual is bytes.Equal, declared locally to keep imports minimal.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
