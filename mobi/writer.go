// Package mobi provides MOBI file writing.
package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sort"

	"github.com/htol/fb2c/opf"
)

// WriteOptions contains options for writing MOBI files
type WriteOptions struct {
	CompressionType int // NoCompression=1, PalmDOCCompression=2, HuffCDCompression=17480
	WithEXTH        bool
	Title           string
	CoverImage      []byte
	GenerateTOC     bool
	Logger          *slog.Logger
}

// DefaultWriteOptions returns default write options
func DefaultWriteOptions() WriteOptions {
	return WriteOptions{
		CompressionType: NoCompression,
		WithEXTH:        true,
		GenerateTOC:     true,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
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

// CalculateRecordCount calculates the number of records for text
func CalculateRecordCount(textSize int) int {
	const recordSize = 4096
	count := textSize / recordSize
	if textSize%recordSize != 0 {
		count++
	}
	return count
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

	w.options.Logger.Debug("starting MOBI file assembly",
		"component", "MOBIWriter",
		"operation", "Write",
		"title", w.options.Title,
		"hasTOC", w.options.GenerateTOC && len(w.book.TOC.Children) > 0,
		"tocEntries", len(w.book.TOC.Children),
		"compressionType", w.options.CompressionType,
	)

	// 1. Resolve image sources and calculate final text size
	// We do this in two passes to get absolute record indices
	hasTOC := w.options.GenerateTOC && len(w.book.TOC.Children) > 0

	// Pass 1: Dummy resolution to get final text size
	dummyContent := resolveImageSources(w.book, w.options.CoverImage != nil, w.book.Content)
	textRecordCount := (len(dummyContent) + 4095) / 4096
	// firstImageRecord is 0-based absolute index: Header (0) + TextRecords + TOC (optional)
	firstImageRecord := 1 + textRecordCount
	if hasTOC {
		firstImageRecord++
		_ = firstImageRecord // Suppress ineffassign
	}

	// Pass 2: Final resolution with relative indices (1st image = 1)
	resolvedContent := resolveImageSources(w.book, w.options.CoverImage != nil, w.book.Content)
	// Convert href="#ID" to filepos=OFFSET for Kindle internal navigation
	resolvedContent = resolveFileposLinks(resolvedContent)
	textData := []byte(resolvedContent)

	uncompressedSize := len(textData)

	w.options.Logger.Debug("processing text data",
		"component", "MOBIWriter",
		"operation", "Write",
		"uncompressedSize", uncompressedSize,
		"compressionEnabled", w.options.CompressionType == PalmDOCCompression,
	)

	// Split and compress records
	// PalmDOC requires comperssing 4096-byte chunks of UNCOMPRESSED text
	var textRecords [][]byte
	const recordSize = 4096

	for i := 0; i < len(textData); i += recordSize {
		end := i + recordSize
		if end > len(textData) {
			end = len(textData)
		}

		chunk := textData[i:end]
		record := chunk
		if w.options.CompressionType == PalmDOCCompression {
			record = compressRecord(chunk)
		}

		// In MOBI with ExtraRecordFlags=0x02 (TBS indexing), we add a 4-byte trailer.
		// The last byte 0x84 indicates 4 bytes of extra data (including the size byte).
		// We use 00 00 00 84 to keep it safe. Reference uses 86 80 02 84.
		trailingRecord := make([]byte, len(record)+4)
		copy(trailingRecord, record)
		trailingRecord[len(record)+3] = 0x84
		textRecords = append(textRecords, trailingRecord)
	}

	w.options.Logger.Debug("creating PalmDBWriter with records",
		"component", "MOBIWriter",
		"operation", "Write",
		"textRecordCount", len(textRecords),
		"bookName", w.getBookName(),
	)

	palmWriter := NewPalmDBWriter(w.getBookName(), w.options.Logger)

	// Calculate record information before creating header
	// Record count is exact number of records we generated
	recordCount := len(textRecords)

	recordIndex := 0
	firstTextRecord := 1 // After MOBI header record 0
	// lastTextRecord is approximated here for the wrapper, but real value is set later
	lastTextRecord := firstTextRecord + recordCount - 1

	// Calculate first image index (after text records)
	// If cover exists, it will be at firstTextRecord + recordCount
	// Otherwise, it's after all text records
	// hasCover := w.options.CoverImage != nil
	// hasImages := w.book.HasImages()
	// FirstNonBookIndex should point to the first record that is NOT content (e.g. INDX, Images)
	// We'll determine it dynamically.
	firstImageIndex := uint32(0xFFFFFFFF)
	firstNonBookIndex := uint32(0xFFFFFFFF)

	// Create MOBI header with correct record indices
	// Use uncompressedSize for header
	mobiHeaderRecord, err := w.createMOBIHeaderRecord(uncompressedSize, firstTextRecord, lastTextRecord, firstImageIndex, firstNonBookIndex)
	if err != nil {
		return fmt.Errorf("failed to create MOBI header: %w", err)
	}

	palmWriter.AddRecord(mobiHeaderRecord, 0, uint32(recordIndex)*2)
	recordIndex++

	// 2. Add text records
	for _, rec := range textRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex)*2)
		recordIndex++
	}

	// Correctly set LastContentRec to the last text record added
	lastContentRec := uint32(recordIndex - 1)

	// FirstNonBookIndex starts here (next record)
	firstNonBookIndex = uint32(recordIndex)

	// 3. Add TOC Index Records (NCX) - Two-record structure for native Kindle TOC
	var tocIndexOffset uint32 = 0xFFFFFFFF
	if w.options.GenerateTOC && len(w.book.TOC.Children) > 0 {
		// Use resolvedContent for accurate TOC offset calculation
		tocINDX, err := GenerateTOCIndex(w.book, resolvedContent, textRecords, w.options.Logger)
		if err != nil {
			return fmt.Errorf("failed to generate TOC index: %w", err)
		}

		// Encode as two-record NCX structure (primary + secondary)
		ncxResult, err := tocINDX.EncodeNCXIndex()
		if err != nil {
			// Fallback to single record if NCX encoding fails
			w.options.Logger.Debug("NCX encoding failed, using single INDX",
				"error", err.Error(),
			)
			indxData, encErr := tocINDX.Encode()
			if encErr != nil {
				return fmt.Errorf("failed to encode TOC INDX: %w", encErr)
			}
			tocIndexOffset = uint32(recordIndex)
			palmWriter.AddRecord(indxData, 0, tocIndexOffset*2)
			recordIndex++
		} else {
			// Add primary INDX (meta record) - this is what INDXRecordOffset points to
			tocIndexOffset = uint32(recordIndex)
			palmWriter.AddRecord(ncxResult.PrimaryINDX, 0, tocIndexOffset*2)
			recordIndex++

			// Add secondary INDX (data record with actual TOC entries)
			palmWriter.AddRecord(ncxResult.SecondaryINDX, 0, uint32(recordIndex)*2)
			recordIndex++

			// Add CNCX record (string table with chapter names)
			if len(ncxResult.CNCXRecord) > 0 {
				palmWriter.AddRecord(ncxResult.CNCXRecord, 0, uint32(recordIndex)*2)
				recordIndex++
			}

			w.options.Logger.Debug("NCX TOC generated",
				"primaryRecordIndex", tocIndexOffset,
				"secondaryRecordIndex", tocIndexOffset+1,
				"cncxRecordIndex", tocIndexOffset+2,
				"totalEntries", ncxResult.TotalEntries,
			)
		}
	}

	// 4. Add Images in consistent order: Cover -> Thumbnail -> Manifest

	// 4. Add Images in consistent order: Cover -> Thumbnail -> Manifest
	firstImageIndex = uint32(0xFFFFFFFF)
	coverID := w.book.Metadata.CoverID

	if w.options.CoverImage != nil || w.book.HasImages() {
		firstImageIndex = uint32(recordIndex)

		// 1. Add cover image if present
		if w.options.CoverImage != nil {
			coverRecord := w.options.CoverImage
			palmWriter.AddRecord(coverRecord, 0, uint32(recordIndex)*2)
			recordIndex++

			// 2. Add thumbnail immediately after cover
			thumbnailData := w.generateThumbnail(w.options.CoverImage)
			if thumbnailData != nil {
				thumbnailRecord := thumbnailData
				palmWriter.AddRecord(thumbnailRecord, 0, uint32(recordIndex)*2)
				recordIndex++
			}
		}

		// 3. Add other images from manifest (excluding cover if already added)
		w.addImagesFiltered(palmWriter, &recordIndex, coverID)
	}

	// lastContentRec is already set correctly after text records.
	// Do not Overwrite it.

	// 5. Add Mandatory Structural Records (FLIS, FCIS, EOF)
	flisIndex := uint32(recordIndex)
	palmWriter.AddRecord(createFLISRecord(), 0, flisIndex*2)
	recordIndex++

	fcisIndex := uint32(recordIndex)
	palmWriter.AddRecord(createFCISRecord(uint32(uncompressedSize)), 0, fcisIndex*2)
	recordIndex++

	// EOF record (4 zero bytes)
	palmWriter.AddRecord([]byte{0x00, 0x00, 0x00, 0x00}, 0, uint32(recordIndex)*2)
	recordIndex++

	// If TOC exists, FirstNonBookIndex should point to it for best compatibility
	fnbi := firstNonBookIndex
	if tocIndexOffset != 0xFFFFFFFF {
		fnbi = tocIndexOffset
	}

	// Refactoring createMOBIHeaderRecord call to include FLIS/FCIS/INDX
	mobiHeaderRecord, err = w.createMOBIHeaderRecordExtended(uncompressedSize,
		len(textRecords), // Text record count MUST match the number of text records
		firstTextRecord, int(lastContentRec),
		firstImageIndex, fnbi,
		flisIndex, fcisIndex, tocIndexOffset)
	if err != nil {
		return fmt.Errorf("failed to create extended MOBI header: %w", err)
	}
	w.options.Logger.Debug("mobiHeaderRecord size before SetRecord(0)", "size", len(mobiHeaderRecord))
	palmWriter.SetRecord(0, mobiHeaderRecord)

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
	// Wrapper to maintain backward compatibility if needed, but we'll use Extended internally
	// For legacy wrapper, assume textRecordCount matches range
	textCount := lastTextRec - firstTextRec + 1
	return w.createMOBIHeaderRecordExtended(textSize, textCount, firstTextRec, lastTextRec, firstImageIndex, firstNonBookIndex, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF)
}

