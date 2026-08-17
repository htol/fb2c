// Package epub provides EPUB file generation.
package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	"github.com/htol/fb2c/opf"
)

// Regex to match id attributes: id="value" or id='value'
var idRegex = regexp.MustCompile(`id=["']([^"']+)["']`)

// Writer writes EPUB files
type Writer struct {
	book         *opf.OEBBook
	bookID       string
	ocfPath      string   // Default: OEBPS
	tocFragments []string // Fragment IDs generated for TOC entries
	playOrder    int      // Counter for NCX playOrder
}

// NewWriter creates a new EPUB writer
func NewWriter(book *opf.OEBBook) *Writer {
	return &Writer{
		book:    book,
		bookID:  generateBookID(book),
		ocfPath: "OEBPS",
	}
}

// Write writes the EPUB file to a writer
func (w *Writer) Write(output io.Writer) error {
	zipWriter := zip.NewWriter(output)
	defer zipWriter.Close()

	if err := w.writeMimetype(zipWriter); err != nil {
		return fmt.Errorf("failed to write mimetype: %w", err)
	}

	if err := w.writeContainer(zipWriter); err != nil {
		return fmt.Errorf("failed to write container.xml: %w", err)
	}

	if err := w.writeOPF(zipWriter); err != nil {
		return fmt.Errorf("failed to write content.opf: %w", err)
	}

	if err := w.writeNCX(zipWriter); err != nil {
		return fmt.Errorf("failed to write toc.ncx: %w", err)
	}

	if err := w.writeContent(zipWriter); err != nil {
		return fmt.Errorf("failed to write content.xhtml: %w", err)
	}

	if err := w.writeResources(zipWriter); err != nil {
		return fmt.Errorf("failed to write resources: %w", err)
	}

	return nil
}

func (w *Writer) writeMimetype(zipWriter *zip.Writer) error {
	header := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("application/epub+zip"))
	return err
}

// writeContainer writes META-INF/container.xml
func (w *Writer) writeContainer(zipWriter *zip.Writer) error {
	const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="%s/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

	writer, err := zipWriter.Create("META-INF/container.xml")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, containerXML, w.ocfPath)
	return err
}

// writeOPF writes the content.opf file
func (w *Writer) writeOPF(zipWriter *zip.Writer) error {
	var buf bytes.Buffer

	// Header - use EPUB 2.0 for simpler compatibility
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
`)

	// Metadata
	w.writeMetadata(&buf)

	// Manifest
	w.writeManifest(&buf)

	// Spine
	w.writeSpine(&buf)

	// Footer
	buf.WriteString(`</package>
`)

	writer, err := zipWriter.Create(fmt.Sprintf("%s/content.opf", w.ocfPath))
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(writer)
	return err
}

// writeMetadata writes the metadata section of content.opf
func (w *Writer) writeMetadata(buf *bytes.Buffer) {
	m := w.book.Metadata

	buf.WriteString(`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
`)

	// Identifier (required)
	buf.WriteString(fmt.Sprintf(`    <dc:identifier id="bookid">%s</dc:identifier>
`, w.bookID))

	// Title
	if m.Title != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:title>%s</dc:title>
`, html.EscapeString(m.Title)))
	}

	// Authors
	for _, author := range m.Authors {
		buf.WriteString(fmt.Sprintf(`    <dc:creator>%s</dc:creator>
`, html.EscapeString(author.FullName)))
	}

	// Publisher
	if m.Publisher != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:publisher>%s</dc:publisher>
`, html.EscapeString(m.Publisher)))
	}

	// ISBN
	if m.ISBN != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:identifier>urn:isbn:%s</dc:identifier>
`, html.EscapeString(m.ISBN)))
	}

	// Date/Year
	if !m.PubDate.IsZero() {
		year := m.PubDate.Year()
		month := m.PubDate.Month()
		day := m.PubDate.Day()
		buf.WriteString(fmt.Sprintf(`    <dc:date>%04d-%02d-%02d</dc:date>
`, year, month, day))
	} else if m.Year != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:date>%s</dc:date>
`, html.EscapeString(m.Year)))
	}

	// Language
	if m.Language != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:language>%s</dc:language>
`, html.EscapeString(m.Language)))
	}

	// Annotation (description)
	if m.Annotation != "" {
		buf.WriteString(`    <dc:description>
`)
		// Indent each line of annotation
		lines := strings.Split(m.Annotation, "\n")
		for _, line := range lines {
			buf.WriteString(fmt.Sprintf("      %s\n", html.EscapeString(line)))
		}
		buf.WriteString(`    </dc:description>
`)
	}

	// Cover
	if m.CoverID != "" {
		coverID := "cover-" + m.CoverID
		buf.WriteString(fmt.Sprintf(`    <meta name="cover" content="%s"/>
`, coverID))
	}

	buf.WriteString(`  </metadata>
`)
}

