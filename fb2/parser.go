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

	"github.com/htol/fb2c/fb2/b64"
	"github.com/htol/fb2c/fb2/encoding"
)

// Parser parses FB2 files
type Parser struct {
	// Options
	NoInlineTOC   bool
	ProcessCSS    bool
	ExtractImages bool

	// Internal state
	imageData   map[string][]byte // binary ID -> decoded image data
	imageTypes  map[string]string // binary ID -> content-type
	stylesheets map[string]string

	// Detected namespace
	fbNamespace string
}

// NewParser creates a new FB2 parser
func NewParser() *Parser {
	return &Parser{
		NoInlineTOC:   false,
		ProcessCSS:    true,
		ExtractImages: true,
		imageData:     make(map[string][]byte),
		imageTypes:    make(map[string]string),
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
	data = bytes.ReplaceAll(data, []byte{0x00}, nil)

	text, _, err := encoding.ToUTF8WithStrip(data, true)
	if err != nil {
		return nil, fmt.Errorf("fb2: encoding detection failed: %w", err)
	}

	text = fixXMLErrors(text)

	var fb2 FictionBook
	err = xml.Unmarshal([]byte(text), &fb2)
	if err != nil {
		return nil, fmt.Errorf("fb2: XML parse failed: %w", err)
	}

	p.fbNamespace = fb2.XMLNS
	if p.fbNamespace == "" {
		p.fbNamespace = FB2NS
	}

	if p.ExtractImages {
		if err := p.extractEmbeddedContent(&fb2); err != nil {
			return nil, fmt.Errorf("failed to extract embedded content: %w", err)
		}
	}

	return &fb2, nil
}

// ParseFile parses an FB2 file from disk
func (p *Parser) ParseFile(path string) (*FictionBook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to read file: %w", err)
	}

	if bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04}) ||
		bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x05, 0x06}) ||
		bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x07, 0x08}) {
		return p.ParseFBZ(path)
	}

	return p.ParseBytes(data)
}

// ParseFBZ parses a zipped FB2 file
func (p *Parser) ParseFBZ(path string) (*FictionBook, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("fb2: failed to open ZIP: %w", err)
	}
	defer r.Close()

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

		data, err := b64.Decode([]byte(binary.Data))
		if err != nil {
			// A broken binary means the document cannot be faithfully converted:
			// fail loudly instead of silently dropping the image (docs/TESTING.md Q15).
			return fmt.Errorf("fb2: failed to decode binary %q: %w", binary.ID, err)
		}

		p.imageData[binary.ID] = data
		p.imageTypes[binary.ID] = binary.GetContentType()
	}

	return nil
}

// GetImageData returns the map of binary IDs to decoded image data
func (p *Parser) GetImageData() map[string][]byte {
	return p.imageData
}

// GetImageType returns the content-type for a binary ID
func (p *Parser) GetImageType(binaryID string) string {
	if ct, ok := p.imageTypes[binaryID]; ok {
		return ct
	}
	return MIMEJPEG // Default fallback
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