// createMOBIHeaderRecordExtended is an extended version that includes mandatory indices
func (w *Writer) createMOBIHeaderRecordExtended(textSize int, textRecordCount int, firstTextRec, lastTextRec int, firstImageIndex, firstNonBookIndex, flisIndex, fcisIndex, indxOffset uint32) ([]byte, error) {
	var buf bytes.Buffer

	// Create MOBI header with REAL text record count (Record 0)
	// This ensures the reader stops DECODING text before it hits binary images.
	mobiHeader := NewMOBIHeader(textSize, textRecordCount)

	// Set content record indices
	mobiHeader.FirstContentRec = uint16(firstTextRec)
	mobiHeader.LastContentRec = uint16(lastTextRec)

	// Set header flags for UTF-8 and structure
	mobiHeader.TextEncoding = UTF8Encoding
	mobiHeader.Locale = 1049 // Russian (Language 25 + Sublanguage 1<<10)

	// Enable ExtraRecordFlags for NCX/TBS indexing when TOC is present
	// Bit 1 (0x01): Multibyte character overlap
	// Bit 2 (0x02): TBS indexing description (required for NCX)
	if indxOffset != 0xFFFFFFFF {
		mobiHeader.ExtraRecordFlags = 0x02 // Enable TBS indexing (0x02) only. 0x01 requires FCIS/FLIS which we don't write.
	} else {
		mobiHeader.ExtraRecordFlags = 0 // No trailing data
	}

	// Set mandatory structural indices
	mobiHeader.FCISIndex = fcisIndex
	mobiHeader.FLISIndex = flisIndex
	mobiHeader.INDXRecordOffset = indxOffset // Point to TOC index

	// Set IndexKeys to 0xFFFFFFFF to match reference
	mobiHeader.IndexKeys = 0xFFFFFFFF

	// Set compression type
	mobiHeader.Compression = uint16(w.options.CompressionType)

	// Set image indices
	mobiHeader.FirstImageIndex = firstImageIndex
	mobiHeader.FirstNonBookIndex = firstNonBookIndex

	// Set title
	bookName := w.getBookName()
	mobiHeader.SetFullName(bookName)

	// Create EXTH header
	if w.options.WithEXTH {
		// Set EXTH flags to 0x50 (Bit 6: EXTH exists + Bit 4: Unknown/Resources?)
		// Reference file has 0x50, while standard 0x40 often suffices.
		// Setting 0x50 to align with reference.
		mobiHeader.SetEXTHFlags(0x50)

		exthWriter := NewEXTHWriter()
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
			w.book.Metadata.Language,
		)

		// Critical for Kindle: CDEType=EBOK (preferred for books)
		exthWriter.AddType("EBOK")

		// Add ASIN (Type 113) - Use standard format
		exthWriter.AddASIN("B00TEST001")

		if w.options.CoverImage != nil {
			exthWriter.AddCoverOffset(0)
			exthWriter.AddThumbnailOffset(1)
		}

		// Add NCX metadata EXTH records when TOC is present
		// These are required for native TOC visibility in Kindle readers
		if indxOffset != 0xFFFFFFFF {
			// Calculate approximate NCX size based on TOC entry count
			tocEntryCount := uint32(len(w.book.TOC.Flatten()) - 1) // Exclude root
			if tocEntryCount > 0 {
				// Approximate NCX size: ~100 bytes per entry is a reasonable estimate
				ncxSize := uint32(500 + tocEntryCount*100)
				exthWriter.AddNCXMetadata(tocEntryCount, ncxSize, indxOffset)
			}
		}

		exthLength := exthWriter.GetTotalLength()
		// FullNameOffset = PalmDOC Header (16) + MOBI Header (264 usually) + EXTH Length
		// Use the constant to be safe
		mobiHeaderOffset := uint32(16 + MOBIHeaderSize)
		mobiHeader.FullNameOffset = mobiHeaderOffset + uint32(exthLength)

		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}

		if _, err := exthWriter.Write(&buf); err != nil {
			return nil, fmt.Errorf("failed to write EXTH: %w", err)
		}
		buf.WriteString(bookName)
	} else {
		mobiHeader.FullNameOffset = uint32(16 + MOBIHeaderSize)
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}
		buf.WriteString(bookName)
	}

	// Pad with zeros to ensure 4-byte alignment
	// This helps with some readers/tools that expect aligned records (e.g. mobitool)
	if padding := buf.Len() % 4; padding > 0 {
		buf.Write(make([]byte, 4-padding))
	}

	return buf.Bytes(), nil
}

