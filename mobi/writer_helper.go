package mobi

// prepareTextContent resolves image sources (recindex attributes relative to
// FirstImageIndex, first image = 1) and filepos links, returning the final
// text data.
func (w *Writer) prepareTextContent() []byte {
	resolvedContent := resolveImageSources(w.book, w.options.CoverImage != nil, w.book.Content)
	// Convert href="#ID" to filepos=OFFSET for Kindle internal navigation
	resolvedContent = resolveFileposLinks(resolvedContent)
	return []byte(resolvedContent)
}
