// Package epub provides EPUB file generation.
package epub

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/htol/fb2c/opf"
)

// Regex to match id attributes: id="value" or id='value'
var idRegex = regexp.MustCompile(`id=["']([^"']+)["']`)

// EPUBWriter writes EPUB files
type EPUBWriter struct {
	book       *opf.OEBBook
	bookID     string
	uuid       string
	ocfPath    string // Default: OEBPS
	tocFragments []string // Fragment IDs generated for TOC entries
}

// NewEPUBWriter creates a new EPUB writer
func NewEPUBWriter(book *opf.OEBBook) *EPUBWriter {
	return &EPUBWriter{
		book:    book,
		bookID:  generateUUID(),
		uuid:    generateUUID(),
		ocfPath: "OEBPS",
	}
}

// Write writes the EPUB file to a writer
func (w *EPUBWriter) Write(output io.Writer) error {
	// Create ZIP writer
	zipWriter := zip.NewWriter(output)
	defer zipWriter.Close()

	// 1. Write mimetype (must be first, uncompressed)
	if err := w.writeMimetype(zipWriter); err != nil {
		return fmt.Errorf("failed to write mimetype: %w", err)
	}

	// 2. Write META-INF/container.xml
	if err := w.writeContainer(zipWriter); err != nil {
		return fmt.Errorf("failed to write container.xml: %w", err)
	}

	// 3. Write content.opf
	if err := w.writeOPF(zipWriter); err != nil {
		return fmt.Errorf("failed to write content.opf: %w", err)
	}

	// 4. Write toc.ncx
	if err := w.writeNCX(zipWriter); err != nil {
		return fmt.Errorf("failed to write toc.ncx: %w", err)
	}

	// 5. Write content XHTML
	if err := w.writeContent(zipWriter); err != nil {
		return fmt.Errorf("failed to write content.xhtml: %w", err)
	}

	// 6. Write resources (images, etc.)
	if err := w.writeResources(zipWriter); err != nil {
		return fmt.Errorf("failed to write resources: %w", err)
	}

	return nil
}

// writeMimetype writes the mimetype file (must be uncompressed, first in archive)
func (w *EPUBWriter) writeMimetype(zipWriter *zip.Writer) error {
	header := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store, // Uncompressed (required for mimetype)
	}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("application/epub+zip"))
	return err
}

// writeContainer writes META-INF/container.xml
func (w *EPUBWriter) writeContainer(zipWriter *zip.Writer) error {
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
func (w *EPUBWriter) writeOPF(zipWriter *zip.Writer) error {
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
func (w *EPUBWriter) writeMetadata(buf *bytes.Buffer) {
	m := w.book.Metadata

	buf.WriteString(`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
`)

	// Identifier (required)
	buf.WriteString(fmt.Sprintf(`    <dc:identifier id="bookid">%s</dc:identifier>
`, w.bookID))

	// Title
	if m.Title != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:title>%s</dc:title>
`, escapeXML(m.Title)))
	}

	// Authors
	for _, author := range m.Authors {
		buf.WriteString(fmt.Sprintf(`    <dc:creator>%s</dc:creator>
`, escapeXML(author.FullName)))
	}

	// Publisher
	if m.Publisher != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:publisher>%s</dc:publisher>
`, escapeXML(m.Publisher)))
	}

	// ISBN
	if m.ISBN != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:identifier>urn:isbn:%s</dc:identifier>