// addImagesFiltered adds images from manifest, skipping the cover if provided
func (w *Writer) addImagesFiltered(palmWriter *PalmDBWriter, recordIndex *int, skipID string) {
	ids := w.book.GetManifestIDs()
	sort.Strings(ids)

	for _, id := range ids {
		if id == skipID {
			continue // Skip cover, already added
		}
		res, ok := w.book.GetResource(id)
		const imageType = "image"
		if !ok || len(res.MediaType) < 6 || res.MediaType[0:5] != imageType {
			continue
		}

		palmWriter.AddRecord(res.Data, 0, uint32(*recordIndex)*2)
		(*recordIndex)++
	}
}

// createFLISRecord creates a standard FLIS record (36 bytes)
func createFLISRecord() []byte {
	data := make([]byte, 36)
	copy(data, "FLIS")
	binary.BigEndian.PutUint32(data[4:8], 8)
	binary.BigEndian.PutUint16(data[8:10], 65)
	binary.BigEndian.PutUint16(data[10:12], 0)
	binary.BigEndian.PutUint32(data[12:16], 0)
	binary.BigEndian.PutUint32(data[16:20], 0xFFFFFFFF)
	binary.BigEndian.PutUint16(data[20:22], 1)
	binary.BigEndian.PutUint16(data[22:24], 3)
	binary.BigEndian.PutUint32(data[24:28], 3)
	binary.BigEndian.PutUint32(data[28:32], 1)
	binary.BigEndian.PutUint32(data[32:36], 0xFFFFFFFF)
	return data
}

// createFCISRecord creates a standard FCIS record (44 bytes) for text size
func createFCISRecord(textSize uint32) []byte {
	data := make([]byte, 44)
	copy(data, "FCIS")
	binary.BigEndian.PutUint32(data[4:8], 20)
	binary.BigEndian.PutUint32(data[8:12], 16)
	binary.BigEndian.PutUint32(data[12:16], 1)
	binary.BigEndian.PutUint32(data[16:20], 0)
	binary.BigEndian.PutUint32(data[20:24], textSize)
	binary.BigEndian.PutUint32(data[24:28], 0)
	binary.BigEndian.PutUint32(data[28:32], 32)
	binary.BigEndian.PutUint32(data[32:36], 8)
	binary.BigEndian.PutUint16(data[36:38], 1)
	binary.BigEndian.PutUint16(data[38:40], 1)
	binary.BigEndian.PutUint32(data[40:44], 0)
	return data
}
