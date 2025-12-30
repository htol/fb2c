// Package mobi provides MOBI file writing.
package mobi

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/htol/fb2c/opf"
	"github.com/htol/fb2c/mobi/index"
)

// WriteOptions contains options for writing MOBI files
type WriteOptions struct {
	CompressionType int // 0=none, 1=PalmDOC, 2=HuffCD
	WithEXTH        bool
	Title           string
	CoverImage      []byte
	GenerateTOC     bool
}

// DefaultWriteOptions returns default write options
func DefaultWriteOptions() WriteOptions {
	return WriteOptions{
		CompressionType: 1, // PalmDOC compression
		WithEXTH:        true,
		GenerateTOC:     true,
	}
}

// Writer writes MOBI files
type Writer struct {
	options WriteOptions
	book    *opf.OEBBook
}

// NewWriter creates a new MOBI writer
func NewWriter(book *opf.OEBBook) *Writer {
	return &Writer{
		options: DefaultWriteOptions(),
		book:    book,
	}
}

// SetOptions sets write options
func (w *Writer) SetOptions(options WriteOptions) {
	w.options = options
}

// GetBookName returns the book name for the database
func (w *Writer) GetBookName() string {
	name := w.options.Title
	if name == "" {
		name = w.book.Metadata.Title
	}
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

// Write writes the MOBI file
func (w *Writer) Write(output io.Writer) error {
	// 1. Prepare text content
	textData := []byte(w.book.Content)

	// 2. Compress text if requested
	if w.options.CompressionType == 1 {
		textData = CompressPalmDOC(textData)
	}

	// 3. Create PalmDB writer
	palmWriter := NewPalmDBWriter(w.getBookName())

	// 4. Create records
	recordIndex := 0

	// Record 0: MOBI header
	mobiHeaderRecord, err := w.createMOBIHeaderRecord(len(w.book.Content))
	if err != nil {
		return fmt.Errorf("failed to create MOBI header: %w", err)
	}
	palmWriter.AddRecord(mobiHeaderRecord, 0, uint32(recordIndex))
	recordIndex++

	// Text records come after MOBI header
	firstTextRecord := recordIndex

	// Split text into records
	textRecords := w.splitTextRecords(textData)
	for _, rec := range textRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex))
		recordIndex++
	}
	lastTextRecord := recordIndex - 1

	// Image records
	if w.options.CoverImage != nil {
		coverRecord := w.createImageRecord(w.options.CoverImage, "cover.jpg")
		palmWriter.AddRecord(coverRecord, 0, uint32(recordIndex))
		recordIndex++
	}

	// Add other images from manifest
	imageIndices := w.addImages(palmWriter, &recordIndex)

	// 5. Generate and add TOC INDX if requested
	if w.options.GenerateTOC && len(w.book.TOC.Children) > 0 {
		// Generate TOC index with HTML content
		tocINDX, err := w.GenerateTOCIndex(w.book.Content, textRecords)
		if err != nil {
			return fmt.Errorf("failed to generate TOC index: %w", err)
		}

		// Encode INDX to bytes
		indxData, err := tocINDX.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode TOC INDX: %w", err)
		}

		// Add INDX as a record
		palmWriter.AddRecord(indxData, 0, uint32(recordIndex))
		recordIndex++
	}

	// 6. Update MOBI header with record info
	w.updateMOBIHeader(firstTextRecord, lastTextRecord, imageIndices)

	// 7. Write PalmDB
	if err := palmWriter.Write(output); err != nil {
		return fmt.Errorf("failed to write PalmDB: %w", err)
	}

	return nil
}

