// Package kf8 provides KF8 (MOBI 8) writer functionality.
package kf8

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/htol/fb2c/mobi"
	"github.com/htol/fb2c/opf"
)

// KF8WriteOptions contains KF8-specific write options
type KF8WriteOptions struct {
	mobi.WriteOptions

	// KF8-specific options
	EnableChunking  bool
	TargetChunkSize int
	SupportFlows    bool
	GenerateFDST    bool
	KF8Boundary     bool // Insert BOUNDARY marker for joint files
}

// DefaultKF8WriteOptions returns default KF8 write options
func DefaultKF8WriteOptions() KF8WriteOptions {
	return KF8WriteOptions{
		WriteOptions:    mobi.DefaultWriteOptions(),
		EnableChunking:  true,
		TargetChunkSize: TargetChunkSize,
		SupportFlows:    true,
		GenerateFDST:    true,
		KF8Boundary:     false,
	}
}

// KF8Writer writes KF8 (MOBI 8) files
type KF8Writer struct {
	mobiWriter  *mobi.Writer
	skeleton    *Skeleton
	flowManager *FlowManager
	fdst        *FDST
	options     KF8WriteOptions
	book        *opf.OEBBook
}

// NewKF8Writer creates a new KF8 writer
func NewKF8Writer(book *opf.OEBBook) *KF8Writer {
	return &KF8Writer{
		mobiWriter:  mobi.NewWriter(book),
		skeleton:    NewSkeleton(),
		flowManager: NewFlowManager(),
		fdst:        NewFDST(),
		options:     DefaultKF8WriteOptions(),
		book:        book,
	}
}

// SetOptions sets KF8 write options
func (w *KF8Writer) SetOptions(options KF8WriteOptions) {
	w.options = options
	w.mobiWriter.SetOptions(options.WriteOptions)
}

// Write writes the KF8 file
func (w *KF8Writer) Write(output io.Writer) error {
	// 1. Prepare content (chunk if enabled)
	var content string

	if w.options.EnableChunking {
		// Chunk the HTML content
		if err := w.skeleton.ChunkHTML(w.book.Content); err != nil {
			return fmt.Errorf("failed to chunk HTML: %w", err)
		}

		// Build chunk hierarchy
		w.skeleton.BuildHierarchy()

		// Assign AID attributes
		content = w.skeleton.AssignAIDAttributes()

		// Generate FDST from skeleton
		if w.options.GenerateFDST {
			w.fdst.GenerateFromSkeleton(w.skeleton)
		}
	} else {
		content = w.book.Content
	}

	// 2. Set up flows if enabled
	if w.options.SupportFlows {
		// Create primary HTML flow
		w.flowManager.CreateFlow("primary", FlowTypeHTML, content)

		// Create CSS flow if styles exist
		// TODO: Extract and add CSS from book

		// Add resources to flows
		w.addResourcesToFlows()

		// Convert links to Kindle format
		w.flowManager.ConvertLinks()

		// Use primary flow content
		primaryFlow, _ := w.flowManager.GetPrimaryFlow()
		content = primaryFlow.Content
	}

	// 3. Update book content
	w.book.Content = content

	// 4. Update MOBI header for KF8
	w.setupKF8Header()

	// 5. Write MOBI (with KF8 extensions)
	if err := w.mobiWriter.Write(output); err != nil {
		return fmt.Errorf("failed to write MOBI: %w", err)
	}

	return nil
}

// setupKF8Header configures the MOBI header for KF8
func (w *KF8Writer) setupKF8Header() {
	// This would update the MOBI header with KF8-specific values
	// In a full implementation, would:
	// - Set MOBI version to 8
	// - Add KF8-specific flags
	// - Set up additional indices
}

// addResourcesToFlows adds manifest resources to flows
func (w *KF8Writer) addResourcesToFlows() {
	// Iterate through book manifest
	ids := w.book.GetManifestIDs()
	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}

		// Determine resource type
		resType := ParseResourceType(res.Href)
		flowID := getFlowForResourceType(resType)

		// Add to appropriate flow
		if flowID != "" {
			w.flowManager.AddResourceToFlow(flowID, id, res.Href)
		}
	}
}

// getFlowForResourceType returns the flow ID for a resource type
func getFlowForResourceType(resType string) string {
	switch resType {
	case "css":
		return "styles"
	case "svg":
		return "svg"
	case "font":
		return "fonts"
	default:
		return "" // No specific flow
	}
}

// GetSkeleton returns the skeleton structure
func (w *KF8Writer) GetSkeleton() *Skeleton {
	return w.skeleton
}

// GetFlowManager returns the flow manager
func (w *KF8Writer) GetFlowManager() *FlowManager {
	return w.flowManager
}

// GetFDST returns the FDST structure
func (w *KF8Writer) GetFDST() *FDST {
	return w.fdst
}

// ConvertOEBToKF8 is a convenience function to convert OPFBook to KF8
func ConvertOEBToKF8(book *opf.OEBBook, output io.Writer) error {
	writer := NewKF8Writer(book)
	return writer.Write(output)
}

// ConvertOEBToKF8WithOptions converts OPFBook to KF8 with options
func ConvertOEBToKF8WithOptions(book *opf.OEBBook, output io.Writer, options KF8WriteOptions) error {
	writer := NewKF8Writer(book)
	writer.SetOptions(options)
	return writer.Write(output)
}

