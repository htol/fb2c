// Package mobi provides MOBI file generation.
package mobi

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strings"
)

const (
	// PalmDB constants
	PalmDBHeaderSize = 78
	PalmDBType       = "BOOK"
	PalmDBCreator    = "MOBI"
)

// PalmDBHeader represents a Palm Database header
type PalmDBHeader struct {
	Name               [32]byte
	Attributes         uint16
	Version            uint16
	CreationDate       uint32
	ModificationDate   uint32
	LastBackupDate     uint32
	ModificationNumber uint32
	AppInfoOffset      uint32
	SortInfoOffset     uint32
	Type               [4]byte
	Creator            [4]byte
	UniqueIDSeed       uint32
	NextRecordListID   uint32
	NumRecords         uint16
}

// RecordIndexEntry represents a record index entry
type RecordIndexEntry struct {
	Offset     uint32
	Attributes uint8
	UniqueID   uint32
}

// NewPalmDBHeader creates a new PalmDB header
func NewPalmDBHeader(name string, numRecords int) *PalmDBHeader {
	h := &PalmDBHeader{
		Attributes:         0,
		Version:            0,
		CreationDate:       uint32(timestampToPalmTime(0)),
		ModificationDate:   uint32(timestampToPalmTime(0)),
		LastBackupDate:     0,
		ModificationNumber: 0,
		AppInfoOffset:      0,
		SortInfoOffset:     0,
		UniqueIDSeed:       generateRandomUniqueIDSeed(),
		NextRecordListID:   0,
		NumRecords:         uint16(numRecords),
	}

	// Transliterate name to ASCII and copy (max 31 chars + null terminator)
	// PalmDB spec requires ASCII/CP1252 in name field, not UTF-8
	asciiName := transliterateName(name)
	copy(h.Name[:], asciiName)
	if len(asciiName) > 31 {
		h.Name[31] = 0
	} else {
		h.Name[len(asciiName)] = 0
	}

	// Set type and creator
	copy(h.Type[:], PalmDBType)
	copy(h.Creator[:], PalmDBCreator)

	return h
}

