// Package fb2 provides FB2 (FictionBook 2.0) file parsing and processing.
package fb2

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/htol/fb2c/b64"
	"github.com/htol/fb2c/fb2encoding"
)

const (
	// FB2Namespaces
	FB2NS  = "http://www.gribuser.ru/xml/fictionbook/2.0"
	FB21NS = "http://www.gribuser.ru/xml/fictionbook/2.1"
	XLINKNS = "http://www.w3.org/1999/xlink"
)

// FictionBook represents the root FB2 document structure
type FictionBook struct {
	XMLName    xml.Name `xml:"FictionBook"`
	XMLNS      string   `xml:"xmlns,attr"`
	Description Description `xml:"description"`
	Body       Body       `xml:"body"`
	Binaries   []Binary   `xml:"binary"`
}

// Description contains book metadata
type Description struct {
	TitleInfo   TitleInfo   `xml:"title-info"`
	SrcTitleInfo *TitleInfo `xml:"src-title-info"`
	PublishInfo PublishInfo `xml:"publish-info"`
	DocumentInfo DocumentInfo `xml:"document-info"`
}

// TitleInfo contains main book metadata
type TitleInfo struct {
	Genre      []string `xml:"genre"`
	Author     []Author `xml:"author"`
	BookTitle  string   `xml:"book-title"`
	Annotation *TextContainer `xml:"annotation"`
	Keywords   *TextContainer `xml:"keywords"`
	Date       Date     `xml:"date"`
	Coverpage  Coverpage `xml:"coverpage"`
	Language   string   `xml:"lang"`
	SrcLang    string   `xml:"src-lang"`
	Sequence   []Sequence `xml:"sequence"`
}

// Author represents a book author
type Author struct {
	FirstName  string `xml:"first-name"`
	MiddleName string `xml:"middle-name"`
	LastName   string `xml:"last-name"`
	Nickname   string `xml:"nickname"`
	HomePage   string `xml:"home-page"`
	Email      string `xml:"email"`
}

// Date represents a date value
type Date struct {
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

// Sequence represents series information
type Sequence struct {
	Name   string `xml:"name,attr"`
	Number int    `xml:"number,attr"`
	// Lang   string `xml:"lang,attr"` // Optional
}

// Coverpage represents cover image reference
type Coverpage struct {
	PrimaryImage ImageRef `xml:"image"`
}

// ImageRef is a reference to an image
type ImageRef struct {
	Href    string `xml:"href,attr"`
	LHref   string `xml:"l:href,attr"`    // Local href (FB2 specific)
	LHref2  string `xml:"http://www.w3.org/1999/xlink href,attr"` // Namespaced href
	// AnyAttr will capture all attributes for fallback processing
	AnyAttr []xml.Attr `xml:",any,attr"`
}

// TextContainer contains text with possible markup
type TextContainer struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
	P       []P `xml:"p"`
}

// P represents a paragraph
type P struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
}

// PublishInfo contains publishing metadata
type PublishInfo struct {
	BookName   string `xml:"book-name"`
	Publisher  string `xml:"publisher"`
	City       string `xml:"city"`
	Year       string `xml:"year"`
	ISBN       string `xml:"isbn"`
	Sequence   []Sequence `xml:"sequence"`
}

// DocumentInfo contains document metadata
type DocumentInfo struct {
	Author     []Author `xml:"author"`
	ProgramUsed string   `xml:"program-used"`
	Date       Date     `xml:"date"`
	ID          string   `xml:"id"`
	Version    string   `xml:"version"`
	History    []History `xml:"history"`
}

// History contains version history
type History struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
	P       []P `xml:"p"`
}

// Body contains the book content
type Body struct {
	XMLName xml.Name `xml:"body"`
	Name    string `xml:"name,attr"`
	Language string `xml:"lang,attr"`
	Title   *Title `xml:"title"`
	Sections []Section `xml:"section"`
	Epigraphs []Epigraph `xml:"epigraph"`
	// Direct content
	Content []ContentNode `xml:",any"`
}

// Title represents a section title
type Title struct {
	XMLName xml.Name `xml:"title"`
	P       []P `xml:"p"`
}

