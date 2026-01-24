package mobi

import (
	"log/slog"
	"strings"

	"github.com/htol/fb2c/mobi/index" // Wait, I put utils in package mobi. No import needed if same package? Yes.
	"github.com/htol/fb2c/opf"
)

// GenerateTOCIndex generates a TOC index from the book's TOC with proper offsets
// and NCX-style chapter length calculation for native Kindle navigation
func GenerateTOCIndex(book *opf.OEBBook, htmlContent string, textRecords [][]byte, logger *slog.Logger) (*index.INDX, error) {
	builder := index.NewTOCIndexBuilder()
	// Set text records for offset calculation
	builder.SetTextRecords(textRecords)

	// Set root title (author + title) for NCX CNCX - matches reference format
	authors := make([]string, 0)
	for _, author := range book.Metadata.Authors {
		authors = append(authors, author.FullName)
	}
	// Note: joinStrings is now in utils.go (same package mobi)
	rootTitle := joinStrings(authors, ", ") + " " + book.Metadata.Title
	builder.SetRootTitle(rootTitle)

	// Build TOC from OEB book
	flatEntries := book.TOC.Flatten()

	for _, entry := range flatEntries {
		if entry.ID == "root" || strings.TrimSpace(entry.Label) == "" {
			continue
		}

		// Calculate offset from HTML by scanning for entry.Href
		offset := builder.FindOffsetForHref(htmlContent, entry.Href)

		logger.Debug("TOC entry offset",
			"label", entry.Label,
			"href", entry.Href,
			"offset", offset,
			"level", entry.Level,
		)

		// Add entry with calculated offset
		builder.AddEntry(entry.Label, entry.Href, uint32(entry.Level), offset) //nolint:gosec // Level fits
	}

	// Build with total text size for accurate chapter length calculation
	totalSize := uint32(len(htmlContent)) //nolint:gosec // Length fits
	return builder.BuildWithTotalSize(totalSize)
}