// writeManifest writes the manifest section of content.opf
func (w *Writer) writeManifest(buf *bytes.Buffer) {
	buf.WriteString(`  <manifest>
`)

	// NCX (navigation) - must use application/x-dtbncx+xml for EPUB 2.0
	buf.WriteString(`    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
`)

	// Content
	buf.WriteString(`    <item id="content" href="content.xhtml" media-type="application/xhtml+xml"/>
`)

	// Resources (images, etc.)
	ids := w.book.GetManifestIDs()
	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}
		// Add prefix for resource IDs
		itemID := "res-" + id
		href := id // Already includes subdirectory if any (e.g., Images/cover.jpg)
		buf.WriteString(fmt.Sprintf(`    <item id="%s" href="%s" media-type="%s"/>
`, itemID, href, res.MediaType))
	}

	buf.WriteString(`  </manifest>
`)
}

// writeSpine writes the spine section of content.opf
func (w *Writer) writeSpine(buf *bytes.Buffer) {
	buf.WriteString(`  <spine toc="ncx">
`)

	// Main content
	buf.WriteString(`    <itemref idref="content"/>
`)

	buf.WriteString(`  </spine>
`)
}

// writeNCX writes the toc.ncx file
func (w *Writer) writeNCX(zipWriter *zip.Writer) error {
	var buf bytes.Buffer

	// Reset and collect fragment IDs
	w.tocFragments = nil

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content="`)
	buf.WriteString(w.bookID)
	buf.WriteString(`"/>
    <meta name="dtb:depth" content="3"/>
    <meta name="dtb:totalPageCount" content="0"/>
    <meta name="dtb:maxPageNumber" content="0"/>
  </head>
  <docTitle>
    <text>`)
	buf.WriteString(html.EscapeString(w.book.Metadata.Title))
	buf.WriteString(`</text>
  </docTitle>
  <navMap>
`)

	// Build TOC from book's TOC structure
	// Only add top-level entries to avoid duplicate target errors
	for _, child := range w.book.TOC.Children {
		w.writeTOCEntries(&buf, child, 1)
	}

	buf.WriteString(`  </navMap>
</ncx>
`)

	writer, err := zipWriter.Create(fmt.Sprintf("%s/toc.ncx", w.ocfPath))
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(writer)
	return err
}

// writeTOCEntries recursively writes TOC entries
func (w *Writer) writeTOCEntries(buf *bytes.Buffer, entry *opf.TOCEntry, depth int) {
	if entry == nil {
		return
	}

	// Write this entry
	playOrder := w.getNextPlayOrder()
	label := html.EscapeString(entry.Label)

	// Generate unique fragment ID for each TOC entry to avoid duplicate target errors
	fragmentID := fmt.Sprintf("toc-%d", playOrder)
	href := fmt.Sprintf("content.xhtml#%s", fragmentID)

	// Collect fragment ID for later injection into HTML
	w.tocFragments = append(w.tocFragments, fragmentID)

	buf.WriteString(fmt.Sprintf(`    <navPoint id="navPoint-%d" playOrder="%d">
      <navLabel>
        <text>%s</text>
      </navLabel>
      <content src="%s"/>
`, playOrder, playOrder, label, href))

	// Write children (indented)
	for _, child := range entry.Children {
		w.writeTOCEntries(buf, child, depth+1)
	}

	buf.WriteString(`    </navPoint>
`)
}

func (w *Writer) getNextPlayOrder() int {
	w.playOrder++
	return w.playOrder
}

// rewriteDuplicateIDs finds and rewrites duplicate IDs in HTML content
func (w *Writer) rewriteDuplicateIDs(html string) string {
	// Find all id attributes in the HTML
	idCounts := make(map[string]int)

	// Pattern to find id="value" or id='value'
	matches := idRegex.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			id := match[1]
			idCounts[id]++
		}
	}

	// If no duplicates, return original
	hasDuplicates := false
	for _, count := range idCounts {
		if count > 1 {
			hasDuplicates = true
			break
		}
	}
	if !hasDuplicates {
		return html
	}

	// Now replace IDs in HTML, tracking occurrences
	occurrences := make(map[string]int)
	result := idRegex.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the ID value from this match
		parts := idRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		id := parts[1]

		occurrences[id]++
		occurrence := occurrences[id]

		if idCounts[id] > 1 && occurrence > 1 {
			// This is a duplicate ID (not the first occurrence)
			newID := fmt.Sprintf("%s-%d", id, occurrence)
			// Preserve the original quote style
			quoteChar := "'"
			if strings.Contains(match, `"`) {
				quoteChar = `"`
			}
			return fmt.Sprintf(`id=%s%s%s`, quoteChar, newID, quoteChar)
		}
		return match
	})

	return result
}

// writeContent writes the main content XHTML file
func (w *Writer) writeContent(zipWriter *zip.Writer) error {
	content := w.book.Content

	// Content from FB2 transformer has full HTML structure
	// For EPUB 2.0, we need to extract just the body content and wrap in proper XHTML
	// Remove the outer HTML/DOCTYPE and keep only body content
	xhtml := w.convertToXHTML(content)

	// Fix any duplicate IDs in the content
	xhtml = w.rewriteDuplicateIDs(xhtml)

	writer, err := zipWriter.Create(fmt.Sprintf("%s/content.xhtml", w.ocfPath))
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte(xhtml))
	return err
}

