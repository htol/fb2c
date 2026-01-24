package mobi

// prepareTextContent resolves image sources and links, returning text data and first image record index
func (w *Writer) prepareTextContent() ([]byte, int) {
	hasTOC := w.options.GenerateTOC && len(w.book.TOC.Children) > 0

	// Pass 1: Dummy resolution to get final text size
	dummyContent := resolveImageSources(w.book, w.options.CoverImage != nil, w.book.Content)
	textRecordCount := (len(dummyContent) + 4095) / 4096
	// firstImageRecord is 0-based absolute index: Header (0) + TextRecords + TOC (optional)
	firstImageRecord := 1 + textRecordCount
	if hasTOC {
		firstImageRecord++
	}

	// Pass 2: Final resolution with relative indices (1st image = 1)
	resolvedContent := resolveImageSources(w.book, w.options.CoverImage != nil, w.book.Content)
	// Convert href="#ID" to filepos=OFFSET for Kindle internal navigation
	resolvedContent = resolveFileposLinks(resolvedContent)

	return []byte(resolvedContent), firstImageRecord
}
