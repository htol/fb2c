// Package mobi provides MOBI header generation.
package mobi

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// MOBI header constants
	MOBIHeaderSize = 232
	MOBIVersion    = 6 // MOBI 6

	// Compression types
	NoCompression      = 1
	PalmDOCCompression = 2
	HuffCDCompression  = 17480

	// Text encoding
	UTF8Encoding = 65001 // CP65001 = UTF-8
	Latin1Encoding = 1252 // CP1252 = Latin-1
)

// MOBIHeader represents the MOBI header
type MOBIHeader struct {
	// PalmDOC header (first 16 bytes)
	Compression     uint16
	Unused          uint16
	UncompressedTextSize uint32
	RecordCount     uint16
	RecordSize      uint32
	EncryptionType  uint16

	// MOBI header (remaining bytes)
	MOBIMarker       [4]byte
	HeaderLength     uint32
	MOBIType         uint32
	TextEncoding     uint32
	ID               uint32
	FormatVersion    uint32
	OrthographicIndex uint32
	OrthographicName  uint32
	InflectionIndex  uint32
	InflectionName   uint32
	IndexNames       uint32
	IndexKeys        uint32
	ExtraIndexData   uint32
	FirstNonBookIndex uint32
	FullNameOffset   uint32
	FullNameLength   uint32
	Locale           uint32
	InputLanguage    uint32
	OutputLanguage   uint32
	MinVersion       uint32
	FirstImageIndex  uint32
	HuffmanRecordOffset uint32
	HuffmanRecordCount uint32
	HuffmanTableOffset uint32
	HuffmanTableLength uint32
	EXTHFlags        uint32
	Unknown6         uint32
	DRMOffset        uint32
	DRMCount         uint32
	DRMKeySize       uint32
	DRMFlags         uint32
	FirstContentRec  uint16
	LastContentRec   uint16
	Unknown10        uint32
	FCISIndex        uint32
	FCISCount        uint32
	FLISIndex        uint32
	FLISCount        uint32
	Unknown11        uint32
	Unknown12        uint32
	Unknown13        uint32
	CoverIndex       uint32
	ThumbnailIndex   uint32
	Unknown14        uint32
	Unknown15        uint32
	Unknown16        uint32
	Unknown17        uint32
	Unknown18        uint32
	Unknown19        uint32
	Unknown20        uint32
	Unknown21        uint32
	Unknown22        uint32
	Unknown23        uint32
	Unknown24        uint32
	Unknown25        uint32
	Unknown26        uint32
	Unknown27        uint32
	Unknown28        uint32
	Unknown29        uint32
	Unknown30        uint32
	Unknown31        uint32
	Unknown32        uint32
	Unknown33        uint32
	Unknown34        uint32
	Unknown35        uint32
	Unknown36        uint32
	Unknown37        uint32
	Unknown38        uint32
	Unknown39        uint32
	Unknown40        uint32
	Unknown41        uint32
	Unknown42        uint32
	Unknown43        uint32
	Unknown44        uint32
	Unknown45        uint32
	Unknown46        uint32
	Unknown47        uint32
	Unknown48        uint32
	Unknown49        uint32
	Unknown50        uint32
	Unknown51        uint32
	Unknown52        uint32
	Unknown53        uint32
	Unknown54        uint32
	Unknown55        uint32
	Unknown56        uint32
	Unknown57        uint32
	Unknown58        uint32
	Unknown59        uint32
	Unknown60        uint32
	Unknown61        uint32
	Unknown62        uint32
	Unknown63        uint32
	Unknown64        uint32
	Unknown65        uint32
	Unknown66        uint32
	Unknown67        uint32
	Unknown68        uint32
	Unknown69        uint32
	Unknown70        uint32
	Unknown71        uint32
	Unknown72        uint32
	Unknown73        uint32
	Unknown74        uint32
	Unknown75        uint32
	Unknown76        uint32
	Unknown77        uint32
	Unknown78        uint32
	Unknown79        uint32
	Unknown80        uint32
	Unknown81        uint32
	Unknown82        uint32
	Unknown83        uint32
	Unknown84        uint32
	Unknown85        uint32
	Unknown86        uint32
	Unknown87        uint32
	Unknown88        uint32
	Unknown89        uint32
	Unknown90        uint32
	Unknown91        uint32
	Unknown92        uint32
	Unknown93        uint32
	Unknown94        uint32
	Unknown95        uint32
	Unknown96        uint32
	Unknown97        uint32
	Unknown98        uint32
	Unknown99        uint32
	Unknown100        uint32
	Unknown101        uint32
	Unknown102        uint32
	Unknown103        uint32
	Unknown104        uint32
	Unknown105        uint32
	Unknown106        uint32
	Unknown107        uint32
	Unknown108        uint32
	Unknown109        uint32
	Unknown110        uint32
	Unknown111        uint32
	Unknown112        uint32
	Unknown113        uint32
	Unknown114        uint32
	Unknown115        uint32
	Unknown116        uint32
	Unknown117        uint32
	Unknown118        uint32
	Unknown119        uint32
	Unknown120        uint32
	Unknown121        uint32
	Unknown122        uint32
	Unknown123        uint32
	Unknown124        uint32
	Unknown125        uint32
	Unknown126        uint32
	Unknown127        uint32
}

