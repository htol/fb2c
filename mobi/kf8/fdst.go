// Package kf8 provides FDST (Flow Division Table) generation.
package kf8

import (
	"encoding/binary"
	"fmt"
	"io"
)

// FDSTHeader represents the FDST header
type FDSTHeader struct {
	Magic      uint32 // Should be 'FDST'
	HeaderLen  uint32 // Length of header
	NumEntries uint32 // Number of flow division entries
}

// FDSTEntry represents a flow division entry
type FDSTEntry struct {
	Start uint32 // Start offset of division
	End   uint32 // End offset of division
}

// FDST represents the complete Flow Division Table
type FDST struct {
	Header  FDSTHeader
	Entries []FDSTEntry
}

// NewFDST creates a new FDST
func NewFDST() *FDST {
	return &FDST{
		Entries: make([]FDSTEntry, 0),
	}
}

// AddEntry adds a flow division entry
func (f *FDST) AddEntry(start, end uint32) {
	f.Entries = append(f.Entries, FDSTEntry{
		Start: start,
		End:   end,
	})
	f.Header.NumEntries++
}

// AddChunk adds a chunk as a flow division
func (f *FDST) AddChunk(chunk *Chunk) {
	f.AddEntry(uint32(chunk.Offset), uint32(chunk.Offset+chunk.Length))
}

// Write writes the FDST to a writer
func (f *FDST) Write(w io.Writer) error {
	// Calculate total length
	// Header: 12 bytes
	// Each entry: 8 bytes (4 + 4)
	totalLen := 12 + (len(f.Entries) * 8)

	f.Header.Magic = 0x46545354 // 'FDST' in big-endian
	f.Header.HeaderLen = uint32(totalLen)

	// Write header
	if err := binary.Write(w, binary.BigEndian, f.Header.Magic); err != nil {
		return fmt.Errorf("failed to write FDST magic: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, f.Header.HeaderLen); err != nil {
		return fmt.Errorf("failed to write FDST header length: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, f.Header.NumEntries); err != nil {
		return fmt.Errorf("failed to write FDST entry count: %w", err)
	}

	// Write entries
	for _, entry := range f.Entries {
		if err := binary.Write(w, binary.BigEndian, entry.Start); err != nil {
			return fmt.Errorf("failed to write FDST start: %w", err)
		}
		if err := binary.Write(w, binary.BigEndian, entry.End); err != nil {
			return fmt.Errorf("failed to write FDST end: %w", err)
		}
	}

	return nil
}

// GenerateFromSkeleton generates FDST from a skeleton structure
func (f *FDST) GenerateFromSkeleton(skeleton *Skeleton) {
	for _, chunk := range skeleton.Chunks {
		f.AddChunk(chunk)
	}
}

// GenerateFromFlows generates FDST from flow manager
func (f *FDST) GenerateFromFlows(fm *FlowManager) error {
	offset := uint32(0)

	for _, flow := range fm.GetFlows() {
		end := offset + uint32(len(flow.Content))
		f.AddEntry(offset, end)
		offset = end
	}

	return nil
}

// GetTotalLength returns the total FDST length in bytes
func (f *FDST) GetTotalLength() int {
	return 12 + (len(f.Entries) * 8)
}

// GetEntryCount returns the number of entries
func (f *FDST) GetEntryCount() int {
	return len(f.Entries)
}

// FindEntryByOffset finds the FDST entry containing a given offset
func (f *FDST) FindEntryByOffset(offset uint32) (int, *FDSTEntry, bool) {
	for i, entry := range f.Entries {
		if offset >= entry.Start && offset < entry.End {
			return i, &entry, true
		}
	}
	return 0, nil, false
}

// GetOffsetRange returns the total offset range covered by FDST
func (f *FDST) GetOffsetRange() (uint32, uint32) {
	if len(f.Entries) == 0 {
		return 0, 0
	}
	return f.Entries[0].Start, f.Entries[len(f.Entries)-1].End
}

// Validate validates the FDST structure
func (f *FDST) Validate() error {
	// Check entries are in order and non-overlapping
	for i := 0; i < len(f.Entries)-1; i++ {
		current := f.Entries[i]
		next := f.Entries[i+1]

		if current.End > next.Start {
			return fmt.Errorf("FDST entries %d and %d overlap", i, i+1)
		}
	}

	// Check start < end for each entry
	for i, entry := range f.Entries {
		if entry.Start >= entry.End {
			return fmt.Errorf("FDST entry %d has start >= end", i)
		}
	}

	return nil
}

// MergeEntries merges consecutive entries
func (f *FDST) MergeEntries(maxGap uint32) {
	if len(f.Entries) <= 1 {
		return
	}

	merged := make([]FDSTEntry, 0)
	merged = append(merged, f.Entries[0])

	for i := 1; i < len(f.Entries); i++ {
		last := &merged[len(merged)-1]
		current := f.Entries[i]

		// Merge if gap is small enough
		if current.Start-last.End <= maxGap {
			last.End = current.End
		} else {
			merged = append(merged, current)
		}
	}

	f.Entries = merged
	f.Header.NumEntries = uint32(len(f.Entries))
}
