package fb2

import (
	"fmt"
	"strings"
)

// ExtractTOC extracts table of contents from FB2 document structure
func (p *Parser) ExtractTOC(fb2 *FictionBook) (*TOCData, error) {
	if len(fb2.Bodies) == 0 || len(fb2.Bodies[0].Sections) == 0 {
		return nil, nil // No TOC available
	}

	toc := &TOCData{
		Entries: []*TOCEntry{},
	}

	// Extract TOC from main body sections (usually the first one)
	for _, section := range fb2.Bodies[0].Sections {
		p.extractSectionTOC(&section, toc.Root, 1, toc)
	}

	return toc, nil
}

// extractSectionTOC recursively extracts TOC from sections
func (p *Parser) extractSectionTOC(section *Section, parent *TOCEntry, level int, toc *TOCData) {
	entry := &TOCEntry{
		Level:   level,
		Section: section,
	}

	// Extract title from section
	if section.Title != nil && len(section.Title.P) > 0 {
		var titleParts []string
		for _, p := range section.Title.P {
			if p.Text != "" {
				titleParts = append(titleParts, strings.TrimSpace(p.Text))
			}
		}
		entry.Label = strings.Join(titleParts, " ")
	}

	// Generate ID if not present
	if section.ID != "" {
		entry.ID = section.ID
		entry.Href = "#" + section.ID
	} else {
		// Generate a unique ID based on title or position
		if entry.Label != "" {
			entry.ID = sanitizeID(entry.Label)
		} else {
			entry.ID = fmt.Sprintf("section_%d", len(toc.Entries)+1)
		}
		entry.Href = "#" + entry.ID
	}

	// Add to entries list
	toc.Entries = append(toc.Entries, entry)

	// Recursively process nested sections
	for _, subSection := range section.Sections {
		p.extractSectionTOC(&subSection, entry, level+1, toc)
	}
}

// TOCData represents table of contents data
type TOCData struct {
	Root    *TOCEntry
	Entries []*TOCEntry
}

// TOCEntry represents a TOC entry
type TOCEntry struct {
	ID      string
	Label   string
	Href    string
	Level   int
	Section *Section
	Parent  *TOCEntry
}

// sanitizeID converts a title to a valid HTML ID
func sanitizeID(title string) string {
	// Convert to lowercase
	id := strings.ToLower(title)

	// Replace spaces with hyphens
	id = strings.ReplaceAll(id, " ", "-")

	// Remove invalid characters
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	// Ensure it's not empty
	if result.Len() == 0 {
		return "section"
	}

	return result.String()
}
