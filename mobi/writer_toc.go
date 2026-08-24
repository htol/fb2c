package mobi

import (
	"fmt"
)

// buildTOCRecords encodes the native TOC index records: the primary INDX
// (meta, what MOBI+0xF4 points at), the secondary INDX (entries) and the
// CNCX string table. Returns nil when no TOC is requested or the book has
// no TOC entries.
func (w *Writer) buildTOCRecords(resolvedContent string, textRecords [][]byte) ([][]byte, error) {
	if !w.options.GenerateTOC || len(w.book.TOC.Children) == 0 {
		return nil, nil
	}

	tocINDX, err := GenerateTOCIndex(w.book, resolvedContent, textRecords, w.options.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOC index: %w", err)
	}

	// Every entry filtered out (e.g. all sections lack a <title>): the book
	// has no navigable TOC, so emit it without TOC records rather than fail.
	if len(tocINDX.IDXT) == 0 {
		return nil, nil
	}

	// No root entry: Calibre's first INDX entry is the book's first section,
	// and the Kindle firmware rejects an index that starts with a root entry
	// carrying offset 0 (the native TOC then shows only Begin/Cover/End).
	// Verified on-device 2026-08-18: dropping the root restores the chapter
	// list.
	tocINDX.RootTitle = ""

	ncxResult, err := tocINDX.EncodeNCXIndex()
	if err != nil {
		// Fail loudly: the old single-record fallback emitted a non-conforming
		// CNCX (NUL-terminated strings, sequential tag-3 indexes instead of
		// byte offsets) — worse than no TOC at all.
		return nil, fmt.Errorf("failed to encode NCX index: %w", err)
	}

	records := [][]byte{ncxResult.PrimaryINDX, ncxResult.SecondaryINDX}
	if len(ncxResult.CNCXRecord) > 0 {
		records = append(records, ncxResult.CNCXRecord)
	}

	w.options.Logger.Debug("NCX TOC encoded",
		"component", "MOBIWriter",
		"operation", "BuildTOCRecords",
		"recordCount", len(records),
		"totalEntries", ncxResult.TotalEntries,
	)
	return records, nil
}
