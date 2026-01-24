// Package mobi provides MOBI file generation.
package mobi

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
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
		NumRecords:         uint16(numRecords), //nolint:gosec // 64K record limit is standard for PDB
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

// PalmDBWriter writes a PalmDB file
type PalmDBWriter struct {
	name          string
	header        *PalmDBHeader
	records       [][]byte
	recordEntries []RecordIndexEntry
	logger        *slog.Logger
}

// NewPalmDBWriter creates a new PalmDB writer
func NewPalmDBWriter(name string, logger *slog.Logger) *PalmDBWriter {
	return &PalmDBWriter{
		name:          name,
		records:       make([][]byte, 0),
		recordEntries: make([]RecordIndexEntry, 0),
		logger:        logger,
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
		// Note: record entry offset is not updated here; it's recalculated in Write()
	}
}

// Write writes the complete PalmDB file
func (w *PalmDBWriter) Write(output io.Writer) error {
	// Update header with actual record count
	w.header = NewPalmDBHeader(w.name, len(w.records))

	// Calculate record offsets (header + index + offset after them)
	dataOffset := PalmDBHeaderSize + (len(w.recordEntries) * 8)

	w.logger.Debug("PalmDBWriter.Write: calculating offsets",
		"component", "PalmDBWriter",
		"operation", "Write",
		"dataOffset", dataOffset,
		"numRecords", len(w.records),
		"numEntries", len(w.recordEntries),
		"dataOffsetUint32", uint32(dataOffset), //nolint:gosec // Overflow checked below
		"dataOffsetHex", fmt.Sprintf("0x%x", uint32(dataOffset)), //nolint:gosec // Overflow checked below
	)

	if dataOffset > 2147483647 { // Max safe value for adding 4096
		w.logger.Error("dataOffset overflow detected",
			"component", "PalmDBWriter",
			"operation", "Write",
			"dataOffset", dataOffset,
			"maxSafeValue", 2147483647,
		)
		return fmt.Errorf("record offset overflow: %d", dataOffset)
	}

	// Update record entries with offsets
	for i := range w.recordEntries {
		if i == 0 {
			w.logger.Debug("PalmDBWriter.Write: calculating offsets",
				"component", "PalmDBWriter",
				"operation", "Write",
				"dataOffset", dataOffset,
				"numRecords", len(w.records),
				"numEntries", len(w.recordEntries),
				"dataOffsetUint32", uint32(dataOffset), //nolint:gosec // Overflow checked
				"dataOffsetHex", fmt.Sprintf("0x%x", uint32(dataOffset)), //nolint:gosec // Overflow checked
			)
		}
		w.recordEntries[i].Offset = uint32(dataOffset) //nolint:gosec // Offset fits
		recordLen := len(w.records[i])
		dataOffset += recordLen
		if i == 0 {
			w.logger.Debug("PalmDBWriter.Write: calculating offsets",
				"component", "PalmDBWriter",
				"operation", "Write",
				"dataOffset", dataOffset,
				"numRecords", len(w.records),
				"numEntries", len(w.recordEntries),
				"dataOffsetUint32", uint32(dataOffset), //nolint:gosec // Overflow checked
				"dataOffsetHex", fmt.Sprintf("0x%x", uint32(dataOffset)), //nolint:gosec // Overflow checked
			)
		}
		if i == 1 {
			w.logger.Debug("PalmDBWriter.Write: calculating offsets",
				"component", "PalmDBWriter",
				"operation", "Write",
				"dataOffset", dataOffset,
				"numRecords", len(w.records),
				"numEntries", len(w.recordEntries),
				"dataOffsetUint32", uint32(dataOffset), //nolint:gosec // Overflow checked
				"dataOffsetHex", fmt.Sprintf("0x%x", uint32(dataOffset)), //nolint:gosec // Overflow checked
			)
		}
		// Log final summary after all offsets are calculated
		if i == len(w.recordEntries)-1 {
			finalOffsets := make(map[int]uint32, len(w.recordEntries))
			for idx, entry := range w.recordEntries {
				finalOffsets[idx] = entry.Offset
			}
			w.logger.Debug("final record offsets calculated",
				"component", "PalmDBWriter",
				"operation", "Write",
				"totalRecords", len(w.recordEntries),
				"finalOffsets", finalOffsets,
			)
		}
	}

	// Write header
	if err := w.header.Write(output); err != nil {
		return fmt.Errorf("failed to write PalmDB header: %w", err)
	}

	if err := WriteRecordIndex(output, w.recordEntries); err != nil {
		return fmt.Errorf("failed to write record index: %w", err)
	}

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
