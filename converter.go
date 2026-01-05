// Package fb2c provides FB2 to MOBI/EPUB conversion.
package fb2c

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/htol/fb2c/b64"
	"github.com/htol/fb2c/epub"
	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/mobi"
	"github.com/htol/fb2c/mobi/kf8"
	"github.com/htol/fb2c/opf"
)

// ConvertOptions contains options for FB2 to MOBI/EPUB conversion
type ConvertOptions struct {
	// Format options
	MobiType    string // "old" (MOBI 6), "new" (KF8), "both" (joint)
	Compression bool   // Enable PalmDOC compression

	// Content options
	NoInlineTOC   bool // Don't generate inline TOC
	ExtractImages bool // Extract embedded images

	// Metadata overrides
	Title      string
	Authors    []string
	CoverImage string

	// KF8-specific options
	EnableChunking  bool
	TargetChunkSize int
}

// DefaultConvertOptions returns default conversion options
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		MobiType:        "old", // Create KF8 joint files by default (like Calibre)
		Compression:     true,
		NoInlineTOC:     false,
		ExtractImages:   true,
		EnableChunking:  true,
		TargetChunkSize: 4096,
	}
}

// Converter handles FB2 to MOBI conversion
type Converter struct {
	options ConvertOptions
	parser  *fb2.Parser
}

// NewConverter creates a new converter
func NewConverter() *Converter {
	return &Converter{
		options: DefaultConvertOptions(),
		parser:  fb2.NewParser(),
	}
}

// SetOptions sets conversion options
func (c *Converter) SetOptions(options ConvertOptions) {
	c.options = options
}

// Convert converts an FB2 to supported formats
func (c *Converter) Convert(inputPath, outputPath string) error {
	fb2Data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read FB2 file: %w", err)
	}

	// Convert FB2 to UTF-8 if needed
	fb2Data, err = convertToUTF8(fb2Data)
	if err != nil {
		return fmt.Errorf("failed to convert FB2 to UTF-8: %w", err)
	}

	fb2Doc, err := c.parser.ParseBytes(fb2Data)
	if err != nil {
		return fmt.Errorf("failed to parse FB2: %w", err)
	}

	metadata, err := c.parser.ExtractMetadata(fb2Doc)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Apply metadata overrides
	c.applyMetadataOverrides(metadata)

	// Transform to HTML
	transformer := fb2.NewTransformer()
	transformer.NoInlineTOC = c.options.NoInlineTOC

	html, _, _, err := transformer.ConvertBytes(fb2Data)
	if err != nil {
		return fmt.Errorf("failed to transform FB2: %w", err)
	}

	// Extract TOC from FB2 document
	tocData, err := c.parser.ExtractTOC(fb2Doc)
	if err != nil {
		return fmt.Errorf("failed to extract TOC: %w", err)
	}

	// Create OPF book
	book := c.createOPFBook(metadata, html, tocData, fb2Doc)

	// Detect output format from file extension
	ext := strings.ToLower(filepath.Ext(outputPath))

	// Write output based on format
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// EPUB format
	if ext == ".epub" {
		return c.writeEPUB(book, outputFile)
	}

	// MOBI format (default)
	switch c.options.MobiType {
	case "old", "6":
		return c.writeMOBI6(book, outputFile)
	case "new", "8":
		return c.writeKF8(book, outputFile)
	case "both":
		return c.writeJoint(book, outputFile)
	default:
		return fmt.Errorf("unknown MOBI type: %s", c.options.MobiType)
	}
}

// ConvertStream converts FB2 from reader to MOBI writer
func (c *Converter) ConvertStream(input io.Reader, output io.Writer) error {
	// Read FB2
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Parse FB2
	fb2Doc, err := c.parser.ParseBytes(data)
	if err != nil {
		return fmt.Errorf("failed to parse FB2: %w", err)
	}

	// Extract metadata
	metadata, err := c.parser.ExtractMetadata(fb2Doc)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Apply overrides
	c.applyMetadataOverrides(metadata)

	// Extract TOC from FB2 document
	tocData, err := c.parser.ExtractTOC(fb2Doc)
	if err != nil {
		return fmt.Errorf("failed to extract TOC: %w", err)
	}

	// Create OPF book
	book := c.createOPFBook(metadata, "", tocData, fb2Doc)

	// Write MOBI
	switch c.options.MobiType {
	case "old", "6":
		return c.writeMOBI6(book, output)
	case "new", "8":
		return c.writeKF8(book, output)
	case "both":
		return c.writeJoint(book, output)
	default:
		return fmt.Errorf("unknown MOBI type: %s", c.options.MobiType)
	}
}

