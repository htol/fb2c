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
	EnableChunking   bool
	TargetChunkSize  int
	SupportFlows     bool
	GenerateFDST     bool
	KF8Boundary      bool // Insert BOUNDARY marker for joint files
}

// DefaultKF8WriteOptions returns default KF8 write options
func DefaultKF8WriteOptions() KF8WriteOptions {
	return KF8WriteOptions{
		WriteOptions:     mobi.DefaultWriteOptions(),
		EnableChunking:   true,
		TargetChunkSize:  TargetChunkSize,
		SupportFlows:     true,
		GenerateFDST:     true,
		KF8Boundary:      false,
	}
}

// KF8Writer writes KF8 (MOBI 8) files
type KF8Writer struct {
	mobiWriter   *mobi.Writer
	skeleton     *Skeleton
	flowManager  *FlowManager
	fdst         *FDST
	options      KF8WriteOptions
	book         *opf.OEBBook
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
// A joint file is a single PalmDB with both MOBI 6 and KF8 records
func (w *KF8Writer) WriteJointFile(output io.Writer) error {
	// Save original content
	originalContent := w.book.Content

	// Create a single PalmDB writer for the joint file
	palmWriter := mobi.NewPalmDBWriter(w.mobiWriter.GetBookName())

	recordIndex := 0

	// === MOBI 6 SECTION ===

	// 1. Create MOBI 6 header (version 6)
	mobi6Header := mobi.NewMOBIHeader(len(w.book.Content),
		mobi.CalculateRecordCount(len(w.book.Content)))
	mobi6Header.SetFullName(w.mobiWriter.GetBookName())
	mobi6Header.FormatVersion = 6 // MOBI 6

	// 2. Add MOBI 6 text records (uncompressed for MOBI 6)
	textData := []byte(w.book.Content)
	textRecords := w.splitTextRecords(textData)
	for _, rec := range textRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex))
		recordIndex++
	}

	// 3. Add images for MOBI 6
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

	// 4. Add TOC INDX if enabled
	if w.options.GenerateTOC && len(w.book.TOC.Children) > 0 {
		tocINDX, err := w.mobiWriter.GenerateTOCIndex(w.book.Content, textRecords)
		if err == nil {
			indxData, err := tocINDX.Encode()
			if err == nil {
				palmWriter.AddRecord(indxData, 0, uint32(recordIndex))
				recordIndex++
			}
		}
	}

	// === BOUNDARY ===

	// Remember the boundary record index
	boundaryRecordIndex := recordIndex

	// Add a boundary marker (the actual content doesn't matter much, but some tools use "BOUNDARY")
	palmWriter.AddRecord([]byte("BOUNDARY"), 0, uint32(recordIndex))
	recordIndex++

	// === KF8 SECTION ===

	// 5. Prepare KF8 content (with chunking)
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

	// 6. Add KF8 text records
	kf8TextData := []byte(kf8Content)
	kf8TextRecords := w.splitTextRecords(kf8TextData)
	for _, rec := range kf8TextRecords {
		palmWriter.AddRecord(rec, 0, uint32(recordIndex))
		recordIndex++
	}

	// 7. Add KF8-specific indices (FDST, skeleton, etc.)
	if w.options.GenerateFDST && w.fdst != nil {
		var fdstBuf bytes.Buffer
		if err := w.fdst.Write(&fdstBuf); err == nil {
			palmWriter.AddRecord(fdstBuf.Bytes(), 0, uint32(recordIndex))
			recordIndex++
		}
	}

	// === HEADERS (written last so we know all record counts) ===

	// Now create EXTH header with KF8 boundary record (we know the boundary index now)
	exthWriter := mobi.NewEXTHWriter()

	// Add metadata from book
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
	)

	// Add KF8 boundary record
	exthWriter.AddKF8Boundary(uint32(boundaryRecordIndex))

	// Update MOBI 6 header with correct content record indices
	// MOBI 6 text records start at index 1 (after MOBI header at index 0)
	// They go up to boundaryRecordIndex - 1 (before BOUNDARY marker)
	mobi6Header.SetContentRecords(1, uint16(boundaryRecordIndex-1))

	// Encode MOBI 6 header + EXTH
	var mobi6HeaderBuf bytes.Buffer
	if err := mobi6Header.Write(&mobi6HeaderBuf); err != nil {
		return fmt.Errorf("failed to write MOBI 6 header: %w", err)
	}

	if _, err := exthWriter.Write(&mobi6HeaderBuf); err != nil {
		return fmt.Errorf("failed to write EXTH: %w", err)
	}

	// 8. Create KF8 header (version 8)
	mobi8Header := mobi.NewMOBIHeader(len(kf8Content),
		mobi.CalculateRecordCount(len(kf8Content)))
	mobi8Header.SetFullName(w.mobiWriter.GetBookName())
	mobi8Header.FormatVersion = 8 // KF8

	// KF8 records start after boundary (boundaryRecordIndex) + KF8 header position
	// The KF8 header will be inserted at boundaryRecordIndex + 1
	// So first KF8 content record is at boundaryRecordIndex + 2
	kf8FirstContentRec := boundaryRecordIndex + 2
	kf8LastContentRec := kf8FirstContentRec + len(kf8TextRecords) - 1
	mobi8Header.SetContentRecords(uint16(kf8FirstContentRec), uint16(kf8LastContentRec))

	// Encode KF8 header (without EXTH this time)
	var kf8HeaderBuf bytes.Buffer
	if err := mobi8Header.Write(&kf8HeaderBuf); err != nil {
		return fmt.Errorf("failed to write KF8 header: %w", err)
	}

	// Insert headers at the beginning (MOBI 6 at record 0, KF8 at record boundary+1)
	// We need to insert these records at the beginning, but PalmDBWriter doesn't support insertion
	// So we'll rebuild the record list

	// Get all current records
	allRecords := palmWriter.GetRecords()

	// Create new record list with headers in the right places
	newRecords := make([][]byte, 0, len(allRecords)+2)
	newRecordEntries := make([]mobi.RecordIndexEntry, 0, len(allRecords)+2)

	// Insert MOBI 6 header at record 0
	newRecords = append(newRecords, mobi6HeaderBuf.Bytes())
	newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
		Attributes: 0,
		UniqueID:   0,
	})

	// Add all records up to and including boundary
	boundaryEnd := boundaryRecordIndex + 1
	for i := 0; i < boundaryEnd && i < len(allRecords); i++ {
		newRecords = append(newRecords, allRecords[i])
		newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
			Attributes: 0,
			UniqueID:   uint32(i + 1),
		})
	}

	// Insert KF8 header after boundary
	newRecords = append(newRecords, kf8HeaderBuf.Bytes())
	newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
		Attributes: 0,
		UniqueID:   uint32(boundaryEnd + 1),
	})

	// Add remaining records
	for i := boundaryEnd; i < len(allRecords); i++ {
		newRecords = append(newRecords, allRecords[i])
		newRecordEntries = append(newRecordEntries, mobi.RecordIndexEntry{
			Attributes: 0,
			UniqueID:   uint32(i + 2),
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
