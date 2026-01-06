// Package mobi provides MOBI file writing.
package mobi

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/htol/fb2c/mobi/index"
	"github.com/htol/fb2c/opf"
)

// WriteOptions contains options for writing MOBI files
type WriteOptions struct {
	CompressionType int // NoCompression=1, PalmDOCCompression=2, HuffCDCompression=17480
	WithEXTH        bool
	Title           string
	CoverImage      []byte
	GenerateTOC     bool
	debug           bool
}

// DefaultWriteOptions returns default write options
func DefaultWriteOptions() WriteOptions {
	return WriteOptions{
		CompressionType: NoCompression, // 1 = no compression (safer compatibility)
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
	textData := []byte(w.book.Content)
	uncompressedSize := len(textData)

	if w.options.CompressionType == PalmDOCCompression {
		textData = CompressPalmDOC(textData)
	}

	palmWriter := NewPalmDBWriter(w.getBookName(), w.options.debug)

	// Calculate record information before creating header
	// Record count is based on UNCOMPRESSED size (for PalmDOC header)
	recordIndex := 0
	firstTextRecord := 1 // After MOBI header record 0
	recordCount := CalculateRecordCount(uncompressedSize)
	lastTextRecord := firstTextRecord + recordCount - 1

	// Calculate first image index (after text records)
	// If cover exists, it will be at firstTextRecord + recordCount
	// Otherwise, it's after all text records
	hasCover := w.options.CoverImage != nil
	hasImages := w.book.HasImages()
	firstImageIndex := uint32(0xFFFFFFFF)   // Default: no images
	firstNonBookIndex := uint32(0xFFFFFFFF) // Default: no non-book records

	if hasCover || hasImages {
		firstImageIndex = uint32(firstTextRecord + recordCount)
		firstNonBookIndex = uint32(firstTextRecord + recordCount)
	}

	// Create MOBI header with correct record indices
	mobiHeaderRecord, err := w.createMOBIHeaderRecord(len(w.book.Content), firstTextRecord, lastTextRecord, firstImageIndex, firstNonBookIndex)
	if err != nil {
		return fmt.Errorf("failed to create MOBI header: %w", err)
	}

	palmWriter.AddRecord(mobiHeaderRecord, 0, uint32(recordIndex))
	recordIndex++

	// Split and add text records
	textRecords := w.splitTextRecords(textData)
	for _, rec := range textRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex))
		recordIndex++
	}

	// Add cover image if present
	if w.options.CoverImage != nil {
		coverRecord := w.createImageRecord(w.options.CoverImage, "cover.jpg")
		palmWriter.AddRecord(coverRecord, 0, uint32(recordIndex))
		recordIndex++
	}

	// Add other images from manifest
	w.addImages(palmWriter, &recordIndex)

	// Generate thumbnail if cover exists (simple resize to thumbnail dimensions)
	if w.options.CoverImage != nil {
		thumbnailData := w.generateThumbnail(w.options.CoverImage)
		if thumbnailData != nil {
			thumbnailRecord := w.createImageRecord(thumbnailData, "thumb.jpg")
			palmWriter.AddRecord(thumbnailRecord, 0, uint32(recordIndex))
			recordIndex++
		}
	}

	if w.options.GenerateTOC && len(w.book.TOC.Children) > 0 {
		tocINDX, err := w.GenerateTOCIndex(w.book.Content, textRecords)
		if err != nil {
			return fmt.Errorf("failed to generate TOC index: %w", err)
		}

		indxData, err := tocINDX.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode TOC INDX: %w", err)
		}

		palmWriter.AddRecord(indxData, 0, uint32(recordIndex))
		recordIndex++
	}

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
func (w *Writer) createMOBIHeaderRecord(textSize int, firstTextRec, lastTextRec int, firstImageIndex, firstNonBookIndex uint32) ([]byte, error) {
	var buf bytes.Buffer

	// Calculate record count
	recordCount := CalculateRecordCount(textSize)

	// Create MOBI header
	mobiHeader := NewMOBIHeader(textSize, recordCount)

	// Set content record indices - tells readers which records contain the book text
	mobiHeader.FirstContentRec = uint16(firstTextRec)
	mobiHeader.LastContentRec = uint16(lastTextRec)

	// Set compression type to match actual compression
	mobiHeader.Compression = uint16(w.options.CompressionType)

	// Set first image index
	mobiHeader.FirstImageIndex = firstImageIndex

	// Set first non-book index (first image or other non-text record)
	mobiHeader.FirstNonBookIndex = firstNonBookIndex

	// Set title
	bookName := w.getBookName()
	mobiHeader.SetFullName(bookName)

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

		// Add cover-related EXTH records if cover image is present
		if w.options.CoverImage != nil {
			// Cover offset: 0 means first image is the cover
			exthWriter.AddCoverOffset(0)
			// Thumbnail offset: 1 means second image is the thumbnail
			exthWriter.AddThumbnailOffset(1)
			// Has fake cover: 0 means real cover
			exthWriter.AddHasFakeCover(0)
			// K8 cover image identifier
			exthWriter.AddK8CoverImage("kindle:embed:0001")
			// Update EXTH flags to include cover bit (0x40 | 0x10 = 0x50)
			mobiHeader.EXTHFlags = mobiHeader.EXTHFlags | 0x10
		}

		// Get EXTH length for calculating full name offset
		exthLength := exthWriter.GetTotalLength()

		// Calculate full name offset: PalmDOC header (16) + MOBI header (232) + EXTH length
		// Note: mobiHeader.Write() writes 248 bytes total (including PalmDOC)
		mobiHeader.FullNameOffset = uint32(248 + exthLength)

		// Write MOBI header first
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}

		// Write EXTH after MOBI header (before full name)
		if _, err := exthWriter.Write(&buf); err != nil {
			return nil, fmt.Errorf("failed to write EXTH: %w", err)
		}

		// Write full name string after EXTH
		buf.WriteString(bookName)
	} else {
		// Set full name offset (PalmDOC header 16 + MOBI header 232 = 248)
		mobiHeader.FullNameOffset = 248

		// Write MOBI header without EXTH
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}

		// Write full name string
		buf.WriteString(bookName)
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

// generateThumbnail creates a thumbnail from cover image
// For now, returns the original image as thumbnail (simplified approach)
// A full implementation would resize to thumbnail dimensions (e.g., 154x240)
func (w *Writer) generateThumbnail(coverData []byte) []byte {
	// Simplified: return the same image as thumbnail
	// In a full implementation, this would resize the image to thumbnail dimensions
	// using an image processing library
	return coverData
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
