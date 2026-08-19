// Package mobi provides MOBI file writing.
package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

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

// GetBookName returns the book title (options.Title override wins). It is
// intentionally NOT truncated: the MOBI FullName field is offset+length in
// record 0 with no size limit, and the PalmDB name is transliterated to ASCII
// and truncated to 31 chars by NewPalmDBHeader, after transliteration.
func (w *Writer) GetBookName() string {
	if w.options.Title != "" {
		return w.options.Title
	}
	return w.book.Metadata.Title
}

// Write writes the MOBI file.
//
// Declarative layout: every record is built as a byte slice first, all
// header indices are derived from the slice lengths, and record 0 is built
// exactly once from the finished layout.
func (w *Writer) Write(output io.Writer) error {
	w.options.Logger.Debug("starting MOBI file assembly",
		"component", "MOBIWriter",
		"operation", "Write",
		"title", w.options.Title,
		"hasTOC", w.options.GenerateTOC && len(w.book.TOC.Children) > 0,
		"tocEntries", len(w.book.TOC.Children),
		"compressionType", w.options.CompressionType,
	)

	// 1. Book text: resolve image sources and links, split into records.
	// PalmDOC compression (disabled for now) would compress each 4096-byte
	// chunk of this UNCOMPRESSED text.
	textData := w.prepareTextContent()
	uncompressedSize := len(textData)
	textRecords := w.splitAndCompressRecords(textData)

	w.options.Logger.Debug("processing text data",
		"component", "MOBIWriter",
		"operation", "Write",
		"uncompressedSize", uncompressedSize,
		"textRecordCount", len(textRecords),
		"compressionEnabled", w.options.CompressionType == PalmDOCCompression,
	)

	// 2. Lay out every record that follows record 0.
	tocRecords, err := w.buildTOCRecords(string(textData), textRecords)
	if err != nil {
		return err
	}
	imageRecords := w.buildImageRecords()

	// 3. Derive all header indices from the layout.
	// Final record order: header | text | TOC | images | FLIS | FCIS | EOF.
	const firstTextRecord = 1 // after the record-0 header
	lastTextRecord := firstTextRecord + len(textRecords) - 1
	next := uint32(firstTextRecord + len(textRecords)) //nolint:gosec // Book sizes are far below uint32

	firstNonBookIndex := next
	tocIndexOffset := uint32(0xFFFFFFFF)
	if len(tocRecords) > 0 {
		tocIndexOffset = next
		next += uint32(len(tocRecords)) //nolint:gosec // Count fits
	}

	firstImageIndex := uint32(0xFFFFFFFF)
	if len(imageRecords) > 0 {
		firstImageIndex = next
		next += uint32(len(imageRecords)) //nolint:gosec // Count fits
	}

	// Spec §3/§12: the content-record range spans text records PLUS images
	// (reference: LastContentRec=368, the last image record, text ends at 363).
	lastContentRec := uint32(lastTextRecord) //nolint:gosec // Book sizes are far below uint32
	if len(imageRecords) > 0 {
		lastContentRec = next - 1
	}

	flisIndex := next
	fcisIndex := next + 1

	// With a TOC, FirstNonBookIndex points at it for best compatibility.
	firstNonBookIndexField := firstNonBookIndex
	if tocIndexOffset != 0xFFFFFFFF {
		firstNonBookIndexField = tocIndexOffset
	}

	// 4. Build record 0 once, from the complete layout.
	layout := recordLayout{
		textRecordCount:   len(textRecords),
		firstTextRecord:   firstTextRecord,
		lastContentRec:    int(lastContentRec),
		firstImageIndex:   firstImageIndex,
		firstNonBookIndex: firstNonBookIndexField,
		flisIndex:         flisIndex,
		fcisIndex:         fcisIndex,
		indxOffset:        tocIndexOffset,
	}
	mobiHeaderRecord, err := w.createMOBIHeaderRecordExtended(uncompressedSize, layout)
	if err != nil {
		return fmt.Errorf("failed to create MOBI header: %w", err)
	}

	// 5. Assemble and write.
	all := make([][]byte, 0, 1+len(textRecords)+len(tocRecords)+len(imageRecords)+3)
	all = append(all, mobiHeaderRecord)
	all = append(all, textRecords...)
	all = append(all, tocRecords...)
	all = append(all, imageRecords...)
	all = append(all, createFLISRecord(), createFCISRecord(uint32(uncompressedSize)), //nolint:gosec // book text stays far below 4 GB
		// EOF record (MOBI spec: E9 8E 0D 0A)
		[]byte{0xE9, 0x8E, 0x0D, 0x0A})

	palmWriter := NewPalmDBWriter(w.GetBookName(), w.options.Logger)
	for i, rec := range all {
		palmWriter.AddRecord(rec, 0, uint32(i)) //nolint:gosec // Index fits
	}

	w.options.Logger.Debug("record layout assembled",
		"component", "MOBIWriter",
		"operation", "Write",
		"totalRecords", len(all),
		"textRecords", len(textRecords),
		"tocRecords", len(tocRecords),
		"imageRecords", len(imageRecords),
		"firstTextRecord", firstTextRecord,
		"lastContentRec", lastContentRec,
		"firstImageIndex", firstImageIndex,
		"firstNonBookIndex", firstNonBookIndexField,
		"indxRecordOffset", tocIndexOffset,
		"flisIndex", flisIndex,
		"fcisIndex", fcisIndex,
	)

	if err := palmWriter.Write(output); err != nil {
		return fmt.Errorf("failed to write PalmDB: %w", err)
	}

	return nil
}

