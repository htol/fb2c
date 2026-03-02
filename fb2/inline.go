package fb2

import (
	"html"
	"regexp"
	"strings"
)

var (
	linkRe      = regexp.MustCompile(`<a\s+([^>]*)>([^<]*)</a>`)
	lHrefRe     = regexp.MustCompile(`l:href="([^"]*)"`)
	xlinkHrefRe = regexp.MustCompile(`xlink:href="([^"]*)"`)
	hrefRe      = regexp.MustCompile(`href="([^"]*)"`)
	typeRe      = regexp.MustCompile(`type="([^"]*)"`)
)

// ParseInlineContent converts FB2 inline XML elements to HTML.
// Handles: <a>, <emphasis>, <strong>, <strikethrough>, <code>, <sup>, <sub>, <empty-line>
func ParseInlineContent(rawXML string) string {
	if rawXML == "" {
		return ""
	}

	result := rawXML

	// Convert empty-line to br
	result = strings.ReplaceAll(result, "<empty-line/>", "<br/>")
	result = strings.ReplaceAll(result, "<empty-line />", "<br/>")
	result = strings.ReplaceAll(result, "<empty-line>", "<br/>")
	result = strings.ReplaceAll(result, "</empty-line>", "")

	// Convert FB2 links to HTML links
	// <a type="note" l:href="#note_1">[1]</a> -> <a href="#note_1" class="noteref" id="noteref_1">[1]</a>
	result = convertLinks(result)

	// Convert emphasis to em
	result = convertTag(result, "emphasis", "em")

	// Convert strong
	result = convertTag(result, "strong", "strong")

	// Convert strikethrough to del
	result = convertTag(result, "strikethrough", "del")

	// Convert code
	result = convertTag(result, "code", "code")

	// Convert sup/sub (already HTML-compatible names)
	// These should work as-is (browsers support them)

	return result
}

// convertLinks converts FB2 <a> tags to HTML anchor tags
func convertLinks(s string) string {
	// Pattern for FB2 links: <a type="note" l:href="#note_1">[1]</a>

	return linkRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract attributes and content
		submatch := linkRe.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}

		attrs := submatch[1]
		content := submatch[2]

		// Extract href (l:href, xlink:href, or href)
		href := extractAttr(attrs, lHrefRe)
		if href == "" {
			href = extractAttr(attrs, xlinkHrefRe)
		}
		if href == "" {
			href = extractAttr(attrs, hrefRe)
		}

		// Extract type
		linkType := extractAttr(attrs, typeRe)

		// Build HTML link
		var htmlAttrs []string

		if href != "" {
			htmlAttrs = append(htmlAttrs, `href="`+html.EscapeString(href)+`"`)
		}

		// For note references, add special class and ID for back-navigation
		anchorTag := ""
		if linkType == "note" {
			htmlAttrs = append(htmlAttrs, `class="noteref"`)
			// Extract note ID from href (e.g., "#note_1" -> "note_1")
			noteID := strings.TrimPrefix(href, "#")
			if noteID != "" {
				// Create separate empty anchor for target - more reliable in MOBI
				anchorTag = `<a id="noteref_` + noteID + `" name="noteref_` + noteID + `"></a>`
			}
		}

		if len(htmlAttrs) == 0 {
			return content // No valid attributes, just return text
		}

		return anchorTag + "<a " + strings.Join(htmlAttrs, " ") + ">" + content + "</a>"
	})
}

// extractAttr extracts an attribute value using regex pattern
func extractAttr(attrs string, re *regexp.Regexp) string {
	match := re.FindStringSubmatch(attrs)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

// convertTag converts FB2 tags to HTML tags
func convertTag(s, fb2Tag, htmlTag string) string {
	// Opening tag
	s = strings.ReplaceAll(s, "<"+fb2Tag+">", "<"+htmlTag+">")
	s = strings.ReplaceAll(s, "<"+fb2Tag+" ", "<"+htmlTag+" ")

	// Closing tag
	s = strings.ReplaceAll(s, "</"+fb2Tag+">", "</"+htmlTag+">")

	return s
}