// Section represents a book section
type Section struct {
	XMLName xml.Name `xml:"section"`
	ID      string `xml:"id,attr"`
	Name    string `xml:"name,attr"`
	Title   *Title `xml:"title"`
	Epigraphs []Epigraph `xml:"epigraph"`
	Sections []Section `xml:"section"`
	// Various content elements
	Paragraphs []P `xml:"p"`
	Subtitle  *P `xml:"subtitle"`
	Cite      []Cite `xml:"cite"`
	Stanza    []Stanza `xml:"stanza"`
	Code      []Code `xml:"code"`
	Table     []Table `xml:"table"`
	Image     []Image `xml:"image"`
	// Content nodes
	Content []ContentNode `xml:",any"`
}

// Epigraph represents an epigraph
type Epigraph struct {
	XMLName xml.Name `xml:"epigraph"`
	TextAlign string `xml:"align,attr"`
	Authors []Author `xml:"author"`
	Content []ContentNode `xml:",any"`
}

// Cite represents a quotation
type Cite struct {
	XMLName xml.Name `xml:"cite"`
	Authors []Author `xml:"author"`
	Content []ContentNode `xml:",any"`
}

// Stanza represents a poem stanza
type Stanza struct {
	XMLName xml.Name `xml:"stanza"`
	Title   *Title `xml:"title"`
	Author []Author `xml:"author"`
	Date   Date `xml:"date"`
	V       []V `xml:"v"`
}

// V represents a verse line
type V struct {
	XMLName xml.Name `xml:"v"`
	Text    string `xml:",chardata"`
}

// Code represents code text
type Code struct {
	XMLName xml.Name `xml:"code"`
	Text    string `xml:",chardata"`
}

// Table represents a table
type Table struct {
	XMLName xml.Name `xml:"table"`
	Rows    []TR `xml:"tr"`
}

// TR is a table row
type TR struct {
	XMLName xml.Name `xml:"tr"`
	Align   string `xml:"align,attr"`
	Cells   []TableCell `xml:",any"`
}

// TableCell is a table cell
type TableCell struct {
	XMLName  xml.Name
	ColSpan  int    `xml:"colspan,attr"`
	RowSpan  int    `xml:"rowspan,attr"`
	Style    string `xml:"style,attr"`
	Class    string `xml:"class,attr"`
	Content  string `xml:",chardata"`
}

// Image represents an inline image
type Image struct {
	XMLName xml.Name `xml:"image"`
	Href    string `xml:"href,attr"`
	Alt     string `xml:"alt,attr"`
	Title   string `xml:"title,attr"`
	XLinkHref string `xml:"http://www.w3.org/1999/xlink href,attr"`
}

// Binary contains embedded binary data
type Binary struct {
	XMLName    xml.Name `xml:"binary"`
	ID         string `xml:"id,attr"`
	ContentType string `xml:"content-type,attr"`
	Data       string `xml:",chardata"`
}

// ContentNode represents any content node
type ContentNode struct {
	XMLName xml.Name
	Content string `xml:",chardata"`
}

// Parser parses FB2 files
type Parser struct {
	// Options
	NoInlineTOC    bool
	ProcessCSS     bool
	ExtractImages  bool

	// Internal state
	imageMap    map[string]string // binary ID -> filename
	stylesheets map[string]string
	coverPath   string

	// Detected namespace
	fbNamespace string
}

// NewParser creates a new FB2 parser
func NewParser() *Parser {
	return &Parser{
		NoInlineTOC:   false,
		ProcessCSS:    true,
		ExtractImages: true,
		imageMap:      make(map[string]string),
		stylesheets:   make(map[string]string),
	}
}

// Parse parses an FB2 file from a reader
func (p *Parser) Parse(r io.Reader) (*FictionBook, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to read: %w", err)
	}

	return p.ParseBytes(data)
}