`, escapeXML(m.ISBN)))
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
`, escapeXML(m.Year)))
	}

	// Language
	if m.Language != "" {
		buf.WriteString(fmt.Sprintf(`    <dc:language>%s</dc:language>
`, escapeXML(m.Language)))
	}

	// Annotation (description)
	if m.Annotation != "" {
		buf.WriteString(`    <dc:description>
`)
		// Indent each line of annotation
		lines := strings.Split(m.Annotation, "\n")
		for _, line := range lines {
			buf.WriteString(fmt.Sprintf("      %s\n", escapeXML(line)))
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
func (w *EPUBWriter) writeManifest(buf *bytes.Buffer) {
	buf.WriteString(`  <manifest>
`)

	// NCX (navigation) - must use application/x-dtbncx+xml for EPUB 2.0
	buf.WriteString(fmt.Sprintf(`    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
`))

	// Content
	buf.WriteString(fmt.Sprintf(`    <item id="content" href="content.xhtml" media-type="application/xhtml+xml"/>
`))

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
func (w *EPUBWriter) writeSpine(buf *bytes.Buffer) {
	buf.WriteString(`  <spine toc="ncx">
`)

	// Main content
	buf.WriteString(`    <itemref idref="content"/>
`)

	buf.WriteString(`  </spine>
`)
}

// writeNCX writes the toc.ncx file
func (w *EPUBWriter) writeNCX(zipWriter *zip.Writer) error {
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
	buf.WriteString(escapeXML(w.book.Metadata.Title))
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
func (w *EPUBWriter) writeTOCEntries(buf *bytes.Buffer, entry *opf.TOCEntry, depth int) {
	if entry == nil {
		return
	}

	// Write this entry
	playOrder := w.getNextPlayOrder()
	label := escapeXML(entry.Label)

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

var playOrderCounter int = 0

func (w *EPUBWriter) getNextPlayOrder() int {
	playOrderCounter++
	return playOrderCounter
}

// rewriteDuplicateIDs finds and rewrites duplicate IDs in HTML content
func (w *EPUBWriter) rewriteDuplicateIDs(html string) string {
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
func (w *EPUBWriter) writeContent(zipWriter *zip.Writer) error {
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
func (w *EPUBWriter) convertToXHTML(html string) string {
	// Simple approach: wrap in XHTML with proper namespace
	// In production, would use html/parser to extract body content

	// Check if content starts with <!DOCTYPE
	if strings.HasPrefix(html, "<!DOCTYPE") || strings.HasPrefix(html, "<html") {
		// Extract body content
		bodyStart := strings.Index(html, "<body")
		if bodyStart == -1 {
			bodyStart = strings.Index(html, "<BODY")
		}
		if bodyStart != -1 {
			// Find opening >
			bodyStart = strings.Index(html[bodyStart:], ">") + bodyStart + 1
			bodyEnd := strings.Index(html[bodyStart:], "</body")
			if bodyEnd == -1 {
				bodyEnd = strings.Index(html[bodyStart:], "</BODY")
			}
			if bodyEnd != -1 {
				bodyContent := html[bodyStart : bodyStart+bodyEnd]

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
`, escapeXML(w.book.Metadata.Title), bodyWithContent)
			}
		}
	}

	// Fallback: just add XML declaration
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
%s
`, html)
}

// writeResources writes resources (images, etc.) to the EPUB
func (w *EPUBWriter) writeResources(zipWriter *zip.Writer) error {
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

// escapeXML escapes special XML characters
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// generateUUID generates a random UUID for the book
func generateUUID() string {
	// Generate 16 random bytes
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		// Fallback to simple ID if random fails
		return "urn:uuid:fb2c-book-id"
	}

	// Set version (4) and variant bits
	rnd[6] = (rnd[6] & 0x0f) | 0x40 // Version 4
	rnd[8] = (rnd[8] & 0x3f) | 0x80 // Variant 1

	return fmt.Sprintf("urn:uuid:%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(rnd[0:4]),
		binary.BigEndian.Uint16(rnd[4:6]),
		binary.BigEndian.Uint16(rnd[6:8]),
		binary.BigEndian.Uint16(rnd[8:10]),
		binary.BigEndian.Uint64(rnd[8:16])&0x0FFFFFFFFFFFF)
}

// ConvertOEBToEPUB converts an OEBBook to EPUB
func ConvertOEBToEPUB(book *opf.OEBBook, output io.Writer) error {
	writer := NewEPUBWriter(book)
	return writer.Write(output)
}
