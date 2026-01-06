// Package mobi provides MOBI header generation.
package mobi

import (
	"encoding/binary"
	"io"
)

const (
	// MOBI header constants
	MOBIHeaderSize = 232 // MOBI header size (from MOBI marker to end)
	MOBIVersion    = 6   // MOBI 6

	// Compression types
	NoCompression      = 1
	PalmDOCCompression = 2
	HuffCDCompression  = 17480

	// Text encoding
	UTF8Encoding   = 65001 // CP65001 = UTF-8
	Latin1Encoding = 1252  // CP1252 = Latin-1

	// Record sizes
	StandardRecordSize = 4096       // Standard record size for MOBI 6
	KF8JointRecordSize = 0x10000000 // Record size for KF8 joint files (bit mask)
)

// MOBIHeader represents the MOBI header (MOBI 6 format, 232 bytes from MOBI marker)
// Offsets are from record start, with MOBI marker offsets in parentheses.
// Specification: https://wiki.mobileread.com/wiki/MOBI
type MOBIHeader struct {
	// PalmDOC Header (offsets 0x00 to 0x0F from record start)
	Compression          uint16 // 0x00: 1=none, 2=PalmDOC, 17480=HuffCDIC
	Unused1              uint16 // 0x02: Always zero
	UncompressedTextSize uint32 // 0x04: Uncompressed text length
	RecordCount          uint16 // 0x08: Number of text records
	RecordSize           uint16 // 0x0A: Always 4096 (or 0x10000000 for KF8)
	EncryptionType       uint16 // 0x0C: 0=none, 1=old DRM, 2=new DRM
	Unused2              uint16 // 0x0E: Always zero

	// MOBI Header (MOBI marker at 0x10 from record start)
	MOBIMarker          [4]byte  // 0x10 (+0x00): "MOBI" magic string
	HeaderLength        uint32   // 0x14 (+0x04): MOBI header length (232)
	MOBIType            uint32   // 0x18 (+0x08): 2=book, 3=PalmDoc, 248=KF8, etc.
	TextEncoding        uint32   // 0x1C (+0x0C): 1252=CP1252, 65001=UTF-8
	UniqueID            uint32   // 0x20 (+0x10): Unique ID
	FileVersion         uint32   // 0x24 (+0x14): MOBI version (6 for MOBI 6)
	OrthographicIndex   uint32   // 0x28 (+0x18): Orthographic index section (0xFFFFFFFF if none)
	InflectionIndex     uint32   // 0x2C (+0x1C): Inflection index section (0xFFFFFFFF if none)
	IndexNames          uint32   // 0x30 (+0x20): Index names section (0xFFFFFFFF if none)
	IndexKeys           uint32   // 0x34 (+0x24): Index keys section (0xFFFFFFFF if none)
	ExtraIndex0         uint32   // 0x38 (+0x28): Extra index 0 (0xFFFFFFFF if none)
	ExtraIndex1         uint32   // 0x3C (+0x2C): Extra index 1 (0xFFFFFFFF if none)
	ExtraIndex2         uint32   // 0x40 (+0x30): Extra index 2 (0xFFFFFFFF if none)
	ExtraIndex3         uint32   // 0x44 (+0x34): Extra index 3 (0xFFFFFFFF if none)
	ExtraIndex4         uint32   // 0x48 (+0x38): Extra index 4 (0xFFFFFFFF if none)
	ExtraIndex5         uint32   // 0x4C (+0x3C): Extra index 5 (0xFFFFFFFF if none)
	FirstNonBookIndex   uint32   // 0x50 (+0x40): First non-book record index
	FullNameOffset      uint32   // 0x54 (+0x44): Offset to full name in record 0
	FullNameLength      uint32   // 0x58 (+0x48): Length of full name
	Locale              uint32   // 0x5C (+0x4C): Locale code (e.g., 1033=US English)
	InputLanguage       uint32   // 0x60 (+0x50): Input language (dictionary)
	OutputLanguage      uint32   // 0x64 (+0x54): Output language (dictionary)
	MinVersion          uint32   // 0x68 (+0x58): Minimum MOBI version needed
	FirstImageIndex     uint32   // 0x6C (+0x5C): First image record index
	HuffmanRecordOffset uint32   // 0x70 (+0x60): Huffman compression record offset
	HuffmanRecordCount  uint32   // 0x74 (+0x64): Huffman compression record count
	HuffmanTableOffset  uint32   // 0x78 (+0x68): Huffman table offset
	HuffmanTableLength  uint32   // 0x7C (+0x6C): Huffman table length
	EXTHFlags           uint32   // 0x80 (+0x70): EXTH flags (0x40 = has EXTH header)
	Unknown1            [32]byte // 0x84 (+0x74): 32 unknown bytes
	Unknown2            uint32   // 0xA4 (+0x94): Unknown (use 0xFFFFFFFF)
	DRMOffset           uint32   // 0xA8 (+0x98): DRM key info offset (0xFFFFFFFF if none)
	DRMCount            uint32   // 0xAC (+0x9C): DRM entry count (0xFFFFFFFF if none)
	DRMSize             uint32   // 0xB0 (+0xA0): DRM info size
	DRMFlags            uint32   // 0xB4 (+0xA4): DRM flags
	Unknown4            [8]byte  // 0xB8 (+0xA8): 8 unknown bytes
	FirstContentRec     uint16   // 0xC0 (+0xB0): First content record number (usually 1)
	LastContentRec      uint16   // 0xC2 (+0xB2): Last content record number
	Unknown5            uint32   // 0xC4 (+0xB4): Unknown (use 0x00000001)
	FCISIndex           uint32   // 0xC8 (+0xB8): FCIS record number
	FCISCount           uint32   // 0xCC (+0xBC): Unknown (FCIS count? use 0x00000001)
	FLISIndex           uint32   // 0xD0 (+0xC0): FLIS record number
	FLISCount           uint32   // 0xD4 (+0xC4): Unknown (FLIS count? use 0x00000001)
	Unknown216          [8]byte  // 0xD8 (+0xC8): 8 unknown bytes
	Unknown224          uint32   // 0xE0 (+0xD0): Unknown (use 0xFFFFFFFF)
	FirstCompilation    uint32   // 0xE4 (+0xD4): First Compilation data section count (use 0x00000000)
	NumCompilation      uint32   // 0xE8 (+0xD8): Number of Compilation data sections (use 0xFFFFFFFF)
	Unknown236          uint32   // 0xEC (+0xDC): Unknown (use 0xFFFFFFFF)
	ExtraRecordFlags    uint32   // 0xF0 (+0xE0): Extra record data flags (0 = no extra data)
	INDXRecordOffset    uint32   // 0xF4 (+0xE4): INDX record offset (0xFFFFFFFF if none)
}

