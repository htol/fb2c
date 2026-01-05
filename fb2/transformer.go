// Package fb2 provides FB2 to HTML transformation.
package fb2

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// Transformer converts FB2 to HTML
type Transformer struct {
	parser *Parser

	// Options
	NoInlineTOC bool
	ProcessCSS  bool
	Title       string // Override title

	// CSS processing
	cssContent string

	// Output
	HTML     string
	CSS      string
	Metadata *Metadata
}

// NewTransformer creates a new FB2 transformer
func NewTransformer() *Transformer {
	return &Transformer{
		parser:     NewParser(),
		NoInlineTOC: false,
		ProcessCSS:  true,
	}
}

// Convert converts an FB2 file to HTML
func (t *Transformer) Convert(input io.Reader) (string, string, *Metadata, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return "", "", nil, fmt.Errorf("fb2: failed to read input: %w", err)
	}

	return t.ConvertBytes(data)
}

// ConvertBytes converts FB2 bytes to HTML
func (t *Transformer) ConvertBytes(data []byte) (string, string, *Metadata, error) {
	// Parse FB2
	fb2, err := t.parser.ParseBytes(data)
	if err != nil {
		return "", "", nil, err
	}

	// Extract metadata
	metadata, err := t.parser.ExtractMetadata(fb2)
	if err != nil {
		return "", "", nil, err
	}
	t.Metadata = metadata

	// Process stylesheets (if any)
	t.processStylesheets(fb2)

	// Generate HTML
	html := t.transformToHTML(fb2)

	return html, t.cssContent, metadata, nil
}

// ConvertFile converts an FB2 file to HTML
func (t *Transformer) ConvertFile(path string) (string, string, *Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", nil, fmt.Errorf("fb2: failed to read file: %w", err)
	}

	return t.ConvertBytes(data)
}

// processStylesheets extracts and processes CSS stylesheets
func (t *Transformer) processStylesheets(fb2 *FictionBook) {
	var css strings.Builder

	// In a full implementation, we'd extract and process stylesheets
	// For now, we'll just note where CSS would go

	t.cssContent = css.String()
}

// transformToHTML transforms FB2 to HTML
func (t *Transformer) transformToHTML(fb2 *FictionBook) string {
	var buf bytes.Buffer

	// HTML header
	buf.WriteString(`<!DOCTYPE html>
<html lang="` + fb2.Description.TitleInfo.Language + `">
<head>
    <meta charset="UTF-8">
    <title>` + htmlEscape(t.getDisplayTitle(fb2)) + `</title>
    <style type="text/css">
        body { text-align: justify; margin: 2em; }
        h1, h2, h3, h4, h5, h6 { font-weight: bold; page-break-before: always; }
        h1 { font-size: 160%; border: 1px solid black; background-color: #E7E7E7; padding: 0.5em; }
        h2 { font-size: 130%; border: 1px solid gray; background-color: #EEEEEE; padding: 0.5em; }
        h3 { font-size: 110%; border: 1px solid silver; background-color: #F1F1F1; padding: 0.5em; }
        h4 { font-size: 100%; border: 1px solid gray; background-color: #F4F4F4; padding: 0.5em; }
        h5 { font-size: 100%; font-style: italic; border: 1px solid gray; background-color: #F4F4F4; padding: 0.5em; }
        h6 { font-size: 100%; font-style: italic; border: 1px solid gray; background-color: #F4F4F4; padding: 0.5em; }
        .epigraph { width: 75%; margin-left: 25%; font-style: italic; }
        .subtitle { text-align: center; }
        .paragraph { text-indent: 2em; margin-top: 0; margin-bottom: 0; }
        blockquote { margin-left: 4em; margin-top: 1em; margin-right: 0.2em; }
        code { font-family: monospace; }
        table { border-collapse: collapse; margin: 1em auto; }
        td, th { border: 1px solid black; padding: 0.3em; }
    </style>
`)

	if t.cssContent != "" {
		buf.WriteString("    <link rel=\"stylesheet\" type=\"text/css\" href=\"inline-styles.css\" />\n")
	}

	buf.WriteString(`</head>
<body>
`)

	// Annotation
	if fb2.Description.TitleInfo.Annotation != nil {
		annotation := extractTextContent(fb2.Description.TitleInfo.Annotation)
		if annotation != "" {
			buf.WriteString("<div>")
			buf.WriteString(htmlEscape(annotation))
			buf.WriteString("</div>\n<hr/>\n")
		}
	}

	// Table of Contents
	if !t.NoInlineTOC {
		buf.WriteString(t.generateTOC(fb2.Body.Sections, 1))
		buf.WriteString("<hr/>\n")
	}

	// Body content
	buf.WriteString(t.renderBody(fb2.Body))

	buf.WriteString("</body>\n</html>")

	return buf.String()
}