// applyMetadataOverrides applies user-specified metadata overrides
func (c *Converter) applyMetadataOverrides(metadata *fb2.Metadata) {
	if c.options.Title != "" {
		metadata.Title = c.options.Title
	}
	if len(c.options.Authors) > 0 {
		metadata.Authors = c.options.Authors
	}
}

// createOPFBook creates an OPF book from metadata and HTML
func (c *Converter) createOPFBook(metadata *fb2.Metadata, html string, tocData *fb2.TOCData, fb2Doc *fb2.FictionBook) *opf.OEBBook {
	book := opf.NewOEBBook()

	// Set metadata
	book.Metadata = opf.ConvertMetadataFromFB2(
		metadata.Title,
		metadata.Authors,
		metadata.AuthorSort,
		metadata.Publisher,
		metadata.ISBN,
		metadata.Year,
		metadata.Language,
		metadata.PubDate,
		metadata.Series,
		metadata.SeriesIndex,
		metadata.Genres,
		metadata.Keywords,
		metadata.Annotation,
		metadata.Cover,
		metadata.CoverID,
		metadata.CoverExt,
	)

	// Set content
	book.Content = html

	// Build TOC from extracted data
	if tocData != nil && len(tocData.Entries) > 0 {
		c.buildOPFTOC(tocData, book)
	}

	// Add resources - first add cover if explicitly set
	if metadata.CoverID != "" && len(metadata.Cover) > 0 {
		// CoverID already includes the extension (e.g., "cover.jpg")
		book.AddResource(metadata.CoverID, metadata.CoverID,
			"image/"+metadata.CoverExt[1:], metadata.Cover)
	}

	// Add all embedded binaries as resources
	// This ensures that inline images (like in with_cover.fb2) are included
	if fb2Doc != nil && len(fb2Doc.Binaries) > 0 {
		for _, binary := range fb2Doc.Binaries {
			if binary.ID == "" {
				continue
			}

			// Decode base64 data
			data, err := b64.Decode([]byte(binary.Data))
			if err != nil {
				continue
			}

			// Determine media type from content-type
			mediaType := binary.ContentType
			if mediaType == "" {
				// Default to jpeg if unknown
				mediaType = "image/jpeg"
			}

			// Use the binary ID as the resource ID (already has extension in most FB2 files)
			// The href will be the same for EPUB
			book.AddResource(binary.ID, binary.ID, mediaType, data)
		}
	}

	return book
}

// buildOPFTOC builds OPF TOC from extracted FB2 TOC data
func (c *Converter) buildOPFTOC(tocData *fb2.TOCData, book *opf.OEBBook) {
	// The OPF TOC starts with a root entry
	book.TOC.ID = "root"
	book.TOC.Label = book.Metadata.Title

	// Map to track parent entries
	entryMap := make(map[int]*opf.TOCEntry)

	// Add all entries to the TOC
	for _, fb2Entry := range tocData.Entries {
		// Add to parent or root
		if fb2Entry.Parent == nil || fb2Entry.Level == 1 {
			// Top-level entry, add directly to root
			book.TOC.AddChild(fb2Entry.ID, fb2Entry.Label, fb2Entry.Href)
			// Store the added child for reference
			if len(book.TOC.Children) > 0 {
				entryMap[fb2Entry.Level] = book.TOC.Children[len(book.TOC.Children)-1]
			}
		} else {
			// Find parent entry
			if parent, ok := entryMap[fb2Entry.Level-1]; ok {
				parent.AddChild(fb2Entry.ID, fb2Entry.Label, fb2Entry.Href)
				// Store this entry as potential parent
				if len(parent.Children) > 0 {
					entryMap[fb2Entry.Level] = parent.Children[len(parent.Children)-1]
				}
			}
		}
	}
}

// writeEPUB writes EPUB format
func (c *Converter) writeEPUB(book *opf.OEBBook, output io.Writer) error {
	return epub.ConvertOEBToEPUB(book, output)
}

// writeMOBI6 writes MOBI 6 format
func (c *Converter) writeMOBI6(book *opf.OEBBook, output io.Writer) error {
	opts := mobi.DefaultWriteOptions()
	if !c.options.Compression {
		opts.CompressionType = mobi.NoCompression
	}

	return mobi.ConvertOEBToMOBIWithOptions(book, output, opts)
}

// writeKF8 writes KF8 format
func (c *Converter) writeKF8(book *opf.OEBBook, output io.Writer) error {
	opts := kf8.DefaultKF8WriteOptions()
	opts.EnableChunking = c.options.EnableChunking
	opts.TargetChunkSize = c.options.TargetChunkSize

	return kf8.ConvertOEBToKF8WithOptions(book, output, opts)
}

