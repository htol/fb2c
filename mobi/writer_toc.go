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
			// Fallback to single record if NCX encoding fails
			w.options.Logger.Debug("NCX encoding failed, using single INDX",
				"error", err.Error(),
			)
			indxData, encErr := tocINDX.Encode()
			if encErr != nil {
				return 0, fmt.Errorf("failed to encode TOC INDX: %w", encErr)
			}
			tocIndexOffset = uint32(*recordIndex) //nolint:gosec // Index fits
			palmWriter.AddRecord(indxData, 0, tocIndexOffset)
			*recordIndex++
		} else {
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
	}
	return tocIndexOffset, nil
}
