// Package mobi provides MOBI file writing.
package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"sort"
	"strings"

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
	dummyContent := w.resolveImageSources(w.book.Content)
	textRecordCount := (len(dummyContent) + 4095) / 4096
	// firstImageRecord is 0-based absolute index: Header (0) + TextRecords + TOC (optional)
	firstImageRecord := 1 + textRecordCount
	if hasTOC {
		firstImageRecord++
		_ = firstImageRecord // Suppress ineffassign
	}

	// Pass 2: Final resolution with relative indices (1st image = 1)
	resolvedContent := w.resolveImageSources(w.book.Content)
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

		if w.options.CompressionType == PalmDOCCompression {
			// Compress individual chunk
			compressed := compressRecord(chunk)
			textRecords = append(textRecords, compressed)
		} else {
			textRecords = append(textRecords, chunk)
		}
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
		tocINDX, err := w.GenerateTOCIndex(resolvedContent, textRecords)
		if err != nil {
			return fmt.Errorf("failed to generate TOC index: %w", err)
		}

		indxData, err := tocINDX.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode TOC INDX: %w", err)
		}

		tocIndexOffset = uint32(recordIndex)
		palmWriter.AddRecord(indxData, 0, tocIndexOffset*2)
		recordIndex++
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

	// Refactoring createMOBIHeaderRecord call to include FLIS/FCIS/INDX
	mobiHeaderRecord, err = w.createMOBIHeaderRecordExtended(uncompressedSize,
		len(textRecords), // Text record count MUST match the number of text records
		firstTextRecord, int(lastContentRec),
		firstImageIndex, firstNonBookIndex,
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
	mobiHeader.Locale = 1049        // Russian (Language 25 + Sublanguage 1<<10)
	mobiHeader.ExtraRecordFlags = 0 // Disable trailers for simplicity and compatibility

	// Set mandatory structural indices
	mobiHeader.FCISIndex = fcisIndex
	mobiHeader.FLISIndex = flisIndex
	mobiHeader.INDXRecordOffset = indxOffset // Point to TOC index

	// Set IndexKeys to 0xFFFFFFFF (no index keys) to match reference/standard simple MOBI
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

		// Critical for Kindle: CDEType=PDOC (safer than EBOK for sideloading)
		exthWriter.AddType("PDOC")

		// Add ASIN (Type 113) - Use standard format
		exthWriter.AddASIN("B00TEST001")

		// Remove StartReading for now to eliminate potential bad offset issues
		// exthWriter.AddStartReading(0)

		if w.options.CoverImage != nil {
			exthWriter.AddCoverOffset(0)
			exthWriter.AddThumbnailOffset(1)
			exthWriter.AddHasFakeCover(0)
			exthWriter.AddK8CoverImage("kindle:embed:0001")
			mobiHeader.EXTHFlags = mobiHeader.EXTHFlags | 0x10
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

// splitTextRecords splits text into 4KB records and adds trailing bytes
func (w *Writer) splitTextRecords(data []byte) [][]byte {
	var records [][]byte

	const recordSize = 4096
	for i := 0; i < len(data); i += recordSize {
		end := i + recordSize
		if end > len(data) {
			end = len(data)
		}
		record := data[i:end]
		// Add trailing bytes for ExtraRecordFlags=0x02
		trailingRecord := make([]byte, len(record)+4)
		copy(trailingRecord, record)
		trailingRecord[len(record)+3] = 0x84
		records = append(records, trailingRecord)
	}

	return records
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

		w.options.Logger.Debug("TOC entry offset",
			"label", entry.Label,
			"href", entry.Href,
			"offset", offset,
		)

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

// resolveImageSources replaces src="filename" with src="recindex:N"
// If baseIndex is 0, it uses relative indexing (1, 2, 3...)
// If baseIndex is > 0, it uses absolute 1-based indexing (baseIndex + 1, baseIndex + 2...)
func (w *Writer) resolveImageSources(content string) string {
	imageMap := make(map[string]int)
	coverID := w.book.Metadata.CoverID

	currentOffset := 0

	// 2. Map cover (index 0) and thumbnail (index 1)
	// These are relative to FirstImageIndex, starting at 0
	if w.options.CoverImage != nil {
		if coverID != "" {
			imageMap[coverID] = currentOffset
		} else {
			imageMap["cover.jpg"] = currentOffset
		}
		currentOffset++ // cover
		currentOffset++ // thumbnail
	}

	// 3. Map other manifest images
	ids := w.book.GetManifestIDs()
	sort.Strings(ids)

	for _, id := range ids {
		if id == coverID {
			continue
		}
		res, ok := w.book.GetResource(id)
		if !ok || len(res.MediaType) < 6 || res.MediaType[0:5] != "image" {
			continue
		}
		imageMap[id] = currentOffset
		currentOffset++
	}

	// 4. Perform replacements
	re := regexp.MustCompile(`src=["']([^"']+)["']`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		quote := match[4]
		url := match[5 : len(match)-1]
		// Remove # prefix if present
		url = strings.TrimPrefix(url, "#")

		if recIndex, ok := imageMap[url]; ok {
			// MOBI 1-based relative index (relative to FirstImageIndex)
			finalIndex := uint32(recIndex + 1)
			// Calibre replaces src with recindex attribute
			return fmt.Sprintf("recindex=%c%05d%c", quote, finalIndex, quote)
		}
		return match
	})
}

// resolveFileposLinks replaces href="#ID" with filepos=OFFSET for Kindle internal navigation
// This is a multi-pass process to ensure correct offset calculation after string length changes
func resolveFileposLinks(content string) string {
	// Pass 1: Collect all href="#ID" matches and their anchor IDs
	hrefRe := regexp.MustCompile(`href=["']#([^"']+)["']`)
	hrefMatches := hrefRe.FindAllStringSubmatch(content, -1)
	if len(hrefMatches) == 0 {
		return content
	}

	// Pass 2: Replace all href="#X" with placeholder "filepos=Q0000000000Q" (same length for any anchor ID)
	// We use placeholder value 0 first, which keeps consistent length
	placeholderContent := hrefRe.ReplaceAllStringFunc(content, func(match string) string {
		quote := match[5]
		return fmt.Sprintf("filepos=%c%010d%c", quote, 0, quote)
	})

	// Pass 3: Find anchor positions in the modified content
	anchorRe := regexp.MustCompile(`<a\s+id=["']([^"']+)["']`)
	anchorMap := make(map[string]int)
	for _, match := range anchorRe.FindAllStringSubmatchIndex(placeholderContent, -1) {
		if len(match) >= 4 {
			anchorID := placeholderContent[match[2]:match[3]]
			position := match[0]
			anchorMap[anchorID] = position
		}
	}

	// Pass 4: Replace placeholder filepos values with actual positions
	fileposRe := regexp.MustCompile(`filepos=["']0000000000["']`)
	idx := 0
	result := fileposRe.ReplaceAllStringFunc(placeholderContent, func(match string) string {
		if idx >= len(hrefMatches) {
			return match
		}
		anchorID := hrefMatches[idx][1]
		idx++

		quote := match[8]
		if pos, ok := anchorMap[anchorID]; ok {
			return fmt.Sprintf("filepos=%c%010d%c", quote, pos, quote)
		}
		return match
	})

	return result
}