// writeJoint writes a joint MOBI file (MOBI 6 + KF8)
func (c *Converter) writeJoint(book *opf.OEBBook, output io.Writer) error {
	writer := kf8.NewKF8Writer(book)
	opts := kf8.DefaultKF8WriteOptions()
	opts.KF8Boundary = true
	opts.EnableChunking = c.options.EnableChunking
	writer.SetOptions(opts)

	return writer.WriteJointFile(output)
}

// ConvertFile is a convenience function to convert an FB2 file to MOBI
func ConvertFile(inputPath, outputPath string) error {
	converter := NewConverter()
	return converter.Convert(inputPath, outputPath)
}

// ConvertFileWithOptions converts an FB2 file to MOBI with options
func ConvertFileWithOptions(inputPath, outputPath string, options ConvertOptions) error {
	converter := NewConverter()
	converter.SetOptions(options)
	return converter.Convert(inputPath, outputPath)
}

// ExtractMetadata extracts metadata from an FB2 file
func ExtractMetadata(path string) (*fb2.Metadata, error) {
	return fb2.GetMetadataFromFile(path)
}

// ExtractMetadataFromBytes extracts metadata from FB2 data
func ExtractMetadataFromBytes(data []byte) (*fb2.Metadata, error) {
	return fb2.GetMetadataFromBytes(data)
}

// convertToUTF8 converts FB2 data to UTF-8 encoding if needed
func convertToUTF8(data []byte) ([]byte, error) {
	// Extract encoding from XML declaration
	// <?xml version="1.0" encoding="windows-1251"?>
	encoding := detectEncoding(data)

	// If already UTF-8 or no encoding specified, return as-is
	if encoding == "" || encoding == "utf-8" || encoding == "UTF-8" {
		return data, nil
	}

	// Convert from detected encoding to UTF-8
	converted, err := decodeToUTF8(data, encoding)
	if err != nil {
		return nil, err
	}

	// Update XML declaration to UTF-8
	converted = updateXMLDeclaration(converted)

	return converted, nil
}

// updateXMLDeclaration updates the XML declaration to UTF-8 encoding
func updateXMLDeclaration(data []byte) []byte {
	xmlStart := bytes.Index(data, []byte("<?xml"))
	if xmlStart == -1 {
		return data
	}

	xmlEnd := bytes.Index(data[xmlStart:], []byte("?>"))
	if xmlEnd == -1 {
		return data
	}

	// Replace encoding="..." with encoding="utf-8"
	declaration := data[xmlStart : xmlStart+xmlEnd+2]

	// Find and replace encoding attribute
	newDecl := bytes.ReplaceAll(
		declaration,
		[]byte("encoding=\"windows-1251\""),
		[]byte("encoding=\"utf-8\""),
	)
	newDecl = bytes.ReplaceAll(
		newDecl,
		[]byte("encoding='windows-1251'"),
		[]byte("encoding='utf-8'"),
	)

	// Reconstruct data with new declaration
	result := make([]byte, 0, len(data))
	result = append(result, data[:xmlStart]...)
	result = append(result, newDecl...)
	result = append(result, data[xmlStart+xmlEnd+2:]...)

	return result
}

// detectEncoding extracts encoding from XML declaration
func detectEncoding(data []byte) string {
	// Look for <?xml version="1.0" encoding="..."?>
	xmlStart := bytes.Index(data, []byte("<?xml"))
	if xmlStart == -1 {
		return ""
	}

	xmlEnd := bytes.Index(data[xmlStart:], []byte("?>"))
	if xmlEnd == -1 {
		return ""
	}

	declaration := string(data[xmlStart : xmlStart+xmlEnd+2])

	// Find encoding=
	encStart := strings.Index(declaration, "encoding=")
	if encStart == -1 {
		return ""
	}

	// Extract encoding value (quoted)
	rest := declaration[encStart+9:] // Skip 'encoding='
	if len(rest) == 0 {
		return ""
	}

	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return ""
	}

	encEnd := strings.Index(rest[1:], string(quote))
	if encEnd == -1 {
		return ""
	}

	encoding := rest[1 : encEnd+1]
	return strings.ToLower(encoding)
}

