// Package fb2 provides metadata extraction from FB2 files.
package fb2

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Metadata represents extracted book metadata
type Metadata struct {
	Title       string
	Authors     []string
	AuthorSort  string
	AuthorsFull string // Formatted "Last, First Middle"
	Publisher   string
	ISBN        string
	Year        string
	PubDate     time.Time
	Language    string
	Languages   []string
	Series      string
	SeriesIndex int
	Genres      []string
	Keywords    []string
	Annotation  string
	Comments    string // Alias for annotation

	// Cover image
	Cover     []byte
	CoverExt  string // jpg, png, etc.
	CoverID   string // Binary ID

	// Additional metadata
	FilePath  string
}

// ExtractMetadata extracts metadata from an FB2 document
func (p *Parser) ExtractMetadata(fb2 *FictionBook) (*Metadata, error) {
	m := &Metadata{
		Languages: []string{},
		Genres:    []string{},
		Keywords:  []string{},
	}

	// Extract from TitleInfo
	ti := fb2.Description.TitleInfo
	if ti.BookTitle != "" {
		m.Title = strings.TrimSpace(ti.BookTitle)
	}

	// Authors
	for _, author := range ti.Author {
		name := formatAuthorName(author)
		if name != "" {
			m.Authors = append(m.Authors, name)
		}
		// Build author sort: "Last, First Middle"
		if author.LastName != "" {
			sortName := author.LastName
			if author.FirstName != "" {
				sortName += ", " + author.FirstName
				if author.MiddleName != "" {
					sortName += " " + author.MiddleName
				}
			}
			if m.AuthorSort == "" {
				m.AuthorSort = sortName
			} else {
				m.AuthorSort += " & " + sortName
			}
		}
	}

	// Full authors string (for display)
	m.AuthorsFull = strings.Join(m.Authors, " & ")

	// Language
	if ti.Language != "" {
		m.Language = ti.Language
		m.Languages = append(m.Languages, ti.Language)
	}

	// Genres
	m.Genres = append(m.Genres, ti.Genre...)

	// Annotation
	if ti.Annotation != nil {
		m.Annotation = extractTextContent(ti.Annotation)
		m.Comments = m.Annotation
	}

	// Keywords
	if ti.Keywords != nil {
		m.Keywords = parseKeywords(ti.Keywords.Text)
	}

	// Sequence (series)
	if len(ti.Sequence) > 0 {
		seq := ti.Sequence[0] // Use first sequence
		m.Series = seq.Name
		m.SeriesIndex = seq.Number
	}

	// Extract from PublishInfo
	pi := fb2.Description.PublishInfo
	if pi.Publisher != "" {
		m.Publisher = strings.TrimSpace(pi.Publisher)
	}
	if pi.ISBN != "" {
		m.ISBN = strings.TrimSpace(pi.ISBN)
	}
	if pi.Year != "" {
		m.Year = pi.Year
		// Try to parse as date (June 2 of year)
		if year, err := parseYear(pi.Year); err == nil {
			m.PubDate = year
		}
	}
	if len(pi.Sequence) > 0 && m.Series == "" {
		// Use publish-info sequence if title-info didn't have one
		seq := pi.Sequence[0]
		m.Series = seq.Name
		m.SeriesIndex = seq.Number
	}

	// Cover image
	if ti.Coverpage.PrimaryImage.Href != "" || ti.Coverpage.PrimaryImage.LHref != "" ||
	   ti.Coverpage.PrimaryImage.LHref2 != "" || len(ti.Coverpage.PrimaryImage.AnyAttr) > 0 {
		href := ti.Coverpage.PrimaryImage.Href

		// Try local href if regular href is empty
		if href == "" && ti.Coverpage.PrimaryImage.LHref != "" {
			href = ti.Coverpage.PrimaryImage.LHref
		}

		// Try namespaced href (l:href with xmlns:l="...")
		if href == "" && ti.Coverpage.PrimaryImage.LHref2 != "" {
			href = ti.Coverpage.PrimaryImage.LHref2
		}

		// Fallback: check AnyAttr for l:href or xlink:href
		if href == "" && len(ti.Coverpage.PrimaryImage.AnyAttr) > 0 {
			for _, attr := range ti.Coverpage.PrimaryImage.AnyAttr {
				if (attr.Name.Local == "href" && (attr.Name.Space == "l" || attr.Name.Space == "xlink")) ||
				   attr.Name.Local == "l:href" || attr.Name.Local == "xlink:href" {
					href = attr.Value
					break
				}
			}
		}

		if href != "" {
			// Remove # prefix if present
			href = strings.TrimPrefix(href, "#")
			m.CoverID = href

			// Try to extract cover from binaries
			m.Cover, m.CoverExt = p.extractCoverImage(href)
		}
	}

	return m, nil
}