// Write writes the PalmDB header to a writer
func (h *PalmDBHeader) Write(w io.Writer) error {
	// Write all fields in big-endian order
	if err := binary.Write(w, binary.BigEndian, h.Name); err != nil {
		return fmt.Errorf("failed to write name: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.Attributes); err != nil {
		return fmt.Errorf("failed to write attributes: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.Version); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.CreationDate); err != nil {
		return fmt.Errorf("failed to write creation date: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.ModificationDate); err != nil {
		return fmt.Errorf("failed to write modification date: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.LastBackupDate); err != nil {
		return fmt.Errorf("failed to write last backup date: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.ModificationNumber); err != nil {
		return fmt.Errorf("failed to write modification number: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.AppInfoOffset); err != nil {
		return fmt.Errorf("failed to write app info offset: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.SortInfoOffset); err != nil {
		return fmt.Errorf("failed to write sort info offset: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.Type); err != nil {
		return fmt.Errorf("failed to write type: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.Creator); err != nil {
		return fmt.Errorf("failed to write creator: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.UniqueIDSeed); err != nil {
		return fmt.Errorf("failed to write unique ID seed: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.NextRecordListID); err != nil {
		return fmt.Errorf("failed to write next record list ID: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, h.NumRecords); err != nil {
		return fmt.Errorf("failed to write number of records: %w", err)
	}

	return nil
}

// WriteRecordIndex writes the record index table
func WriteRecordIndex(w io.Writer, entries []RecordIndexEntry) error {
	// Each entry is 8 bytes: offset (4) + attributes (1) + uniqueID (3, but stored as uint32)
	for _, entry := range entries {
		// Pack unique ID into 3 bytes (we only use lower 24 bits)
		uniqueIDBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(uniqueIDBytes, entry.UniqueID)

		if err := binary.Write(w, binary.BigEndian, entry.Offset); err != nil {
			return fmt.Errorf("failed to write offset: %w", err)
		}
		if err := binary.Write(w, binary.BigEndian, entry.Attributes); err != nil {
			return fmt.Errorf("failed to write attributes: %w", err)
		}
		// Write 3 bytes of unique ID (big-endian, skipping first byte)
		if _, err := w.Write(uniqueIDBytes[1:4]); err != nil {
			return fmt.Errorf("failed to write unique ID: %w", err)
		}
	}

	return nil
}

// timestampToPalmTime converts Unix timestamp to Palm OS time
// Palm OS time = seconds since Jan 1, 1904
// Unix time = seconds since Jan 1, 1970
// Difference = 2082844800 seconds (66 years)
func timestampToPalmTime(unix int64) uint32 {
	// Use 0 as "now" for now
	// In production, would use time.Now().Unix()
	const unixToPalmOffset = 2082844800
	return uint32(unix + unixToPalmOffset)
}

// generateRandomUniqueIDSeed generates a random unique ID seed
func generateRandomUniqueIDSeed() uint32 {
	// Generate random number between 1 and 2^32-1
	n, _ := rand.Int(rand.Reader, big.NewInt(0xFFFFFFFF))
	return uint32(n.Uint64()) + 1
}

// generateRandomID generates a random MOBI ID
func generateRandomID() uint32 {
	// Generate random number between 1 and 2^32-1
	n, _ := rand.Int(rand.Reader, big.NewInt(0xFFFFFFFF))
	return uint32(n.Uint64()) + 1
}

// transliterateName converts Cyrillic characters to Latin transliteration
// This ensures the PalmDB name field contains only ASCII characters as required by the PalmDB spec
func transliterateName(name string) string {
	result := &strings.Builder{}

	for _, r := range name {
		if r < 128 {
			// ASCII - keep as is (but avoid null bytes)
			if r != 0 {
				result.WriteRune(r)
			}
		} else {
			// Cyrillic - map to Latin approximation
			result.WriteString(transliterateRune(r))
		}
	}

	resultStr := result.String()
	// Truncate to 31 chars max (for PalmDB name field)
	if len(resultStr) > 31 {
		resultStr = resultStr[:31]
	}

	return resultStr
}

// transliterateRune maps a single Cyrillic character to its Latin approximation
func transliterateRune(r rune) string {
	// Uppercase Cyrillic
	switch r {
	case 0x0410: // А
		return "A"
	case 0x0411: // Б
		return "B"
	case 0x0412: // В
		return "V"
	case 0x0413: // Г
		return "G"
	case 0x0414: // Д
		return "D"
	case 0x0415: // Е
		return "E"
	case 0x0401: // Ё
		return "Yo"
	case 0x0416: // Ж
		return "Zh"
	case 0x0417: // З
		return "Z"
	case 0x0418: // И
		return "I"
	case 0x0419: // Й
		return "Y"
	case 0x041A: // К
		return "K"
	case 0x041B: // Л
		return "L"
	case 0x041C: // М
		return "M"
	case 0x041D: // Н
		return "N"
	case 0x041E: // О
		return "O"
	case 0x041F: // П
		return "P"
	case 0x0420: // Р
		return "R"
	case 0x0421: // С
		return "S"
	case 0x0422: // Т
		return "T"
	case 0x0423: // У
		return "U"
	case 0x0424: // Ф
		return "F"
	case 0x0425: // Х
		return "Kh"
	case 0x0426: // Ц
		return "Ts"
	case 0x0427: // Ч
		return "Ch"
	case 0x0428: // Ш
		return "Sh"
	case 0x0429: // Щ
		return "Shch"
	case 0x042A: // Ъ
		return "\""
	case 0x042B: // Ы
		return "'"
	case 0x042C: // Ь
		return "'"
	case 0x042D: // Э
		return "E"
	case 0x042E: // Ю
		return "Yu"
	case 0x042F: // Я
		return "Ya"
	// Lowercase Cyrillic
	case 0x0430: // а
		return "a"
	case 0x0431: // б
		return "b"
	case 0x0432: // в
		return "v"
	case 0x0433: // г
		return "g"
	case 0x0434: // д
		return "d"
	case 0x0435: // е
		return "e"
	case 0x0451: // ё
		return "yo"
	case 0x0436: // ж
		return "zh"
	case 0x0437: // з
		return "z"
	case 0x0438: // и
		return "i"
	case 0x0439: // й
		return "y"
	case 0x043A: // к
		return "k"
	case 0x043B: // л
		return "l"
	case 0x043C: // м
		return "m"
	case 0x043D: // н
		return "n"
	case 0x043E: // о
		return "o"
	case 0x043F: // п
		return "p"
	case 0x0440: // р
		return "r"
	case 0x0441: // с
		return "s"
	case 0x0442: // т
		return "t"
	case 0x0443: // у
		return "u"
	case 0x0444: // ф
		return "f"
	case 0x0445: // х
		return "kh"
	case 0x0446: // ц
		return "ts"
	case 0x0447: // ч
		return "ch"
	case 0x0448: // ш
		return "sh"
	case 0x0449: // щ
		return "shch"
	case 0x044A: // ъ
		return "\""
	case 0x044B: // ы
		return "'"
	case 0x044C: // ь
		return "'"
	case 0x044D: // э
		return "e"
	case 0x044E: // ю
		return "yu"
	case 0x044F: // я
		return "ya"
	default:
		// Unknown character, use replacement
		return "?"
	}
}

// PalmDBWriter writes a PalmDB file
type PalmDBWriter struct {
	name          string
	header        *PalmDBHeader
	records       [][]byte
	recordEntries []RecordIndexEntry
	debug         bool
}

// NewPalmDBWriter creates a new PalmDB writer
func NewPalmDBWriter(name string, debug bool) *PalmDBWriter {
	return &PalmDBWriter{
		name:          name,
		records:       make([][]byte, 0),
		recordEntries: make([]RecordIndexEntry, 0),
		debug:         debug,
	}
}

// AddRecord adds a record
func (w *PalmDBWriter) AddRecord(data []byte, attributes uint8, uniqueID uint32) {
	w.records = append(w.records, data)
	w.recordEntries = append(w.recordEntries, RecordIndexEntry{
		Attributes: attributes,
		UniqueID:   uniqueID,
	})
}

// SetRecord sets the data for an existing record
func (w *PalmDBWriter) SetRecord(index int, data []byte) {
	if index >= 0 && index < len(w.records) {
		w.records[index] = data
	}
}

// Write writes the complete PalmDB file
func (w *PalmDBWriter) Write(output io.Writer) error {
	// Update header with actual record count
	w.header = NewPalmDBHeader(w.name, len(w.records))

	// Calculate record offsets (header + index + offset after them)
	dataOffset := PalmDBHeaderSize + (len(w.recordEntries) * 8)

	// Update record entries with offsets
	for i := range w.recordEntries {
		w.recordEntries[i].Offset = uint32(dataOffset)
		dataOffset += len(w.records[i])
	}

	// Write header
	if err := w.header.Write(output); err != nil {
		return fmt.Errorf("failed to write PalmDB header: %w", err)
	}

	// Write record index
	if err := WriteRecordIndex(output, w.recordEntries); err != nil {
		return fmt.Errorf("failed to write record index: %w", err)
	}

	// Write records
	for _, record := range w.records {
		if _, err := output.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	return nil
}

// SetName sets the database name
func (w *PalmDBWriter) SetName(name string) {
	if w.header != nil {
		// Transliterate name to ASCII for PalmDB compatibility
		asciiName := transliterateName(name)
		copy(w.header.Name[:], asciiName)
		if len(asciiName) > 31 {
			w.header.Name[31] = 0
		} else {
			w.header.Name[len(asciiName)] = 0
		}
	}
}

// GetRecords returns all records
func (w *PalmDBWriter) GetRecords() [][]byte {
	return w.records
}

// SetRecords sets all records and entries
func (w *PalmDBWriter) SetRecords(records [][]byte, entries []RecordIndexEntry) {
	w.records = records
	w.recordEntries = entries
}

// GetRecordEntries returns all record index entries
func (w *PalmDBWriter) GetRecordEntries() []RecordIndexEntry {
	return w.recordEntries
}