// localeForLanguage maps the book language (BCP-47-ish code, e.g. "ru",
// "en", "en-GB") to the Windows locale identifier expected in MOBI+0x5C
// (spec §3). Unknown or empty languages fall back to neutral English (9),
// the primary-language-only form the reference file uses for Russian (25).
func localeForLanguage(lang string) uint32 {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(lang)), func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return 9
	}
	primary, region := parts[0], ""
	if len(parts) > 1 {
		region = parts[1]
	}

	switch primary {
	case "ru":
		return 1049 // ru-RU
	case "en":
		if region == "gb" {
			return 2057 // en-GB
		}
		return 1033 // en-US
	default:
		return 9
	}
}

// recordLayout is the finished record layout of the MOBI file: every header
// index Write derives from the assembled record slices. record 0 is built
// from it in one pass and never patched afterwards.
type recordLayout struct {
	textRecordCount   int
	firstTextRecord   int
	lastContentRec    int
	firstImageIndex   uint32
	firstNonBookIndex uint32
	flisIndex         uint32
	fcisIndex         uint32
	indxOffset        uint32
}

// createMOBIHeaderRecordExtended builds record 0: PalmDOC header, MOBI
// header and (optionally) EXTH metadata plus the full name, from the
// finished record layout.
func (w *Writer) createMOBIHeaderRecordExtended(textSize int, layout recordLayout) ([]byte, error) {
	var buf bytes.Buffer

	// Create MOBI header with REAL text record count (Record 0)
	// This ensures the reader stops DECODING text before it hits binary images.
	mobiHeader := NewHeader(textSize, layout.textRecordCount)

	// Set content record indices
	mobiHeader.FirstContentRec = uint16(layout.firstTextRecord) //nolint:gosec // Limit verified
	mobiHeader.LastContentRec = uint16(layout.lastContentRec)   //nolint:gosec // Limit verified

	// Set header flags for UTF-8 and structure
	mobiHeader.TextEncoding = UTF8Encoding
	mobiHeader.Locale = localeForLanguage(w.book.Metadata.Language)

	// ExtraRecordFlags = 0 means no trailing data on text records
	// We don't use TBS indexing - TOC works via INDX records without it
	mobiHeader.ExtraRecordFlags = 0

	// Set mandatory structural indices
	mobiHeader.FCISIndex = layout.fcisIndex
	mobiHeader.FLISIndex = layout.flisIndex
	mobiHeader.INDXRecordOffset = layout.indxOffset // Point to TOC index

	// Set IndexKeys to 0xFFFFFFFF to match reference
	mobiHeader.IndexKeys = 0xFFFFFFFF

	// Set compression type
	mobiHeader.Compression = uint16(w.options.CompressionType) //nolint:gosec // Enum values fit

	// Set image indices
	mobiHeader.FirstImageIndex = layout.firstImageIndex
	mobiHeader.FirstNonBookIndex = layout.firstNonBookIndex

	// Set title
	bookName := w.GetBookName()
	mobiHeader.SetFullName(bookName)
	mobiHeader.UniqueID = generateUniqueID(bookName)

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

		// cdeType must be PDOC, not EBOK: EBOK + ASIN routes the firmware into
		// its "store book" path, where the shelf-thumbnail generator dies on an
		// unvalidatable ASIN and leaves a 0-byte thumbnail_<ASIN>_EBOK_portrait.jpg
		// .tmp.partial (verified on-device 2026-08-18; probes B/D–G). PDOC is the
		// honest value for a sideload and renders tiles from the EXTH 201 cover.
		exthWriter.AddType("PDOC")

		// Add ASIN (Type 113): deterministic per-book UUID (reference carries
		// a per-book UUID here too); a constant broke Kindle library grouping
		exthWriter.AddASIN(opf.BookID(w.book))

		if w.options.CoverImage != nil {
			exthWriter.AddCoverOffset(0)
			exthWriter.AddThumbnailOffset(1)
		}

		exthLength := exthWriter.GetTotalLength()
		// FullNameOffset = PalmDOC Header (16) + MOBI Header (264 usually) + EXTH Length
		// Use the constant to be safe
		mobiHeaderOffset := uint32(16 + HeaderSize)
		mobiHeader.FullNameOffset = mobiHeaderOffset + uint32(exthLength) //nolint:gosec // Offset fits

		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}

		if _, err := exthWriter.Write(&buf); err != nil {
			return nil, fmt.Errorf("failed to write EXTH: %w", err)
		}
		buf.WriteString(bookName)
		// Spec §5: the full name is followed by two NUL bytes, then the record
		// is padded to a 4-byte boundary.
		buf.Write([]byte{0, 0})
	} else {
		mobiHeader.FullNameOffset = uint32(16 + HeaderSize)
		if err := mobiHeader.Write(&buf); err != nil {
			return nil, err
		}
		buf.WriteString(bookName)
		buf.Write([]byte{0, 0})
	}

	// Pad with zeros to ensure 4-byte alignment
	// This helps with some readers/tools that expect aligned records (e.g. mobitool)
	if padding := buf.Len() % 4; padding > 0 {
		buf.Write(make([]byte, 4-padding))
	}

	return buf.Bytes(), nil
}

// buildImageRecords lays out the image records in stable order: the cover,
// its thumbnail, then every manifest image (the cover excluded), sorted by
// resource ID.
func (w *Writer) buildImageRecords() [][]byte {
	var records [][]byte
	if w.options.CoverImage != nil {
		records = append(records, encodeCoverJPEG(w.options.CoverImage))
		if thumb := buildThumbnail(w.options.CoverImage); thumb != nil {
			records = append(records, thumb)
		}
	}

	skipID := w.book.Metadata.CoverID // already added above, if present
	ids := w.book.GetManifestIDs()
	sort.Strings(ids)
	for _, id := range ids {
		if id == skipID {
			continue
		}
		res, ok := w.book.GetResource(id)
		if !ok || len(res.MediaType) < 6 || res.MediaType[0:5] != imageMediaTypePrefix {
			continue
		}
		records = append(records, res.Data)
	}
	return records
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