// formatAuthorName formats an author's name
func formatAuthorName(author Author) string {
	parts := []string{}
	if author.FirstName != "" {
		parts = append(parts, author.FirstName)
	}
	if author.MiddleName != "" {
		parts = append(parts, author.MiddleName)
	}
	if author.LastName != "" {
		parts = append(parts, author.LastName)
	}

	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}

	// Fallback to nickname
	return author.Nickname
}

// extractTextContent extracts text from a TextContainer
func extractTextContent(tc *TextContainer) string {
	if tc == nil {
		return ""
	}

	var buf strings.Builder

	if tc.Text != "" {
		buf.WriteString(tc.Text)
		buf.WriteString(" ")
	}

	for _, p := range tc.P {
		if p.Text != "" {
			buf.WriteString(p.Text)
			buf.WriteString(" ")
		}
	}

	return strings.TrimSpace(buf.String())
}

// parseKeywords parses keywords from a string
func parseKeywords(text string) []string {
	if text == "" {
		return nil
	}

	// Keywords can be separated by commas, semicolons, or various delimiters
	// For now, split on common delimiters
	text = strings.ReplaceAll(text, ", ", ",")
	text = strings.ReplaceAll(text, ";", ",")
	text = strings.ReplaceAll(text, "\t", ",")
	text = strings.ReplaceAll(text, "\n", ",")

	parts := strings.Split(text, ",")
	keywords := make([]string, 0, len(parts))

	for _, kw := range parts {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			keywords = append(keywords, kw)
		}
	}

	if len(keywords) == 0 {
		return nil
	}

	return keywords
}

// parseYear parses a year string to a time.Time
func parseYear(yearStr string) (time.Time, error) {
	// Year is just a number like "2010"
	// FB2 spec says: "The year of the publication of this book."
	// In calibre, it's converted to June 2 of that year
	year := 0
	_, err := fmt.Sscanf(yearStr, "%d", &year)
	if err != nil {
		return time.Time{}, err
	}

	// Return June 2 of the given year (following calibre's convention)
	return time.Date(year, time.June, 2, 0, 0, 0, 0, time.UTC), nil
}

// extractCoverImage extracts cover image data from binaries
func (p *Parser) extractCoverImage(binaryID string) ([]byte, string) {
	// Look for the binary in the image map
	if filename, ok := p.imageMap[binaryID]; ok {
		// Read the file (this is a simple implementation)
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, ""
		}

		// Detect image format
		ext := filepath.Ext(filename)
		if ext == "" {
			// Try to detect from data
			_, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err == nil {
				ext = "." + format
			}
		}

		return data, ext
	}

	return nil, ""
}

// GetMetadataFromFile is a convenience function to extract metadata from an FB2 file
func GetMetadataFromFile(path string) (*Metadata, error) {
	parser := NewParser()
	fb2, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}

	return parser.ExtractMetadata(fb2)
}

// GetMetadataFromBytes is a convenience function to extract metadata from FB2 data
func GetMetadataFromBytes(data []byte) (*Metadata, error) {
	parser := NewParser()
	fb2, err := parser.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	return parser.ExtractMetadata(fb2)
}