// getDisplayTitle returns the title for display
func (t *Transformer) getDisplayTitle(fb2 *FictionBook) string {
	if t.Title != "" {
		return t.Title
	}
	return fb2.Description.TitleInfo.BookTitle
}

// generateTOC generates a table of contents
func (t *Transformer) generateTOC(sections []Section, depth int) string {
	var buf strings.Builder

	buf.WriteString("<ul>\n")

	for i, section := range sections {
		// Generate section title
		title := ""
		if section.Title != nil && len(section.Title.P) > 0 {
			title = section.Title.P[0].Text
		} else if section.Name != "" {
			title = section.Name
		}

		if title == "" {
			title = fmt.Sprintf("Section %d", i+1)
		}

		// Generate ID
		id := section.ID
		if id == "" {
			id = fmt.Sprintf("section_%d", i+1)
		}

		buf.WriteString(fmt.Sprintf("  <li><a href=\"#%s\">%s</a>", id, htmlEscape(title)))

		// Recurse for subsections
		if len(section.Sections) > 0 {
			buf.WriteString("\n")
			buf.WriteString(t.generateTOC(section.Sections, depth+1))
		}

		buf.WriteString("</li>\n")
	}

	buf.WriteString("</ul>\n")

	return buf.String()
}

// renderBody renders the body content
func (t *Transformer) renderBody(body Body) string {
	var buf strings.Builder

	buf.WriteString("<div>\n")

	// Body name if present
	if body.Name != "" {
		buf.WriteString(fmt.Sprintf("<h4 align=\"center\">%s</h4>\n", htmlEscape(body.Name)))
	}

	// Process sections
	for i, section := range body.Sections {
		buf.WriteString(t.renderSection(section, i+1))
	}

	buf.WriteString("</div>\n")

	return buf.String()
}

// renderSection renders a section
func (t *Transformer) renderSection(section Section, index int) string {
	var buf strings.Builder

	// Section ID
	id := section.ID
	if id == "" {
		id = fmt.Sprintf("section_%d", index)
	}

	buf.WriteString(fmt.Sprintf("<div id=\"%s\">\n", id))

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

	// Subsections
	for i, subsection := range section.Sections {
		buf.WriteString(t.renderSection(subsection, i+1))
	}

	buf.WriteString("</div>\n")

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
	if data, ok := t.parser.imageData[binaryID]; ok {
		// Generate data URL
		contentType := t.parser.GetImageType(binaryID)
		dataURL := fmt.Sprintf("data:%s;base64,%s",
			contentType,
			base64.StdEncoding.EncodeToString(data))
		href = dataURL
	}
	// If no image data found, keep original href (for external images)

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

	return fmt.Sprintf("<img src=\"%s\"%s%s/>\n", href, altAttr, titleAttr)
}

// getHeadingLevel determines the heading level (h1-h6) based on nesting
func (t *Transformer) getHeadingLevel(section Section) int {
	// Count ancestor sections
	depth := t.countSectionDepth(section)
	if depth > 5 {
		return 6
	}
	return depth + 1
}

// countSectionDepth counts the nesting depth of a section
func (t *Transformer) countSectionDepth(section Section) int {
	// This is a simplified version - a full implementation would track parent hierarchy
	// For now, we'll just use a heuristic
	return 1 // Default to h2 for top-level sections under body
}

// htmlEscape escapes HTML special characters
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ConvertFile is a convenience function to convert an FB2 file to HTML
func ConvertFile(path string, noTOC bool) (string, string, *Metadata, error) {
	transformer := NewTransformer()
	transformer.NoInlineTOC = noTOC

	return transformer.ConvertFile(path)
}
