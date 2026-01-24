package fb2

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// renderBody renders the body content
func (t *Transformer) renderBody(body Body) string {
	var buf strings.Builder

	if !t.MOBIMode {
		buf.WriteString("<div>\n")
	}

	// Body name if present
	if body.Name != "" {
		if t.MOBIMode {
			buf.WriteString(fmt.Sprintf("<p align=\"center\"><b>%s</b></p>\n", htmlEscape(body.Name)))
		} else {
			buf.WriteString(fmt.Sprintf("<h4 align=\"center\">%s</h4>\n", htmlEscape(body.Name)))
		}
	}

	// Process sections
	for i, section := range body.Sections {
		buf.WriteString(t.renderSection(section, i+1))
	}

	if !t.MOBIMode {
		buf.WriteString("</div>\n")
	}

	return buf.String()
}

// renderSection renders a section
func (t *Transformer) renderSection(section Section, index int) string {
	var buf strings.Builder

	// Section ID
	t.GlobalIDCounter++
	id := section.ID
	if id == "" {
		id = fmt.Sprintf("section_%d", t.GlobalIDCounter)
	}

	if t.MOBIMode {
		buf.WriteString(fmt.Sprintf("<a id=\"%s\"></a>\n", id))
	} else {
		buf.WriteString(fmt.Sprintf("<div id=\"%s\">\n", id))
	}

	// Section title
	if section.Title != nil && len(section.Title.P) > 0 {
		// Determine heading level based on depth (h1-h6)
		level := t.getHeadingLevel(section)
		buf.WriteString(fmt.Sprintf("<h%d>", level))

		for _, p := range section.Title.P {
			buf.WriteString(htmlEscape(p.Text))
			buf.WriteString("<br/>\n")
		}

		buf.WriteString(fmt.Sprintf("</h%d>\n", level))
	}

	// Subtitle
	if section.Subtitle != nil {
		buf.WriteString(fmt.Sprintf("<h5 class=\"subtitle\">%s</h5>\n", htmlEscape(section.Subtitle.Text)))
	}

	// Epigraphs
	for _, epigraph := range section.Epigraphs {
		buf.WriteString(t.renderEpigraph(epigraph))
	}

	// Cites
	for _, cite := range section.Cite {
		buf.WriteString(t.renderCite(cite))
	}

	// Stanza (poems)
	for _, stanza := range section.Stanza {
		buf.WriteString(t.renderStanza(stanza))
	}

	// Code
	for _, code := range section.Code {
		buf.WriteString(fmt.Sprintf("<code>%s</code><br/>\n", htmlEscape(code.Text)))
	}

	// Tables
	for _, table := range section.Table {
		buf.WriteString(t.renderTable(table))
	}

	// Images
	for _, img := range section.Image {
		buf.WriteString(t.renderImage(img))
	}

	// Paragraphs
	for _, p := range section.Paragraphs {
		buf.WriteString(fmt.Sprintf("<p class=\"paragraph\">%s</p>\n", htmlEscape(p.Text)))
	}

	// subsections
	for i, subsection := range section.Sections {
		buf.WriteString(t.renderSection(subsection, i+1))
	}

	if !t.MOBIMode {
		buf.WriteString("</div>\n")
	}

	return buf.String()
}

// renderEpigraph renders an epigraph
func (t *Transformer) renderEpigraph(epigraph Epigraph) string {
	var buf strings.Builder

	align := ""
	if epigraph.TextAlign != "" {
		align = fmt.Sprintf(" align=\"%s\"", epigraph.TextAlign)
	}

	buf.WriteString(fmt.Sprintf("<blockquote class=\"epigraph\"%s>\n", align))

	// Authors
	for _, author := range epigraph.Authors {
		buf.WriteString(fmt.Sprintf("  <p><em>%s</em></p>\n", htmlEscape(formatAuthorName(author))))
	}

	// Content
	for _, node := range epigraph.Content {
		buf.WriteString(fmt.Sprintf("  <p>%s</p>\n", htmlEscape(node.Content)))
	}

	buf.WriteString("</blockquote>\n")

	return buf.String()
}

// renderCite renders a citation
func (t *Transformer) renderCite(cite Cite) string {
	var buf strings.Builder

	buf.WriteString("<blockquote>\n")

	// Authors
	for _, author := range cite.Authors {
		buf.WriteString(fmt.Sprintf("  <p><em>%s</em></p>\n", htmlEscape(formatAuthorName(author))))
	}

	// Content
	for _, node := range cite.Content {
		buf.WriteString(fmt.Sprintf("  <p>%s</p>\n", htmlEscape(node.Content)))
	}

	buf.WriteString("</blockquote>\n")

	return buf.String()
}

