package mobi

import (
	"fmt"
)

// addTOCIndexRecords adds TOC index records to the PalmDB writer
// It returns error if fails. It modifies recordIndex.
func (w *Writer) addTOCIndexRecords(palmWriter *PalmDBWriter, recordIndex *int, resolvedContent string, textRecords [][]byte) (uint32, error) {
	tocIndexOffset := uint32(0xFFFFFFFF)
	// 3. Add TOC Index Records (NCX) - Two-record structure for native Kindle TOC
	if w.options.GenerateTOC && len(w.book.TOC.Children) > 0 {
		// Use resolvedContent for accurate TOC offset calculation
		tocINDX, err := GenerateTOCIndex(w.book, resolvedContent, textRecords, w.options.Logger)
		if err != nil {
			return 0, fmt.Errorf("failed to generate TOC index: %w", err)
		}

		// Encode as two-record NCX structure (primary + secondary)
		ncxResult, err := tocINDX.EncodeNCXIndex()
		if err != nil {
			// Fail loudly: the old single-record fallback emitted a non-conforming
			// CNCX (NUL-terminated strings, sequential tag-3 indexes instead of
			// byte offsets) — worse than no TOC at all.
			return 0, fmt.Errorf("failed to encode NCX index: %w", err)
		}
		// Add primary INDX (meta record) - this is what INDXRecordOffset points to
		tocIndexOffset = uint32(*recordIndex) //nolint:gosec // Index fits
		palmWriter.AddRecord(ncxResult.PrimaryINDX, 0, tocIndexOffset)
		*recordIndex++

		// Add secondary INDX (data record with actual TOC entries)
		palmWriter.AddRecord(ncxResult.SecondaryINDX, 0, uint32(*recordIndex)) //nolint:gosec // Index fits
		*recordIndex++

		// Add CNCX record (string table with chapter names)
		if len(ncxResult.CNCXRecord) > 0 {
			palmWriter.AddRecord(ncxResult.CNCXRecord, 0, uint32(*recordIndex)) //nolint:gosec // Index fits
			*recordIndex++
		}

		w.options.Logger.Debug("NCX TOC generated",
			"primaryRecordIndex", tocIndexOffset,
			"secondaryRecordIndex", tocIndexOffset+1,
			"cncxRecordIndex", tocIndexOffset+2,
			"totalEntries", ncxResult.TotalEntries,
		)
	}
	return tocIndexOffset, nil
}