// getBookName returns the book name for the database
func (w *Writer) getBookName() string {
	name := w.options.Title
	if name == "" {
		name = w.book.Metadata.Title
	}
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

// createMOBIHeaderRecord creates the MOBI header record
func (w *Writer) createMOBIHeaderRecord(textSize int) ([]byte, error) {
	var buf bytes.Buffer

	// Calculate record count
	recordCount := CalculateRecordCount(textSize)

	// Create MOBI header
	mobiHeader := NewMOBIHeader(textSize, recordCount)

	// Set title
	mobiHeader.SetFullName(w.getBookName())

	// Create EXTH header if requested
	if w.options.WithEXTH {
		exthWriter := NewEXTHWriter()

		// Add metadata
		authors := make([]string, 0)
		for _, author := range w.book.Metadata.Authors {
			authors = append(authors, author.FullName)
		}

		exthWriter.AddFromMetadata(
			w.book.Metadata.Title,
			joinStrings(authors, ", "),
			w.book.Metadata.Publisher,
			w.book.Metadata.ISBN,
			w.book.Metadata.Year,
			w.book.Metadata.Annotation,
			w.book.Metadata.Rights,
		)

		// Write EXTH
		exthData := bytes.NewBuffer(nil)
		if _, err := exthWriter.Write(exthData); err != nil {
			return nil, fmt.Errorf("failed to write EXTH: %w", err)
		}

		// Write MOBI header
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}

		// Write EXTH
		buf.Write(exthData.Bytes())
	} else {
		// Write MOBI header without EXTH
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// splitTextRecords splits text into 4KB records
func (w *Writer) splitTextRecords(data []byte) [][]byte {
	var records [][]byte

	const recordSize = 4096
	for i := 0; i < len(data); i += recordSize {
		end := i + recordSize
		if end > len(data) {
			end = len(data)
		}
		records = append(records, data[i:end])
	}

	return records
}

// createImageRecord creates an image record
func (w *Writer) createImageRecord(data []byte, filename string) []byte {
	return data
}

// addImages adds images from the OEB book manifest
func (w *Writer) addImages(palmWriter *PalmDBWriter, recordIndex *int) map[string]int {
	indices := make(map[string]int)

	// Get sorted resource IDs
	ids := w.book.GetManifestIDs()

	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}

		// Skip non-images
		if len(res.MediaType) < 6 || res.MediaType[0:5] != "image" {
			continue
		}

		// Add image record
		palmWriter.AddRecord(res.Data, 0, uint32(*recordIndex))
		indices[id] = *recordIndex
		(*recordIndex)++
	}

	return indices
}

// updateMOBIHeader updates the MOBI header with final values
func (w *Writer) updateMOBIHeader(firstTextRec, lastTextRec int, imageIndices map[string]int) {
	// In a real implementation, would update the header with calculated values
	// For now, this is a placeholder
}

// CalculateRecordCount calculates the number of records for text
func CalculateRecordCount(textSize int) int {
	const recordSize = 4096
	count := textSize / recordSize
	if textSize%recordSize != 0 {
		count++
	}
	return count
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// GenerateTOCIndex generates a TOC index from the book's TOC with proper offsets
func (w *Writer) GenerateTOCIndex(htmlContent string, textRecords [][]byte) (*index.INDX, error) {
	builder := index.NewTOCIndexBuilder()

	// Set text records for offset calculation
	builder.SetTextRecords(textRecords)

	// Build TOC from OEB book
	flatEntries := w.book.TOC.Flatten()

	for _, entry := range flatEntries {
		if entry.ID == "root" {
			continue
		}

		// Calculate offset from HTML by scanning for entry.Href
		offset := builder.FindOffsetForHref(htmlContent, entry.Href)

		// Add entry with calculated offset
		builder.AddEntry(entry.Label, entry.Href, uint32(entry.Level), offset)
	}

	return builder.Build()
}

// ConvertOEBToMOBI is a convenience function to convert OEBBook to MOBI
func ConvertOEBToMOBI(book *opf.OEBBook, output io.Writer) error {
	writer := NewWriter(book)
	return writer.Write(output)
}

// ConvertOEBToMOBIWithOptions converts OEBBook to MOBI with options
func ConvertOEBToMOBIWithOptions(book *opf.OEBBook, output io.Writer, options WriteOptions) error {
	writer := NewWriter(book)
	writer.SetOptions(options)
	return writer.Write(output)
}

// SortManifestIDs returns sorted manifest resource IDs
func SortManifestIDs(book *opf.OEBBook) []string {
	ids := book.GetManifestIDs()
	sort.Strings(ids)
	return ids
}