// NewMOBIHeader creates a new MOBI header with default values
func NewMOBIHeader(textSize, recordCount int) *MOBIHeader {
	h := &MOBIHeader{
		// PalmDOC header
		Compression:          NoCompression,
		Unused1:              0,
		UncompressedTextSize: uint32(textSize),
		RecordCount:          uint16(recordCount),
		RecordSize:           StandardRecordSize,
		EncryptionType:       0,
		Unused2:              0,

		// MOBI header
		MOBIMarker:          [4]byte{'M', 'O', 'B', 'I'},
		HeaderLength:        232, // MOBI header size (0xE8) including previous 4 bytes - up to Unknown236
		MOBIType:            2,   // MOBI type 2 = book
		TextEncoding:        UTF8Encoding,
		UniqueID:            generateRandomID(),
		FileVersion:         6,
		OrthographicIndex:   0xFFFFFFFF,
		InflectionIndex:     0xFFFFFFFF,
		IndexNames:          0xFFFFFFFF,
		IndexKeys:           0xFFFFFFFF,
		ExtraIndex0:         0xFFFFFFFF,
		ExtraIndex1:         0xFFFFFFFF,
		ExtraIndex2:         0xFFFFFFFF,
		ExtraIndex3:         0xFFFFFFFF,
		ExtraIndex4:         0xFFFFFFFF,
		ExtraIndex5:         0xFFFFFFFF,
		FirstNonBookIndex:   0xFFFFFFFF,
		FullNameOffset:      0xFFFFFFFF,
		FullNameLength:      0,
		Locale:              0,
		InputLanguage:       0,
		OutputLanguage:      0,
		MinVersion:          6,
		FirstImageIndex:     0xFFFFFFFF,
		HuffmanRecordOffset: 0,
		HuffmanRecordCount:  0,
		HuffmanTableOffset:  0,
		HuffmanTableLength:  0,
		EXTHFlags:           0x40, // Has EXTH header
		Unknown1:            [32]byte{},
		Unknown2:            0xFFFFFFFF,
		DRMOffset:           0xFFFFFFFF,
		DRMCount:            0,
		DRMSize:             0,
		DRMFlags:            0,
		Unknown4:            [8]byte{},
		FirstContentRec:     1,
		LastContentRec:      uint16(recordCount),
		Unknown5:            0x00000001,
		FCISIndex:           0xFFFFFFFF,
		FCISCount:           0x00000001,
		FLISIndex:           0xFFFFFFFF,
		FLISCount:           0x00000001,
		Unknown216:          [8]byte{},
		Unknown224:          0xFFFFFFFF,
		FirstCompilation:    0x00000000,
		NumCompilation:      0xFFFFFFFF,
		Unknown236:          0xFFFFFFFF,
		ExtraRecordFlags:    0,
		INDXRecordOffset:    0xFFFFFFFF,
	}

	return h
}

// Write writes the complete MOBI header to a writer.
// The struct is properly aligned with the MOBI specification, so we can
// write it all at once with binary.Write without padding issues.
func (h *MOBIHeader) Write(w io.Writer) error {
	return binary.Write(w, binary.BigEndian, h)
}

// SetFullName sets the book full name
func (h *MOBIHeader) SetFullName(name string) {
	// In production, this would write the name to data and set offset/length
	// For now, just set the length
	h.FullNameLength = uint32(len(name))
}

// SetEXTHFlags sets the EXTH flags
func (h *MOBIHeader) SetEXTHFlags(flags uint32) {
	h.EXTHFlags = flags
}

// SetContentRecords sets the first and last content record indices
func (h *MOBIHeader) SetContentRecords(first, last uint16) {
	h.FirstContentRec = first
	h.LastContentRec = last
}
