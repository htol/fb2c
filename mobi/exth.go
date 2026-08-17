// Package mobi provides EXTH metadata generation.
package mobi

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// EXTH record type constants
const (
	EXTHAuthor          = 100
	EXTHPublisher       = 101
	EXTHImprint         = 102
	EXTHDescription     = 103
	EXTHISBN            = 104
	EXTHSubject         = 105
	EXTHPublishedDate   = 106
	EXTHReview          = 107
	EXTHContributor     = 108
	EXTHRights          = 109
	EXTHSubjectCode     = 110
	EXTHType            = 501
	EXTHLanguage        = 524
	EXTHSource          = 112
	EXTHASIN            = 113
	EXTHVersion         = 114
	EXTHSample          = 115
	EXTHStartReading    = 116
	EXTHAdultRating     = 117
	EXTHRetailPrice     = 118
	EXTHCurrency        = 119
	EXTHKF8Bounded      = 121
	EXTHResourceCount   = 125
	// 200 = dictionary short name (spec §4); the constant stays for the
	// reader.go dump label. fb2c does not write this record.
	EXTHCreatorSoftware = 200
	EXTHCoverOffset     = 201
	EXTHThumbOffset     = 202
	EXTHHasFakeCover    = 203
	EXTHK8CoverImage    = 129
	EXTHTitle           = 503
	EXTHMajorMajor      = 501
	EXTHMajorMinor      = 502
	EXTHMinorCount      = 503
	// EXTH 204–207 are creator software / major / minor / build (spec §4);
	// fb2c does not write them. The constants stay for reader.go dump labels.
	EXTHNCXOffset     = 204
	EXTHNCXChunkCount = 205
	EXTHNCXFlowCount  = 206
	EXTHNCXTotalSize  = 207
)

// EXTHRecord represents an EXTH metadata record
type EXTHRecord struct {
	RecordType uint32
	Data       []byte
}

// EXTHHeader represents the EXTH header structure
type EXTHHeader struct {
	Identifier   [4]byte // Should be "EXTH"
	HeaderLength uint32
	RecordCount  uint32
}

// EXTHWriter writes EXTH metadata
type EXTHWriter struct {
	records []EXTHRecord
}

// NewEXTHWriter creates a new EXTH writer
func NewEXTHWriter() *EXTHWriter {
	return &EXTHWriter{
		records: make([]EXTHRecord, 0),
	}
}

// AddAuthor adds an author record
func (w *EXTHWriter) AddAuthor(author string) {
	w.addStringRecord(EXTHAuthor, author)
}

// AddTitle adds a title record
func (w *EXTHWriter) AddTitle(title string) {
	w.addStringRecord(EXTHTitle, title)
}

// AddPublisher adds a publisher record
func (w *EXTHWriter) AddPublisher(publisher string) {
	w.addStringRecord(EXTHPublisher, publisher)
}

// AddDescription adds a description/annotation record
func (w *EXTHWriter) AddDescription(description string) {
	w.addStringRecord(EXTHDescription, description)
}

// AddISBN adds an ISBN record
func (w *EXTHWriter) AddISBN(isbn string) {
	w.addStringRecord(EXTHISBN, isbn)
}

// AddSubject adds a subject/genre record
func (w *EXTHWriter) AddSubject(subject string) {
	w.addStringList(EXTHSubject, []string{subject})
}

// AddPublishedDate adds a publication date record
func (w *EXTHWriter) AddPublishedDate(date string) {
	w.addStringRecord(EXTHPublishedDate, date)
}

// AddContributor adds a contributor record
func (w *EXTHWriter) AddContributor(contributor string) {
	w.addStringRecord(EXTHContributor, contributor)
}

// AddRights adds a copyright record
func (w *EXTHWriter) AddRights(rights string) {
	w.addStringRecord(EXTHRights, rights)
}

// AddASIN adds an Amazon ASIN record
func (w *EXTHWriter) AddASIN(asin string) {
	w.addStringRecord(EXTHASIN, asin)
}

// AddType adds a type/genre record
func (w *EXTHWriter) AddType(typ string) {
	w.addStringRecord(EXTHType, typ)
}

// AddSource adds a source record
func (w *EXTHWriter) AddSource(source string) {
	w.addStringRecord(EXTHSource, source)
}

// AddLanguage adds a language record
func (w *EXTHWriter) AddLanguage(lang string) {
	w.addStringRecord(EXTHLanguage, lang)
}

// AddCoverOffset adds a cover offset record
func (w *EXTHWriter) AddCoverOffset(offset uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, offset)
	w.addRecord(EXTHCoverOffset, data)
}

// AddThumbnailOffset adds a thumbnail offset record
func (w *EXTHWriter) AddThumbnailOffset(offset uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, offset)
	w.addRecord(EXTHThumbOffset, data)
}

// AddHasFakeCover adds a has fake cover record
func (w *EXTHWriter) AddHasFakeCover(hasFake uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, hasFake)
	w.addRecord(EXTHHasFakeCover, data)
}

// AddK8CoverImage adds a K8 cover image record
func (w *EXTHWriter) AddK8CoverImage(imageID string) {
	w.addStringRecord(EXTHK8CoverImage, imageID)
}

// AddReview adds a review record
func (w *EXTHWriter) AddReview(review string) {
	w.addStringRecord(EXTHReview, review)
}

