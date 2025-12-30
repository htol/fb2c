// Package opf provides HTML processing for OPF content.
package opf

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// HTMLProcessor processes HTML content for OEB format
type HTMLProcessor struct {
	// Options
	PrettyPrint bool
	Indent      string

	// Note/cite link resolution
	Notes     map[string]string // ID -> content
	Cites     map[string]string // ID -> content
}

// NewHTMLProcessor creates a new HTML processor
func NewHTMLProcessor() *HTMLProcessor {
	return &HTMLProcessor{
		PrettyPrint: true,
		Indent:      "    ",
		Notes:       make(map[string]string),
		Cites:       make(map[string]string),
	}
}

// Process processes HTML content and returns cleaned HTML
func (p *HTMLProcessor) Process(html string) string {
	// 1. Remove XML declaration if present
	html = p.stripXMLDeclaration(html)

	// 2. Normalize line breaks
	html = p.normalizeLineBreaks(html)

	// 3. Convert paragraph divs to p tags if needed
	html = p.convertParagraphDivs(html)

	// 4. Fix common HTML issues
	html = p.fixHTMLEncoding(html)

	// 5. Clean up empty elements
	html = p.removeEmptyElements(html)

	// 6. Normalize whitespace
	html = p.normalizeWhitespace(html)

	return html
}

// stripXMLDeclaration removes the XML declaration from HTML
func (p *HTMLProcessor) stripXMLDeclaration(html string) string {
	// Remove <?xml ...?> declaration
	re := regexp.MustCompile(`^<\?xml[^>]*\?>\s*`)
	return re.ReplaceAllString(html, "")
}

// normalizeLineBreaks normalizes line endings to \n
func (p *HTMLProcessor) normalizeLineBreaks(html string) string {
	// Convert Windows \r\n to \n
	html = strings.ReplaceAll(html, "\r\n", "\n")
	// Convert old Mac \r to \n
	html = strings.ReplaceAll(html, "\r", "\n")
	return html
}

// convertParagraphDivs converts <div class="paragraph"> to <p> tags
func (p *HTMLProcessor) convertParagraphDivs(html string) string {
	// Convert <div class="paragraph">...</div> to <p>...</p>
	re := regexp.MustCompile(`<div\s+class=["']paragraph["']\s*>(.*?)</div>`)
	html = re.ReplaceAllString(html, "<p>$1</p>")

	// Convert <div>text</div> without other content to <p>text</p>
	// Only if div contains only text (no nested divs, tables, etc.)
	re = regexp.MustCompile(`<div\s*>([^<]+)</div>`)
	html = re.ReplaceAllString(html, "<p>$1</p>")

	return html
}

// fixHTMLEncoding fixes HTML encoding issues
func (p *HTMLProcessor) fixHTMLEncoding(html string) string {
	// Fix unescaped ampersands in text (not in entities)
	// This is a simplified version - production would be more sophisticated

	// Fix common unescaped characters
	html = strings.ReplaceAll(html, " & ", " &amp; ")

	// Fix other entities that might be wrong
	html = strings.ReplaceAll(html, "&lt;", "&lt;")
	html = strings.ReplaceAll(html, "&gt;", "&gt;")
	html = strings.ReplaceAll(html, "&quot;", "&quot;")
	html = strings.ReplaceAll(html, "&apos;", "&apos;")

	return html
}

// removeEmptyElements removes empty elements
func (p *HTMLProcessor) removeEmptyElements(html string) string {
	// Remove empty p tags
	re := regexp.MustCompile(`<p>\s*</p>`)
	html = re.ReplaceAllString(html, "")

	// Remove empty div tags
	re = regexp.MustCompile(`<div>\s*</div>`)
	html = re.ReplaceAllString(html, "")

	// Remove empty span tags
	re = regexp.MustCompile(`<span>\s*</span>`)
	html = re.ReplaceAllString(html, "")

	return html
}