// decodeToUTF8 converts data from specified encoding to UTF-8
func decodeToUTF8(data []byte, encoding string) ([]byte, error) {
	// Map common encodings to IANA names
	encodingMap := map[string]string{
		"windows-1251": "windows-1251",
		"cp1251":       "windows-1251",
		"1251":         "windows-1251",
		"windows-1252": "windows-1252",
		"cp1252":       "windows-1252",
		"1252":         "windows-1252",
		"iso-8859-1":   "iso-8859-1",
		"latin1":       "iso-8859-1",
	}

	// Normalize encoding name
	if normalized, ok := encodingMap[encoding]; ok {
		encoding = normalized
	}

	// Use golang.org/x/text/encoding for conversion
	// For simplicity, we'll use a basic approach for windows-1251
	if encoding == "windows-1251" {
		return convertWindows1251ToUTF8(data)
	}

	// For other encodings, return an error to avoid corrupting the output
	// TODO: Add support for more encodings using golang.org/x/text/encoding
	return nil, fmt.Errorf("unsupported encoding: %s (supported: windows-1251, utf-8)", encoding)
}

// convertWindows1251ToUTF8 converts windows-1251 encoded data to UTF-8
func convertWindows1251ToUTF8(data []byte) ([]byte, error) {
	// windows-1251 to UTF-8 conversion table for Cyrillic
	// This is a simplified approach - for production use golang.org/x/text/encoding
	runes := make([]rune, 0, len(data))

	for _, b := range data {
		if b < 128 {
			// ASCII characters are the same
			runes = append(runes, rune(b))
		} else {
			// windows-1251 Cyrillic characters
			// Map to Unicode code points
			rune, ok := windows1251ToUnicode[b]
			if !ok {
				// Unknown character, use replacement
				runes = append(runes, '�')
			} else {
				runes = append(runes, rune)
			}
		}
	}

	return []byte(string(runes)), nil
}

// windows1251ToUnicode maps windows-1251 byte values to Unicode runes
var windows1251ToUnicode = map[byte]rune{
	0xC0: 0x0410, // А
	0xC1: 0x0411, // Б
	0xC2: 0x0412, // В
	0xC3: 0x0413, // Г
	0xC4: 0x0414, // Д
	0xC5: 0x0415, // Е
	0xC6: 0x0416, // Ж
	0xC7: 0x0417, // З
	0xC8: 0x0418, // И
	0xC9: 0x0419, // Й
	0xCA: 0x041A, // К
	0xCB: 0x041B, // Л
	0xCC: 0x041C, // М
	0xCD: 0x041D, // Н
	0xCE: 0x041E, // О
	0xCF: 0x041F, // П
	0xD0: 0x0420, // Р
	0xD1: 0x0421, // С
	0xD2: 0x0422, // Т
	0xD3: 0x0423, // У
	0xD4: 0x0424, // Ф
	0xD5: 0x0425, // Х
	0xD6: 0x0426, // Ц
	0xD7: 0x0427, // Ч
	0xD8: 0x0428, // Ш
	0xD9: 0x0429, // Щ
	0xDA: 0x042A, // Ъ
	0xDB: 0x042B, // Ы
	0xDC: 0x042C, // Ь
	0xDD: 0x042D, // Э
	0xDE: 0x042E, // Ю
	0xDF: 0x042F, // Я
	0xE0: 0x0430, // а
	0xE1: 0x0431, // б
	0xE2: 0x0432, // в
	0xE3: 0x0433, // г
	0xE4: 0x0434, // д
	0xE5: 0x0435, // е
	0xE6: 0x0436, // ж
	0xE7: 0x0437, // з
	0xE8: 0x0438, // и
	0xE9: 0x0439, // й
	0xEA: 0x043A, // к
	0xEB: 0x043B, // л
	0xEC: 0x043C, // м
	0xED: 0x043D, // н
	0xEE: 0x043E, // о
	0xEF: 0x043F, // п
	0xF0: 0x0440, // р
	0xF1: 0x0441, // с
	0xF2: 0x0442, // т
	0xF3: 0x0443, // у
	0xF4: 0x0444, // ф
	0xF5: 0x0445, // х
	0xF6: 0x0446, // ц
	0xF7: 0x0447, // ч
	0xF8: 0x0448, // ш
	0xF9: 0x0449, // щ
	0xFA: 0x044A, // ъ
	0xFB: 0x044B, // ы
	0xFC: 0x044C, // ь
	0xFD: 0x044D, // э
	0xFE: 0x044E, // ю
	0xFF: 0x044F, // я
	0xA8: 0x0401, // Ё
	0xB8: 0x0451, // ё
}

// ValidateFB2 validates an FB2 file
func ValidateFB2(path string) error {
	parser := fb2.NewParser()
	_, err := parser.ParseFile(path)
	return err
}