// NewMOBIHeader creates a new MOBI header
func NewMOBIHeader(textSize, recordCount int) *MOBIHeader {
	h := &MOBIHeader{
		// PalmDOC header
		Compression:     PalmDOCCompression, // 2 = PalmDOC compression
		Unused:          0,
		UncompressedTextSize: uint32(textSize),
		RecordCount:     uint16(recordCount),
		RecordSize:      4096, // Standard record size
		EncryptionType:  0, // No encryption

		// MOBI header
		MOBIMarker:      [4]byte{'M', 'O', 'B', 'I'},
		HeaderLength:    232, // Header size
		MOBIType:        2, // MOBI type 2 = book
		TextEncoding:    UTF8Encoding,
		ID:              generateRandomID(),
		FormatVersion:   6,
		FirstNonBookIndex: 0xFFFFFFFF, // No images yet
		FullNameOffset:  0xFFFFFFFF,
		FullNameLength:  0,
		Locale:          0, // Language/locale
		InputLanguage:   0,
		OutputLanguage:  0,
		MinVersion:      6,
		FirstImageIndex: 0xFFFFFFFF,
		EXTHFlags:       0x50, // Has EXTH header
		DRMFlags:        0, // No DRM
		FirstContentRec: 1,
		LastContentRec:  uint16(recordCount),
		CoverIndex:      0xFFFFFFFF,
		ThumbnailIndex:  0xFFFFFFFF,
	}

	// Initialize unknown fields to 0
	// In production, would set appropriate values

	return h
}