// WriteJointFile writes a joint MOBI file (MOBI 6 + KF8)
// For now, we create pure KF8 like Calibre (smaller, works better)
func (w *KF8Writer) WriteJointFile(output io.Writer) error {
	// Save original content
	originalContent := w.book.Content

	// Create a single PalmDB writer for the joint file
	palmWriter := mobi.NewPalmDBWriter(w.mobiWriter.GetBookName(), false)

	recordIndex := 0

	// === KF8 SECTION (like Calibre - no MOBI 6 section) ===

	// 1. Prepare KF8 content (with chunking)
	var kf8Content string
	if w.options.EnableChunking {
		if err := w.skeleton.ChunkHTML(originalContent); err != nil {
			return fmt.Errorf("failed to chunk HTML: %w", err)
		}
		w.skeleton.BuildHierarchy()
		kf8Content = w.skeleton.AssignAIDAttributes()

		// Generate FDST from skeleton
		if w.options.GenerateFDST {
			w.fdst.GenerateFromSkeleton(w.skeleton)
		}
	} else {
		kf8Content = originalContent
	}

	// 2. Add KF8 text records FIRST (before images)
	// Use PalmDOC compression like Calibre
	kf8TextData := mobi.CompressPalmDOC([]byte(kf8Content))

	// Remember first text record index (will be record 1 after prepending header)
	firstTextRec := recordIndex

	kf8TextRecords := w.splitTextRecords(kf8TextData)
	for _, rec := range kf8TextRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex))
		recordIndex++
	}

	lastTextRec := recordIndex - 1

	// 3. Add images AFTER text
	ids := w.book.GetManifestIDs()
	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}
		if len(res.MediaType) >= 6 && res.MediaType[0:5] == "image" {
			palmWriter.AddRecord(res.Data, 0, uint32(recordIndex))
			recordIndex++
		}
	}

	// 4. Add KF8-specific indices (FDST, skeleton, etc.)
	if w.options.GenerateFDST && w.fdst != nil {
		var fdstBuf bytes.Buffer
		if err := w.fdst.Write(&fdstBuf); err == nil {
			palmWriter.AddRecord(fdstBuf.Bytes(), 0, uint32(recordIndex))
			recordIndex++
		}
	}

	// === HEADER (written last, like Calibre) ===
	// Create MOBI 6 header with KF8 flag (RecordSize=0x10000000)
	// This tells readers to expect KF8 content

	mobiHeader := mobi.NewMOBIHeader(len(kf8Content),
		mobi.CalculateRecordCount(len(kf8Content)))
	mobiHeader.SetFullName(w.mobiWriter.GetBookName())
	// Signal KF8 through MOBIType instead of RecordSize
	// RecordSize field is uint16, can't hold 0x10000000
	mobiHeader.MOBIType = 248  // 248 = KF8
	mobiHeader.FileVersion = 8 // KF8 format version

	// Use PalmDOC compression like Calibre
	mobiHeader.Compression = mobi.PalmDOCCompression // 2 = PalmDOC compression

	// Adjust firstTextRec/lastTextRec for prepended header (+1 offset)
	mobiHeader.SetContentRecords(uint16(firstTextRec+1), uint16(lastTextRec+1))

	// Create EXTH header with metadata (like Calibre)
	exthWriter := mobi.NewEXTHWriter()
	authors := make([]string, 0)
	for _, author := range w.book.Metadata.Authors {
		authors = append(authors, author.FullName)
	}
	exthWriter.AddFromMetadata(
		w.book.Metadata.Title,
		strings.Join(authors, ", "),
		w.book.Metadata.Publisher,
		w.book.Metadata.ISBN,
		w.book.Metadata.Year,
		w.book.Metadata.Annotation,
		w.book.Metadata.Rights,
		w.book.Metadata.Language,
	)

	// Set EXTH flag BEFORE writing header
	mobiHeader.SetEXTHFlags(0x50) // Has EXTH header (like mobi writer)

	// Encode MOBI header
	var headerBuf bytes.Buffer
	if err := mobiHeader.Write(&headerBuf); err != nil {
		return fmt.Errorf("failed to write MOBI header: %w", err)
	}

	// Write EXTH after MOBI header
	exthData := bytes.NewBuffer(nil)
	if _, err := exthWriter.Write(exthData); err != nil {
		return fmt.Errorf("failed to write EXTH: %w", err)
	}
	headerBuf.Write(exthData.Bytes())

	// Get all records and prepend header
	allRecords := palmWriter.GetRecords()
	allRecordEntries := palmWriter.GetRecordEntries()

	// Create new record list with header at the beginning
	newRecords := make([][]byte, 0, len(allRecords)+1)
	newRecordEntries := make([]mobi.RecordIndexEntry, 0, len(allRecordEntries)+1)

	// Add header as record 0
	newRecords = append(newRecords, headerBuf.Bytes())
	newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
		Attributes: 0,
		UniqueID:   0,
	})

	// Add all other records with adjusted unique IDs
	for i, rec := range allRecords {
		newRecords = append(newRecords, rec)
		newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
			Attributes: allRecordEntries[i].Attributes,
			UniqueID:   uint32(i + 1),
		})
	}

	// Set the records back to PalmDBWriter
	palmWriter.SetRecords(newRecords, newRecordEntries)

	// Write the complete PalmDB
	if err := palmWriter.Write(output); err != nil {
		return fmt.Errorf("failed to write PalmDB: %w", err)
	}

	return nil
}

// splitTextRecords splits text into 4KB records
func (w *KF8Writer) splitTextRecords(data []byte) [][]byte {
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

// GenerateResourceLinks generates Kindle resource links for all manifest resources
func (w *KF8Writer) GenerateResourceLinks() map[string]string {
	links := make(map[string]string)

	ids := w.book.GetManifestIDs()
	for _, id := range ids {
		res, ok := w.book.GetResource(id)
		if !ok {
			continue
		}

		resType := ParseResourceType(res.Href)
		links[id] = GenerateResourceLinks(id, resType)
	}

	return links
}
