// Package fb2c provides FB2 to MOBI/EPUB conversion.
package fb2c

import (
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
		MobiType:        "old", // MOBI 6 format
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

	// Encoding conversion is handled by the parser using fb2encoding package
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

// ValidateFB2 validates an FB2 file
func ValidateFB2(path string) error {
	parser := fb2.NewParser()
	_, err := parser.ParseFile(path)
	return err
}
