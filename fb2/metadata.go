// Package fb2 provides metadata extraction from FB2 files.
package fb2

import (
	"bytes"
	"fmt"
	"image"

	// Register image decoders
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
)

const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEGIF  = "image/gif"
	MIMESVG  = "image/svg+xml"
	MIMEWEBP = "image/webp"

	ExtJPG  = ".jpg"
	ExtPNG  = ".png"
	ExtGIF  = ".gif"
	ExtSVG  = ".svg"
	ExtWEBP = ".webp"
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
	Cover    []byte
	CoverExt string // jpg, png, etc.
	CoverID  string // Binary ID

	// Additional metadata
	FilePath string
}

// ImageProvider provides access to book images
type ImageProvider interface {
	GetImageData() map[string][]byte
	GetImageType(binaryID string) string
}

// ExtractMetadata extracts metadata from an FB2 document
func ExtractMetadata(fb2 *FictionBook, ip ImageProvider) (*Metadata, error) {
	m := &Metadata{
		Languages: []string{},
		Genres:    []string{},
		Keywords:  []string{},
	}

	extractTitleInfo(m, fb2.Description.TitleInfo)
	extractPublishInfo(m, fb2.Description.PublishInfo)
	extractCover(m, fb2.Description.TitleInfo.Coverpage, ip)

	return m, nil
}

func extractTitleInfo(m *Metadata, ti TitleInfo) {
	if ti.BookTitle != "" {
		m.Title = strings.TrimSpace(ti.BookTitle)
	}

	for _, author := range ti.Author {
		name := formatAuthorName(author)
		if name != "" {
			m.Authors = append(m.Authors, name)
		}
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
	m.AuthorsFull = strings.Join(m.Authors, " & ")

	if ti.Language != "" {
		m.Language = ti.Language
		m.Languages = append(m.Languages, ti.Language)
	}
	m.Genres = append(m.Genres, ti.Genre...)

	if ti.Annotation != nil {
		m.Annotation = extractTextContent(ti.Annotation)
		m.Comments = m.Annotation
	}

	if ti.Keywords != nil {
		m.Keywords = parseKeywords(ti.Keywords.Text)
	}

	if len(ti.Sequence) > 0 {
		seq := ti.Sequence[0]
		m.Series = seq.Name
		m.SeriesIndex = seq.Number
	}
}

func extractPublishInfo(m *Metadata, pi PublishInfo) {
	if pi.Publisher != "" {
		m.Publisher = strings.TrimSpace(pi.Publisher)
	}
	if pi.ISBN != "" {
		m.ISBN = strings.TrimSpace(pi.ISBN)
	}
	if pi.Year != "" {
		m.Year = pi.Year
		if year, err := parseYear(pi.Year); err == nil {
			m.PubDate = year
		}
	}
	if len(pi.Sequence) > 0 && m.Series == "" {
		seq := pi.Sequence[0]
		m.Series = seq.Name
		m.SeriesIndex = seq.Number
	}
}

func extractCover(m *Metadata, cover Coverpage, ip ImageProvider) {
	href := resolveCoverHref(cover)
	if href != "" {
		href = strings.TrimPrefix(href, "#")
		m.CoverID = href
		m.Cover, m.CoverExt = extractCoverImage(href, ip)
	}
}

func resolveCoverHref(cover Coverpage) string {
	if cover.PrimaryImage.Href != "" {
		return cover.PrimaryImage.Href
	}
	if cover.PrimaryImage.LHref != "" {
		return cover.PrimaryImage.LHref
	}
	if cover.PrimaryImage.LHref2 != "" {
		return cover.PrimaryImage.LHref2
	}

	for _, attr := range cover.PrimaryImage.AnyAttr {
		if (attr.Name.Local == "href" && (attr.Name.Space == "l" || attr.Name.Space == "xlink")) ||
			attr.Name.Local == "l:href" || attr.Name.Local == "xlink:href" {
			return attr.Value
		}
	}

	return ""
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
func extractCoverImage(binaryID string, ip ImageProvider) ([]byte, string) {
	// Look for the binary data in imageData
	if data, ok := ip.GetImageData()[binaryID]; ok {
		// Get content-type from imageTypes
		contentType := ip.GetImageType(binaryID)

		// Convert content-type to extension
		ext := contentTypeToExtension(contentType)

		// Fallback: detect from data if extension unknown
		if ext == "" {
			_, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err == nil {
				ext = "." + format
			}
		}

		return data, ext
	}

	return nil, ""
}

// contentTypeToExtension converts a content-type to a file extension
func contentTypeToExtension(contentType string) string {
	switch contentType {
	case MIMEJPEG:
		return ExtJPG
	case MIMEPNG:
		return ExtPNG
	case MIMEGIF:
		return ExtGIF
	case MIMESVG:
		return ExtSVG
	case MIMEWEBP:
		return ExtWEBP
	default:
		return ""
	}
}

// GetMetadataFromFile is a convenience function to extract metadata from an FB2 file
func GetMetadataFromFile(path string) (*Metadata, error) {
	parser := NewParser()
	fb2, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}

	return ExtractMetadata(fb2, parser)
}

// GetMetadataFromBytes is a convenience function to extract metadata from FB2 data
func GetMetadataFromBytes(data []byte) (*Metadata, error) {
	parser := NewParser()
	fb2, err := parser.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	return ExtractMetadata(fb2, parser)
}
