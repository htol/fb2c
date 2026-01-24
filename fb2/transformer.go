// Package fb2 provides FB2 to HTML transformation.
package fb2

import (
	"bytes"
	"fmt"
	"strings"
)

// Transformer converts FB2 to HTML
type Transformer struct {
	parser *Parser

	// Options
	ProcessCSS  bool
	UseDataURLs bool   // If true, images are embedded as data URLs. If false, href is used.
	Title       string // Override title
	MOBIMode    bool   // If true, generate minimalist HTML for MOBI

	// CSS processing
	cssContent string

	// Internal
	GlobalIDCounter int

	// Output
	HTML     string
	CSS      string
	Metadata *Metadata
}

// NewTransformer creates a new FB2 transformer
func NewTransformer() *Transformer {
	return &Transformer{
		parser:     NewParser(),
		ProcessCSS: true,
		MOBIMode:   true,
	}
}

// SetParser sets the parser instance to use
func (t *Transformer) SetParser(p *Parser) {
	t.parser = p
}

// TransformResult contains the result of FB2 transformation
type TransformResult struct {
	HTML     string
	CSS      string
	Metadata *Metadata
}

// Transform converts an already parsed FB2 book to HTML
func (t *Transformer) Transform(fb2 *FictionBook) (*TransformResult, error) {
	metadata, err := ExtractMetadata(fb2, t.parser)
	if err != nil {
		return nil, err
	}
	t.Metadata = metadata

	t.processStylesheets(fb2)

	html := t.transformToHTML(fb2)

	return &TransformResult{
		HTML:     html,
		CSS:      t.cssContent,
		Metadata: metadata,
	}, nil
}

// processStylesheets extracts and processes CSS stylesheets
func (t *Transformer) processStylesheets(_ *FictionBook) {
	var css strings.Builder

	// In a full implementation, we'd extract and process stylesheets
	// For now, we'll just note where CSS would go

	t.cssContent = css.String()
}

// transformToHTML transforms FB2 to HTML
func (t *Transformer) transformToHTML(fb2 *FictionBook) string {
	var buf bytes.Buffer
	t.GlobalIDCounter = 0

	if t.MOBIMode {
		buf.WriteString("<html>\n<head>\n")
		if fb2.Description.TitleInfo.Coverpage.PrimaryImage.Href != "" {
			buf.WriteString("<guide>\n")
			buf.WriteString("  <reference type=\"cover\" title=\"Cover\" href=\"#cover\" />\n")
			buf.WriteString("  <reference type=\"toc\" title=\"Table of Contents\" href=\"#toc\" />\n")
			buf.WriteString("</guide>\n")
		}
		buf.WriteString("</head>\n")
	} else {
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
		buf.WriteString("</head>\n")
	}

	buf.WriteString("<body>\n")

	if fb2.Description.TitleInfo.Coverpage.PrimaryImage.Href != "" {
		buf.WriteString(t.renderCoverPage(fb2.Description.TitleInfo.Coverpage))
		if t.MOBIMode {
			buf.WriteString("<p>&nbsp;</p>\n")
		} else {
			buf.WriteString("<hr/>\n")
		}
	}

	if fb2.Description.TitleInfo.Annotation != nil {
		annotation := extractTextContent(fb2.Description.TitleInfo.Annotation)
		if annotation != "" {
			buf.WriteString("<div>")
			buf.WriteString(htmlEscape(annotation))
			buf.WriteString("</div>\n<hr/>\n")
		}
	}

	if len(fb2.Bodies) > 0 {
		buf.WriteString("<a id=\"toc\"></a>\n")
		buf.WriteString(t.generateTOC(fb2.Bodies[0].Sections, 0))
		buf.WriteString("<hr/>\n")
	}

	for _, body := range fb2.Bodies {
		buf.WriteString(t.renderBody(body))
	}

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
func (t *Transformer) countSectionDepth(_ Section) int {
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