// convertToXHTML converts HTML content to XHTML format for EPUB
func (w *Writer) convertToXHTML(rawHTML string) string {
	// Simple approach: wrap in XHTML with proper namespace
	// In production, would use html/parser to extract body content

	// Check if content starts with <!DOCTYPE
	if strings.HasPrefix(rawHTML, "<!DOCTYPE") || strings.HasPrefix(rawHTML, "<html") {
		// Extract body content
		bodyStart := strings.Index(rawHTML, "<body")
		if bodyStart == -1 {
			bodyStart = strings.Index(rawHTML, "<BODY")
		}
		if bodyStart != -1 {
			// Find opening >
			bodyStart = strings.Index(rawHTML[bodyStart:], ">") + bodyStart + 1
			bodyEnd := strings.Index(rawHTML[bodyStart:], "</body")
			if bodyEnd == -1 {
				bodyEnd = strings.Index(rawHTML[bodyStart:], "</BODY")
			}
			if bodyEnd != -1 {
				bodyContent := rawHTML[bodyStart : bodyStart+bodyEnd]

				// Build body with optional anchor navigation markers
				bodyWithContent := bodyContent
				if len(w.tocFragments) > 0 {
					// Wrap anchors in a div for XHTML 1.1 compliance
					var anchorsBuilder strings.Builder
					anchorsBuilder.WriteString(`<div class="toc-anchors">`)
					for _, fragID := range w.tocFragments {
						anchorsBuilder.WriteString(fmt.Sprintf(`<span id="%s"></span>%s`, fragID, "\n"))
					}
					anchorsBuilder.WriteString(`</div>`)
					bodyWithContent = anchorsBuilder.String() + "\n" + bodyContent
				}

				// Wrap in XHTML
				return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
  <title>%s</title>
</head>
<body>
%s
</body>
</html>
`, html.EscapeString(w.book.Metadata.Title), bodyWithContent)
			}
		}
	}

	// Fallback: just add XML declaration
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
%s
`, rawHTML)
}

// writeResources writes resources (images, etc.) to the EPUB
func (w *Writer) writeResources(zipWriter *zip.Writer) error {
	ids := w.book.GetManifestIDs()
	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}

		// Create path in OEBPS directory
		// If href already has subdirectory (e.g., Images/cover.jpg), keep it
		path := fmt.Sprintf("%s/%s", w.ocfPath, id)

		writer, err := zipWriter.Create(path)
		if err != nil {
			return err
		}

		if _, err := writer.Write(res.Data); err != nil {
			return fmt.Errorf("failed to write resource %s: %w", id, err)
		}
	}

	// Write CSS if we had any (placeholder for now)
	// Could be extended to extract CSS from FB2 or use a default

	return nil
}

// fb2cNamespace is the UUIDv5 namespace for fb2c-generated identifiers:
// UUIDv5(NAMESPACE_DNS, "github.com/htol/fb2c"). Fixed so book IDs are stable
// across fb2c releases.
var fb2cNamespace = mustParseUUID("47c7651a-de61-58d9-ac07-568c89c97043")

func mustParseUUID(s string) [16]byte {
	var u [16]byte
	hex := strings.ReplaceAll(s, "-", "")
	for i := 0; i < 16; i++ {
		u[i] = hexByte(hex[2*i])<<4 | hexByte(hex[2*i+1])
	}
	return u
}

func hexByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return 0
	}
}

// normalizeIDName normalizes a string for use in identifier derivation:
// case-insensitive and insensitive to whitespace runs.
func normalizeIDName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// generateBookID returns a UUIDv5 identifier for the book: SHA-1 of the
// fb2c namespace plus the normalized title and authors. Deterministic for the
// same book, distinct between different books, so EPUB output is byte-stable.
func generateBookID(book *opf.OEBBook) string {
	var name strings.Builder
	name.WriteString(normalizeIDName(book.Metadata.Title))
	for _, author := range book.Metadata.Authors {
		name.WriteString("\x1f") // field separator: not produced by normalizeIDName
		name.WriteString(normalizeIDName(author.FullName))
	}

	h := sha1.New() //nolint:gosec // UUIDv5 mandates SHA-1; not used for security
	h.Write(fb2cNamespace[:])
	h.Write([]byte(name.String()))
	sum := h.Sum(nil)

	// Set UUID version 5 and RFC 4122 variant bits
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80

	return fmt.Sprintf("urn:uuid:%08x-%04x-%04x-%04x-%012x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// ConvertOEBToEPUB converts an OEBBook to EPUB
func ConvertOEBToEPUB(book *opf.OEBBook, output io.Writer) error {
	writer := NewWriter(book)
	return writer.Write(output)
}