// Write writes the MOBI header to a writer
func (h *MOBIHeader) Write(w io.Writer) error {
	// Write PalmDOC header (16 bytes)
	if err := binary.Write(w, binary.BigEndian, h.Compression); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.Unused); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.UncompressedTextSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.RecordCount); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.RecordSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.EncryptionType); err != nil {
		return err
	}

	// Write MOBI marker
	if _, err := w.Write(h.MOBIMarker[:]); err != nil {
		return err
	}

	// Write remaining MOBI header fields
	fields := []uint32{
		h.HeaderLength,
		h.MOBIType,
		h.TextEncoding,
		h.ID,
		h.FormatVersion,
		h.OrthographicIndex,
		h.OrthographicName,
		h.InflectionIndex,
		h.InflectionName,
		h.IndexNames,
		h.IndexKeys,
		h.ExtraIndexData,
		h.FirstNonBookIndex,
		h.FullNameOffset,
		h.FullNameLength,
		h.Locale,
		h.InputLanguage,
		h.OutputLanguage,
		h.MinVersion,
		h.FirstImageIndex,
		h.HuffmanRecordOffset,
		h.HuffmanRecordCount,
		h.HuffmanTableOffset,
		h.HuffmanTableLength,
		h.EXTHFlags,
		h.Unknown6,
		h.DRMOffset,
		h.DRMCount,
		h.DRMKeySize,
		h.DRMFlags,
		0, // padding before FirstContentRec
	}

	for _, field := range fields {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}

	// Write FirstContentRec and LastContentRec
	if err := binary.Write(w, binary.BigEndian, h.FirstContentRec); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, h.LastContentRec); err != nil {
		return err
	}

	// Write remaining fields
	remaining := []interface{}{
		h.Unknown10, h.FCISIndex, h.FCISCount,
		h.FLISIndex, h.FLISCount,
		h.Unknown11, h.Unknown12, h.Unknown13,
		h.CoverIndex, h.ThumbnailIndex,
		h.Unknown14, h.Unknown15, h.Unknown16,
		h.Unknown17, h.Unknown18, h.Unknown19,
		h.Unknown20, h.Unknown21, h.Unknown22,
		h.Unknown23, h.Unknown24, h.Unknown25,
		h.Unknown26, h.Unknown27, h.Unknown28,
		h.Unknown29, h.Unknown30, h.Unknown31,
		h.Unknown32, h.Unknown33, h.Unknown34,
		h.Unknown35, h.Unknown36, h.Unknown37,
		h.Unknown38, h.Unknown39, h.Unknown40,
		h.Unknown41, h.Unknown42, h.Unknown43,
		h.Unknown44, h.Unknown45, h.Unknown46,
		h.Unknown47, h.Unknown48, h.Unknown49,
		h.Unknown50, h.Unknown51, h.Unknown52,
		h.Unknown53, h.Unknown54, h.Unknown55,
		h.Unknown56, h.Unknown57, h.Unknown58,
		h.Unknown59, h.Unknown60, h.Unknown61,
		h.Unknown62, h.Unknown63, h.Unknown64,
		h.Unknown65, h.Unknown66, h.Unknown67,
		h.Unknown68, h.Unknown69, h.Unknown70,
		h.Unknown71, h.Unknown72, h.Unknown73,
		h.Unknown74, h.Unknown75, h.Unknown76,
		h.Unknown77, h.Unknown78, h.Unknown79,
		h.Unknown80, h.Unknown81, h.Unknown82,
		h.Unknown83, h.Unknown84, h.Unknown85,
		h.Unknown86, h.Unknown87, h.Unknown88,
		h.Unknown89, h.Unknown90, h.Unknown91,
		h.Unknown92, h.Unknown93, h.Unknown94,
		h.Unknown95, h.Unknown96, h.Unknown97,
		h.Unknown98, h.Unknown99, h.Unknown100,
		h.Unknown101, h.Unknown102, h.Unknown103,
		h.Unknown104, h.Unknown105, h.Unknown106,
		h.Unknown107, h.Unknown108, h.Unknown109,
		h.Unknown110, h.Unknown111, h.Unknown112,
		h.Unknown113, h.Unknown114, h.Unknown115,
		h.Unknown116, h.Unknown117, h.Unknown118,
		h.Unknown119, h.Unknown120, h.Unknown121,
		h.Unknown122, h.Unknown123, h.Unknown124,
		h.Unknown125, h.Unknown126, h.Unknown127,
	}

	for _, field := range remaining {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return fmt.Errorf("failed to write MOBI header field: %w", err)
		}
	}

	return nil
}

// SetFullName sets the book full name
func (h *MOBIHeader) SetFullName(name string) {
	// In production, this would write the name to data and set offset/length
	// For now, placeholder
	h.FullNameLength = uint32(len(name))
}

// SetEXTHFlags sets the EXTH flags
func (h *MOBIHeader) SetEXTHFlags(flags uint32) {
	h.EXTHFlags = flags
}

// SetCoverIndex sets the cover image index
func (h *MOBIHeader) SetCoverIndex(index uint32) {
	h.CoverIndex = index
}

// SetContentRecords sets the first and last content record indices
func (h *MOBIHeader) SetContentRecords(first, last uint16) {
	h.FirstContentRec = first
	h.LastContentRec = last
}
