package mapper

import (
	"log/slog"

	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/opf"
)

// FromFB2 creates an OPF OEBBook from FB2 data structures.
// It acts as an integration layer, translating FB2-specific types into generic OEB types.
func FromFB2(metadata *fb2.Metadata, html string, tocData *fb2.TOCData, fb2Doc *fb2.FictionBook) (*opf.OEBBook, error) {
	book := opf.NewOEBBook()

	// 1. Map Metadata
	book.Metadata = opf.ConvertMetadataFromFB2(
		metadata.Title,
		metadata.Authors,
		metadata.AuthorSort,
		metadata.Publisher,
		metadata.ISBN,
		metadata.Year,
		metadata.Language,
		metadata.PubDate,
		metadata.Series,
		metadata.SeriesIndex,
		metadata.Genres,
		metadata.Keywords,
		metadata.Annotation,
		metadata.Cover,
		metadata.CoverID,
		metadata.CoverExt,
	)

	// 2. Set Content
	book.Content = html

	// 3. Map TOC
	if tocData != nil && len(tocData.Entries) > 0 {
		mapTOC(tocData.Entries, &book.Metadata, &book.TOC)
	}

	// 4. Map Resources
	// First add cover if explicitly set in metadata
	if metadata.CoverID != "" && len(metadata.Cover) > 0 {
		// CoverID already includes the extension (e.g., "cover.jpg")
		book.AddResource(metadata.CoverID, metadata.CoverID,
			"image/"+metadata.CoverExt[1:], metadata.Cover)
	}

	// Add all embedded binaries as resources
	if fb2Doc != nil && len(fb2Doc.Binaries) > 0 {
		for _, binary := range fb2Doc.Binaries {
			if binary.ID == "" {
				continue
			}

			// Decode data using the helper that hides base64 details
			data, err := binary.Bytes()
			if err != nil {
				slog.Warn("mapper: failed to decode binary resource", "id", binary.ID, "error", err)
				continue
			}

			// Determine media type using standard logic
			mediaType := binary.GetContentType()

			// Use the binary ID as the resource ID
			book.AddResource(binary.ID, binary.ID, mediaType, data)
		}
	}

	return book, nil
}

// mapTOC maps a flat list of FB2 TOC entries to a hierarchical OPF TOC tree.
// It reconstructs the tree based on entry levels.
func mapTOC(fb2Entries []*fb2.TOCEntry, metadata *opf.Metadata, root *opf.TOCEntry) {
	// Initialize root
	root.ID = "root"
	root.Label = metadata.Title

	// Map to track parent entries by level.
	// entryMap[level] points to the last added entry at that level.
	entryMap := make(map[int]*opf.TOCEntry)

	for _, fb2Entry := range fb2Entries {
		// Determine where to add this entry
		// If it's a top-level entry (Level 1) or has no parent logic in FB2 structure
		if fb2Entry.Parent == nil || fb2Entry.Level == 1 {
			// Add directly to root
			child := root.AddChild(fb2Entry.ID, fb2Entry.Label, fb2Entry.Href)

			// Store as the current active parent for this level
			entryMap[fb2Entry.Level] = child
		} else {
			// Find hierarchy parent (at level N-1)
			if parent, ok := entryMap[fb2Entry.Level-1]; ok {
				child := parent.AddChild(fb2Entry.ID, fb2Entry.Label, fb2Entry.Href)

				// Store this new entry as potential parent for level N+1
				entryMap[fb2Entry.Level] = child
			} else {
				// Fallback: orphan entry or broken hierarchy. Add to root to ensure accessibility.
				child := root.AddChild(fb2Entry.ID, fb2Entry.Label, fb2Entry.Href)
				entryMap[fb2Entry.Level] = child
			}
		}
	}
}