// renderStanza renders a poem stanza
func (t *Transformer) renderStanza(stanza Stanza) string {
	var buf strings.Builder

	buf.WriteString("<blockquote>\n")

	// Title
	if stanza.Title != nil && len(stanza.Title.P) > 0 {
		for _, p := range stanza.Title.P {
			buf.WriteString(fmt.Sprintf("  <p><strong>%s</strong></p>\n", htmlEscape(p.Text)))
		}
	}

	// Author
	for _, author := range stanza.Author {
		buf.WriteString(fmt.Sprintf("  <p><em>%s</em></p>\n", htmlEscape(formatAuthorName(author))))
	}

	// Date
	if stanza.Date.Text != "" {
		buf.WriteString(fmt.Sprintf("  <p>%s</p>\n", htmlEscape(stanza.Date.Text)))
	}

	// Verses
	for _, v := range stanza.V {
		buf.WriteString(fmt.Sprintf("  <p>%s</p>\n", htmlEscape(v.Text)))
		buf.WriteString("<br/>\n")
	}

	buf.WriteString("</blockquote>\n")

	return buf.String()
}

// renderTable renders a table
func (t *Transformer) renderTable(table Table) string {
	var buf strings.Builder

	buf.WriteString("<table>\n")

	for _, row := range table.Rows {
		buf.WriteString("  <tr")
		if row.Align != "" {
			buf.WriteString(fmt.Sprintf(" align=\"%s\"", row.Align))
		}
		buf.WriteString(">\n")

		for _, cell := range row.Cells {
			buf.WriteString("    <td")
			if cell.ColSpan > 0 {
				buf.WriteString(fmt.Sprintf(" colspan=\"%d\"", cell.ColSpan))
			}
			if cell.RowSpan > 0 {
				buf.WriteString(fmt.Sprintf(" rowspan=\"%d\"", cell.RowSpan))
			}
			if cell.Style != "" {
				buf.WriteString(fmt.Sprintf(" style=\"%s\"", htmlEscape(cell.Style)))
			}
			if cell.Class != "" {
				buf.WriteString(fmt.Sprintf(" class=\"%s\"", htmlEscape(cell.Class)))
			}
			buf.WriteString(">")

			buf.WriteString(htmlEscape(cell.Content))

			buf.WriteString("</td>\n")
		}

		buf.WriteString("  </tr>\n")
	}

	buf.WriteString("</table>\n")

	return buf.String()
}

// renderImage renders an image
func (t *Transformer) renderImage(img Image) string {
	href := img.Href
	if href == "" {
		href = img.XLinkHref
	}

	// Remove # prefix if present to get binary ID
	binaryID := strings.TrimPrefix(href, "#")

	// Check if we have image data for data URL generation
	// Only generate data URL if explicitly enabled
	if t.UseDataURLs {
		if data, ok := t.parser.imageData[binaryID]; ok {
			// Generate data URL
			contentType := t.parser.GetImageType(binaryID)
			dataURL := fmt.Sprintf("data:%s;base64,%s",
				contentType,
				base64.StdEncoding.EncodeToString(data))
			href = dataURL
		}
	} else {
		// When not using Data URLs, use the binary ID as the href
		// This ensures it matches the resource ID in the OEB book
		href = binaryID
	}
	// If no image data found and not local reference, keep original href (for external images)

	// Always include alt attribute (empty if not specified) for EPUB compliance
	alt := ""
	if img.Alt != "" {
		alt = htmlEscape(img.Alt)
	}
	altAttr := fmt.Sprintf(" alt=\"%s\"", alt)

	titleAttr := ""
	if img.Title != "" {
		titleAttr = fmt.Sprintf(" title=\"%s\"", htmlEscape(img.Title))
	}

	if t.MOBIMode {
		// MOBI 6 uses <img> tag with recindex:NNNNN
		return fmt.Sprintf("<img src=\"%s\"%s%s/>\n", href, altAttr, titleAttr)
	}

	return fmt.Sprintf("<img src=\"%s\"%s%s/>\n", href, altAttr, titleAttr)
}

// renderCoverPage renders the cover page
func (t *Transformer) renderCoverPage(cover Coverpage) string {
	img := Image{
		Href: cover.PrimaryImage.Href,
		Alt:  "Cover",
	}

	if t.MOBIMode {
		// Just the image, centered, with anchor for guide reference
		return fmt.Sprintf("<a id=\"cover\"></a>\n<p align=\"center\">%s</p>\n", t.renderImage(img))
	}

	// Render the image centered and with a page break after
	return fmt.Sprintf("<div style=\"text-align: center; page-break-after: always;\">\n%s</div>\n", t.renderImage(img))
}