// ParseBytes parses FB2 data from bytes
func (p *Parser) ParseBytes(data []byte) (*FictionBook, error) {
	// Remove null bytes
	data = bytes.ReplaceAll(data, []byte{0x00}, nil)

	// Detect encoding and convert to UTF-8
	text, _, err := fb2encoding.ToUTF8WithStrip(data, true)
	if err != nil {
		return nil, fmt.Errorf("fb2: encoding detection failed: %w", err)
	}

	// Fix common XML syntax errors
	text = fixXMLErrors(text)

	// Parse XML
	var fb2 FictionBook
	err = xml.Unmarshal([]byte(text), &fb2)
	if err != nil {
		return nil, fmt.Errorf("fb2: XML parse failed: %w", err)
	}

	// Ensure namespace
	p.fbNamespace = fb2.XMLNS
	if p.fbNamespace == "" {
		p.fbNamespace = FB2NS
	}

	// Extract embedded content (images, etc.)
	if p.ExtractImages {
		p.extractEmbeddedContent(&fb2)
	}

	return &fb2, nil
}

// ParseFile parses an FB2 file from disk
func (p *Parser) ParseFile(path string) (*FictionBook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to read file: %w", err)
	}

	// Check if it's a ZIP file (FBZ)
	if bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04}) ||
	   bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x05, 0x06}) ||
	   bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x07, 0x08}) {
		return p.ParseFBZ(path)
	}

	return p.ParseBytes(data)
}

// ParseFBZ parses a zipped FB2 file
func (p *Parser) ParseFBZ(path string) (*FictionBook, error) {
	// Open ZIP archive
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to open ZIP: %w", err)
	}
	defer r.Close()

	// Find .fb2 file in archive
	var fb2File *zip.File
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".fb2") {
			fb2File = f
			break
		}
	}

	if fb2File == nil {
		return nil, fmt.Errorf("fb2: no .fb2 file found in archive")
	}

	// Read FB2 content
	rc, err := fb2File.Open()
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to open file in ZIP: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to read file in ZIP: %w", err)
	}

	return p.ParseBytes(data)
}

// extractEmbeddedContent extracts binary data (images) from FB2
func (p *Parser) extractEmbeddedContent(fb2 *FictionBook) error {
	for i := range fb2.Binaries {
		binary := &fb2.Binaries[i]

		if binary.ID == "" {
			continue
		}

		// Decode base64 data
		data, err := b64.Decode([]byte(binary.Data))
		if err != nil {
			// Use robust decoder
			continue
		}

		// Determine filename
		filename := sanitizeFilename(binary.ID)

		// Add extension based on content type (only if not already present)
		ct := strings.ToLower(binary.ContentType)
		if strings.HasPrefix(ct, "image/") {
			ext := extensionFromMIME(ct)
			if ext != "" && !strings.HasSuffix(filename, ext) {
				filename += ext
			}
			p.imageMap[binary.ID] = filename
		}

		// Write to file (in current directory for now)
		// In production, you'd want to control the output directory
		p.writeExtractedFile(filename, data)
	}

	return nil
}

// writeExtractedFile writes extracted content to a file
func (p *Parser) writeExtractedFile(filename string, data []byte) error {
	// Create output directory if needed
	// For now, write to current directory
	return os.WriteFile(filename, data, 0644)
}

// GetImageMap returns the map of binary IDs to filenames
func (p *Parser) GetImageMap() map[string]string {
	return p.imageMap
}

// GetNamespace returns the detected FB2 namespace
func (p *Parser) GetNamespace() string {
	return p.fbNamespace
}

// fixXMLErrors fixes common XML syntax errors in FB2 files
func fixXMLErrors(text string) string {
	// Fix unescaped ampersands (common issue)
	// Replace '& ' (ampersand followed by space) with '&amp; '
	text = strings.ReplaceAll(text, "& ", "&amp; ")

	return text
}

// sanitizeFilename sanitizes a filename by removing dangerous characters
func sanitizeFilename(name string) string {
	// Remove or replace dangerous characters
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	name = reg.ReplaceAllString(name, "_")

	// Collapse multiple consecutive underscores to single underscore
	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	// Remove leading/trailing dots and spaces
	name = strings.Trim(name, ". ")

	// Limit length
	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

// extensionFromMIME returns a file extension for a MIME type
func extensionFromMIME(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
