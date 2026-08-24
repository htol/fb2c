package fb2

import (
	"encoding/base64"
	"fmt"
	"html"
	"strings"
)

// renderBody renders the body content
func (t *Transformer) renderBody(body Body) string {
	var buf strings.Builder

	if !t.MOBIMode {
		buf.WriteString("<div>\n")
	}

	// Add anchor for notes body (for TOC linking)
	if body.Name == notesBodyName {
		buf.WriteString("<a id=\"notes\"></a>\n")
	}

	// Body Title or Name
	displayTitle := ""
	if body.Title != nil && len(body.Title.P) > 0 {
		var titleParts []string
		for _, p := range body.Title.P {
			titleParts = append(titleParts, t.renderP(p))
		}
		displayTitle = strings.Join(titleParts, "<br/>")
	} else if body.Name != "" {
		displayTitle = html.EscapeString(body.Name)
	}

	if displayTitle != "" {
		if t.MOBIMode {
			fmt.Fprintf(&buf, "<p align=\"center\"><b>%s</b></p>\n", displayTitle)
		} else {
			fmt.Fprintf(&buf, "<h4 align=\"center\">%s</h4>\n", displayTitle)
		}
	}

	// Process sections
	isNotesBody := body.Name == notesBodyName
	for _, section := range body.Sections {
		buf.WriteString(t.renderSectionWithBackLink(section, isNotesBody))
	}

	if !t.MOBIMode {
		buf.WriteString("</div>\n")
	}

	return buf.String()
}

// renderSection renders a section
func (t *Transformer) renderSection(section Section, _ int) string {
	return t.renderSectionWithBackLink(section, false)
}

// renderSectionWithBackLink renders a section, optionally adding a back-link for notes
func (t *Transformer) renderSectionWithBackLink(section Section, isNoteSection bool) string {
	var buf strings.Builder

	// Section ID
	t.GlobalIDCounter++
	id := section.ID
	if id == "" {
		id = fmt.Sprintf("section_%d", t.GlobalIDCounter)
	}

	if t.MOBIMode {
		fmt.Fprintf(&buf, "<a id=\"%s\"></a>\n", id)
	} else {
		fmt.Fprintf(&buf, "<div id=\"%s\">\n", id)
	}

	buf.WriteString(t.renderSectionTitle(section))
	buf.WriteString(t.renderSectionContent(section))

	// Add back-link for note sections
	if isNoteSection && id != "" {
		fmt.Fprintf(&buf, " <a href=\"#noteref_%s\" class=\"noteback\">↩</a>\n", id)
	}

	// subsections (also as note sections if parent is)
	for _, subsection := range section.Sections {
		buf.WriteString(t.renderSectionWithBackLink(subsection, isNoteSection))
	}

	if !t.MOBIMode {
		buf.WriteString("</div>\n")
	}

	return buf.String()
}

func (t *Transformer) renderSectionTitle(section Section) string {
	var buf strings.Builder
	// Section title
	if section.Title != nil && len(section.Title.P) > 0 {
		// Determine heading level based on depth (h1-h6)
		level := t.getHeadingLevel(section)
		fmt.Fprintf(&buf, "<h%d>", level)

		for _, p := range section.Title.P {
			buf.WriteString(t.renderP(p))
			buf.WriteString("<br/>\n")
		}

		fmt.Fprintf(&buf, "</h%d>\n", level)
	}

	// Subtitle
	if section.Subtitle != nil {
		fmt.Fprintf(&buf, "<h5 class=\"subtitle\">%s</h5>\n", t.renderP(*section.Subtitle))
	}
	return buf.String()
}

func (t *Transformer) renderSectionContent(section Section) string {
	var buf strings.Builder
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
		fmt.Fprintf(&buf, "<code>%s</code><br/>\n", html.EscapeString(code.Text))
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
		fmt.Fprintf(&buf, "<p class=\"paragraph\">%s</p>\n", t.renderP(p))
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

	fmt.Fprintf(&buf, "<blockquote class=\"epigraph\"%s>\n", align)

	// Authors
	for _, author := range epigraph.Authors {
		fmt.Fprintf(&buf, "  <p><em>%s</em></p>\n", html.EscapeString(formatAuthorName(author)))
	}

	// Content
	for _, node := range epigraph.Content {
		fmt.Fprintf(&buf, "  <p>%s</p>\n", html.EscapeString(node.Content))
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
		fmt.Fprintf(&buf, "  <p><em>%s</em></p>\n", html.EscapeString(formatAuthorName(author)))
	}

	// Content
	for _, node := range cite.Content {
		fmt.Fprintf(&buf, "  <p>%s</p>\n", html.EscapeString(node.Content))
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
			fmt.Fprintf(&buf, "  <p><strong>%s</strong></p>\n", html.EscapeString(p.Text))
		}
	}

	// Author
	for _, author := range stanza.Author {
		fmt.Fprintf(&buf, "  <p><em>%s</em></p>\n", html.EscapeString(formatAuthorName(author)))
	}

	// Date
	if stanza.Date.Text != "" {
		fmt.Fprintf(&buf, "  <p>%s</p>\n", html.EscapeString(stanza.Date.Text))
	}

	// Verses
	for _, v := range stanza.V {
		fmt.Fprintf(&buf, "  <p>%s</p>\n", html.EscapeString(v.Text))
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
			fmt.Fprintf(&buf, " align=\"%s\"", row.Align)
		}
		buf.WriteString(">\n")

		for _, cell := range row.Cells {
			buf.WriteString("    <td")
			if cell.ColSpan > 0 {
				fmt.Fprintf(&buf, " colspan=\"%d\"", cell.ColSpan)
			}
			if cell.RowSpan > 0 {
				fmt.Fprintf(&buf, " rowspan=\"%d\"", cell.RowSpan)
			}
			if cell.Style != "" {
				fmt.Fprintf(&buf, " style=\"%s\"", html.EscapeString(cell.Style))
			}
			if cell.Class != "" {
				fmt.Fprintf(&buf, " class=\"%s\"", html.EscapeString(cell.Class))
			}
			buf.WriteString(">")

			buf.WriteString(html.EscapeString(cell.Content))

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
		alt = html.EscapeString(img.Alt)
	}
	altAttr := fmt.Sprintf(" alt=\"%s\"", alt)

	titleAttr := ""
	if img.Title != "" {
		titleAttr = fmt.Sprintf(" title=\"%s\"", html.EscapeString(img.Title))
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

// renderP renders a paragraph content, preserving inline markup if available
func (t *Transformer) renderP(p P) string {
	if p.RawXML != "" {
		return ParseInlineContent(p.RawXML)
	}
	return html.EscapeString(p.Text)
}
