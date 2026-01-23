// Package fb2c provides FB2 to MOBI/EPUB conversion.
package fb2c

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/htol/fb2c/epub"
	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/internal/mapper"
	"github.com/htol/fb2c/mobi"
	"github.com/htol/fb2c/mobi/kf8"
	"github.com/htol/fb2c/opf"
)

// ConvertOptions contains options for FB2 to MOBI/EPUB conversion
type ConvertOptions struct {
	// Format options
	MobiType    string // "old" (MOBI 6), "new" (KF8), "both" (joint)
	Compression bool   // Enable PalmDOC compression

	// Metadata overrides
	Title      string
	Authors    []string
	CoverImage string

	Logger *slog.Logger
}

// DefaultConvertOptions returns default conversion options
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		MobiType:    "old", // MOBI 6 format
		Compression: false, // Compression is currently broken, do not enable
		Logger:      slog.Default(),
	}
}

const (
	MobiTypeOld  = "old"
	MobiTypeNew  = "new"
	MobiTypeBoth = "both"
)

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

	// Process FB2 data (Parse -> Metadata -> HTML -> OPF)
	book, err := c.processFB2(fb2Data)
	if err != nil {
		return err
	}

	// Detect output format from file extension
	ext := strings.ToLower(filepath.Ext(outputPath))

	// Write output based on format
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if ext == ".epub" {
		return c.writeEPUB(book, outputFile)
	}
	return c.writeOutput(book, outputFile)
}

// ConvertStream converts FB2 from reader to MOBI writer
func (c *Converter) ConvertStream(input io.Reader, output io.Writer) error {
	// Read FB2
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Process FB2 data (Parse -> Metadata -> HTML -> OPF)
	book, err := c.processFB2(data)
	if err != nil {
		return err
	}

	// Write MOBI
	return c.writeOutput(book, output)
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

// writeEPUB writes EPUB format
func (c *Converter) writeEPUB(book *opf.OEBBook, output io.Writer) error {
	return epub.ConvertOEBToEPUB(book, output)
}

// writeMOBI6 writes MOBI 6 format
func (c *Converter) writeMOBI6(book *opf.OEBBook, output io.Writer) error {
	opts := mobi.DefaultWriteOptions()
	// Properly set compression based on options
	if c.options.Compression {
		opts.CompressionType = mobi.PalmDOCCompression // Type 2
	} else {
		opts.CompressionType = mobi.NoCompression // Type 1
	}

	// Propagate logger
	opts.Logger = c.options.Logger

	opts.Logger.Debug("writeMOBI6 calling ConvertOEBToMOBIWithOptions",
		"component", "converter.go",
		"Compression", c.options.Compression,
		"CompressionType", opts.CompressionType)

	// Pass cover image from book metadata if available
	if book.Metadata.Cover != nil {
		opts.CoverImage = book.Metadata.Cover
	}

	return mobi.ConvertOEBToMOBIWithOptions(book, output, opts)
}

// writeKF8 writes KF8 format
func (c *Converter) writeKF8(book *opf.OEBBook, output io.Writer) error {
	opts := kf8.DefaultKF8WriteOptions()
	opts.Logger = c.options.Logger

	return kf8.ConvertOEBToKF8WithOptions(book, output, opts)
}

// writeJoint writes a joint MOBI file (MOBI 6 + KF8)
func (c *Converter) writeJoint(book *opf.OEBBook, output io.Writer) error {
	writer := kf8.NewKF8Writer(book)
	opts := kf8.DefaultKF8WriteOptions()
	opts.KF8Boundary = true
	writer.SetOptions(opts)

	return writer.WriteJointFile(output)
}

// processFB2 handles the core parsing and conversion logic
func (c *Converter) processFB2(data []byte) (*opf.OEBBook, error) {
	// Encoding conversion is handled by the parser using fb2encoding package
	fb2Doc, err := c.parser.ParseBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse FB2: %w", err)
	}

	metadata, err := c.parser.ExtractMetadata(fb2Doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Apply metadata overrides
	c.applyMetadataOverrides(metadata)

	// Transform to HTML
	transformer := fb2.NewTransformer()
	// Enable MOBI mode by default for pipeline
	transformer.MOBIMode = true

	// Share parser state to avoid re-parsing
	transformer.SetParser(c.parser)

	html, _, _, err := transformer.Transform(fb2Doc)
	if err != nil {
		return nil, fmt.Errorf("failed to transform FB2: %w", err)
	}

	// Extract TOC from FB2 document
	tocData, err := c.parser.ExtractTOC(fb2Doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract TOC: %w", err)
	}

	// Create OPF book using the mapper service
	book, err := mapper.FromFB2(metadata, html, tocData, fb2Doc)
	if err != nil {
		return nil, fmt.Errorf("failed to map FB2 to OPF: %w", err)
	}

	return book, nil
}

// writeOutput writes the book to the output writer based on configuration
func (c *Converter) writeOutput(book *opf.OEBBook, output io.Writer) error {
	switch c.options.MobiType {
	case MobiTypeOld, "6":
		return c.writeMOBI6(book, output)
	case MobiTypeNew, "8":
		return c.writeKF8(book, output)
	case MobiTypeBoth:
		return c.writeJoint(book, output)
	default:
		return fmt.Errorf("unknown MOBI type: %s", c.options.MobiType)
	}
}