// AddRetailPrice adds a retail price record
func (w *EXTHWriter) AddRetailPrice(price float32, currency string) {
	// Price is stored as 4-byte float
	priceBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(priceBytes, math.Float32bits(price))

	// Combine with currency
	data := append(priceBytes, []byte(currency)...)
	w.addRecord(EXTHRetailPrice, data)
}

// addStringList adds multiple strings as a single record (comma-separated)
func (w *EXTHWriter) AddSubjectList(subjects []string) {
	w.addStringList(EXTHSubject, subjects)
}

// addRecord adds a record with raw bytes
func (w *EXTHWriter) addRecord(recordType uint32, data []byte) {
	w.records = append(w.records, EXTHRecord{
		RecordType: recordType,
		Data:       data,
	})
}

// addStringRecord adds a record from a string value
func (w *EXTHWriter) addStringRecord(recordType uint32, data string) {
	w.addRecord(recordType, []byte(data))
}

// addStringList adds multiple strings as comma-separated values
func (w *EXTHWriter) addStringList(recordType uint32, strings []string) {
	combined := ""
	for i, s := range strings {
		if i > 0 {
			combined += ", "
		}
		combined += s
	}
	w.addStringRecord(recordType, combined)
}

// AddStartReading adds a start reading record
func (w *EXTHWriter) AddStartReading(offset uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, offset)
	w.addRecord(EXTHStartReading, data)
}

// Write writes the EXTH header and records.
//
// HeaderLength (spec §4) includes the 12-byte header and all records but
// EXCLUDES the final padding; the returned byte count INCLUDES the padding,
// because FullNameOffset must point past the padded EXTH (spec §5).
func (w *EXTHWriter) Write(output io.Writer) (int, error) {
	if len(w.records) == 0 {
		return 0, nil
	}

	// Pure length: 12-byte header (identifier + length + count) plus each
	// record's 8 bytes of overhead (type + length) and its data.
	pureLength := 12
	for _, record := range w.records {
		pureLength += 8 + len(record.Data)
	}

	// Write header
	header := EXTHHeader{
		Identifier:   [4]byte{'E', 'X', 'T', 'H'},
		HeaderLength: uint32(pureLength),    //nolint:gosec // Length fits
		RecordCount:  uint32(len(w.records)), //nolint:gosec // Count fits
	}

	if err := binary.Write(output, binary.BigEndian, header.Identifier); err != nil {
		return 0, fmt.Errorf("failed to write EXTH identifier: %w", err)
	}
	if err := binary.Write(output, binary.BigEndian, header.HeaderLength); err != nil {
		return 0, fmt.Errorf("failed to write EXTH length: %w", err)
	}
	if err := binary.Write(output, binary.BigEndian, header.RecordCount); err != nil {
		return 0, fmt.Errorf("failed to write EXTH record count: %w", err)
	}

	// Write records
	for _, record := range w.records {
		if err := binary.Write(output, binary.BigEndian, record.RecordType); err != nil {
			return 0, fmt.Errorf("failed to write EXTH record type: %w", err)
		}
		// Record length includes the 8 bytes for type and length fields, plus data
		if err := binary.Write(output, binary.BigEndian, uint32(8+len(record.Data))); err != nil { //nolint:gosec // Length fits
			return 0, fmt.Errorf("failed to write EXTH record length: %w", err)
		}
		if _, err := output.Write(record.Data); err != nil {
			return 0, fmt.Errorf("failed to write EXTH record data: %w", err)
		}
	}

	// Pad to a 4-byte boundary after the records; the padding is not counted
	// in HeaderLength.
	padBytes := 0
	if pureLength%4 != 0 {
		padBytes = 4 - (pureLength % 4)
		if _, err := output.Write(make([]byte, padBytes)); err != nil {
			return 0, fmt.Errorf("failed to write EXTH padding: %w", err)
		}
	}

	return pureLength + padBytes, nil
}

// GetRecordCount returns the number of records
func (w *EXTHWriter) GetRecordCount() int {
	return len(w.records)
}

// GetTotalLength returns the total EXTH byte count on disk, i.e. the pure
// length plus alignment padding. Used for FullNameOffset, which must point
// past the padded EXTH (spec §5).
func (w *EXTHWriter) GetTotalLength() int {
	if len(w.records) == 0 {
		return 0
	}
	totalLength := 12
	for _, record := range w.records {
		totalLength += 8 + len(record.Data)
	}

	// Add padding to ensure 4-byte alignment
	padding := totalLength % 4
	if padding != 0 {
		totalLength += 4 - padding
	}

	return totalLength
}

// AddFromMetadata adds common metadata fields
func (w *EXTHWriter) AddFromMetadata(title, author, publisher, isbn, year, description, copyright, language string) {
	w.AddTitle(title)
	w.AddAuthor(author)
	w.AddPublisher(publisher)
	w.AddDescription(description)
	w.AddISBN(isbn)
	w.AddPublishedDate(year)
	w.AddRights(copyright)
	if language != "" {
		w.AddLanguage(language)
	}
}

// AddKF8Boundary adds the KF8 boundary record (type 121)
// This record contains the record index where KF8 content starts
func (w *EXTHWriter) AddKF8Boundary(boundaryRecordIndex uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, boundaryRecordIndex)
	w.records = append(w.records, EXTHRecord{
		RecordType: EXTHKF8Bounded,
		Data:       data,
	})
}

// AddUint32Record adds a generic uint32 record
func (w *EXTHWriter) AddUint32Record(recordType, value uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, value)
	w.records = append(w.records, EXTHRecord{
		RecordType: recordType,
		Data:       data,
	})
}