// normalizeWhitespace normalizes whitespace in HTML
func (p *HTMLProcessor) normalizeWhitespace(html string) string {
	// Collapse multiple spaces into one (outside of tags)
	re := regexp.MustCompile(`\s+`)
	html = re.ReplaceAllString(html, " ")

	// Remove leading/trailing whitespace from lines
	lines := strings.Split(html, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Remove completely empty lines
	result := []string{}
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// ExtractBodyContent extracts content from within <body> tags
func (p *HTMLProcessor) ExtractBodyContent(html string) string {
	// Find body content
	start := strings.Index(html, "<body>")
	if start == -1 {
		start = strings.Index(strings.ToLower(html), "<body>")
	}
	if start == -1 {
		return html // No body tag found
	}

	end := strings.LastIndex(html, "</body>")
	if end == -1 {
		end = strings.LastIndex(strings.ToLower(html), "</body>")
	}
	if end == -1 {
		return html
	}

	// Adjust start to after the body tag
	if html[start:start+5] == "<body>" {
		start += 6
	} else {
		// Find closing > of lowercase <body>
		endTag := strings.Index(html[start:], ">")
		if endTag != -1 {
			start += endTag + 1
		}
	}

	return strings.TrimSpace(html[start:end])
}

// WrapInHTML wraps content in a basic HTML structure
func (p *HTMLProcessor) WrapInHTML(content, title, lang string) string {
	if lang == "" {
		lang = "en"
	}

	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html lang="`)
	buf.WriteString(lang)
	buf.WriteString(`">
<head>
    <meta charset="UTF-8">
    <title>`)
	buf.WriteString(htmlEscape(title))
	buf.WriteString(`</title>
    <style type="text/css">
        body { font-family: serif; margin: 2em; text-align: justify; }
        h1, h2, h3, h4, h5, h6 { font-weight: bold; margin-top: 1em; margin-bottom: 0.5em; }
        h1 { font-size: 160%; border: 1px solid black; background-color: #E7E7E7; padding: 0.5em; }
        h2 { font-size: 130%; border: 1px solid gray; background-color: #EEEEEE; padding: 0.5em; }
        h3 { font-size: 110%; border: 1px solid silver; background-color: #F1F1F1; padding: 0.5em; }
        p { text-indent: 2em; margin: 0; line-height: 1.3; }
        .epigraph { width: 75%; margin-left: 25%; font-style: italic; }
        .subtitle { text-align: center; font-style: italic; }
        blockquote { margin-left: 4em; }
        code { font-family: monospace; }
        table { border-collapse: collapse; margin: 1em auto; }
        td, th { border: 1px solid black; padding: 0.3em; }
    </style>
</head>
<body>
`)

	buf.WriteString(content)
	buf.WriteString("\n</body>\n</html>")

	return buf.String()
}

// GenerateTitlePage generates a title page HTML
func (p *HTMLProcessor) GenerateTitlePage(metadata Metadata) string {
	var buf bytes.Buffer

	buf.WriteString(`<div style="text-align: center; page-break-after: always;">
`)

	// Title
	if metadata.Title != "" {
		buf.WriteString(fmt.Sprintf(`<h1>%s</h1>\n`, htmlEscape(metadata.Title)))
	}

	// Authors
	if len(metadata.Authors) > 0 {
		for _, author := range metadata.Authors {
			if author.FullName != "" {
				buf.WriteString(fmt.Sprintf(`<h2>%s</h2>\n`, htmlEscape(author.FullName)))
			}
		}
		buf.WriteString("<br/>\n")
	}

	// Series info
	if metadata.Series != "" {
		seriesText := metadata.Series
		if metadata.SeriesIndex > 0 {
			seriesText += fmt.Sprintf(" (#%d)", metadata.SeriesIndex)
		}
		buf.WriteString(fmt.Sprintf(`<h3>%s</h3>\n`, htmlEscape(seriesText)))
		buf.WriteString("<br/>\n")
	}

	// Publisher info
	if metadata.Publisher != "" {
		buf.WriteString(fmt.Sprintf(`<p>%s</p>\n`, htmlEscape(metadata.Publisher)))
	}

	if metadata.Year != "" {
		buf.WriteString(fmt.Sprintf(`<p>%s</p>\n`, htmlEscape(metadata.Year)))
	}

	// ISBN
	if metadata.ISBN != "" {
		buf.WriteString(fmt.Sprintf(`<p>ISBN: %s</p>\n`, htmlEscape(metadata.ISBN)))
	}

	buf.WriteString("</div>\n")

	return buf.String()
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

// Cleanup removes temporary/cleanup markers from HTML
func (p *HTMLProcessor) Cleanup(html string) string {
	// Remove any comments that might be used for processing
	re := regexp.MustCompile(`<!--.*?-->`)
	html = re.ReplaceAllString(html, "")

	// Remove processing instructions
	re = regexp.MustCompile(`<\?.*?\?>`)
	html = re.ReplaceAllString(html, "")

	return html
}
