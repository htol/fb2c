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
	EXTHCreatorSoftware = 200
	EXTHCoverOffset     = 201
	EXTHThumbOffset     = 202
	EXTHHasFakeCover    = 203
	EXTHK8CoverImage    = 129
	EXTHTitle           = 503
	EXTHMajorMajor      = 501
	EXTHMajorMinor      = 502
	EXTHMinorCount      = 503
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
	w.addRecord(EXTHAuthor, author)
}

// AddTitle adds a title record
func (w *EXTHWriter) AddTitle(title string) {
	w.addRecord(EXTHTitle, title)
}

// AddPublisher adds a publisher record
func (w *EXTHWriter) AddPublisher(publisher string) {
	w.addRecord(EXTHPublisher, publisher)
}

// AddDescription adds a description/annotation record
func (w *EXTHWriter) AddDescription(description string) {
	w.addRecord(EXTHDescription, description)
}

// AddISBN adds an ISBN record
func (w *EXTHWriter) AddISBN(isbn string) {
	w.addRecord(EXTHISBN, isbn)
}

// AddSubject adds a subject/genre record
func (w *EXTHWriter) AddSubject(subject string) {
	w.addStringList(EXTHSubject, []string{subject})
}

// AddPublishedDate adds a publication date record
func (w *EXTHWriter) AddPublishedDate(date string) {
	w.addRecord(EXTHPublishedDate, date)
}

// AddContributor adds a contributor record
func (w *EXTHWriter) AddContributor(contributor string) {
	w.addRecord(EXTHContributor, contributor)
}

// AddRights adds a copyright record
func (w *EXTHWriter) AddRights(rights string) {
	w.addRecord(EXTHRights, rights)
}

// AddASIN adds an Amazon ASIN record
func (w *EXTHWriter) AddASIN(asin string) {
	w.addRecord(EXTHASIN, asin)
}

// AddType adds a type/genre record
func (w *EXTHWriter) AddType(typ string) {
	w.addRecord(EXTHType, typ)
}

// AddSource adds a source record
func (w *EXTHWriter) AddSource(source string) {
	w.addRecord(EXTHSource, source)
}

// AddLanguage adds a language record
func (w *EXTHWriter) AddLanguage(lang string) {
	w.addRecord(EXTHLanguage, lang)
}

// AddCoverOffset adds a cover offset record
func (w *EXTHWriter) AddCoverOffset(offset uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, offset)
	w.addRecord(EXTHCoverOffset, string(data))
}

// AddThumbnailOffset adds a thumbnail offset record
func (w *EXTHWriter) AddThumbnailOffset(offset uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, offset)
	w.addRecord(EXTHThumbOffset, string(data))
}

// AddHasFakeCover adds a has fake cover record
func (w *EXTHWriter) AddHasFakeCover(hasFake uint32) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, hasFake)
	w.addRecord(EXTHHasFakeCover, string(data))
}

// AddK8CoverImage adds a K8 cover image record
func (w *EXTHWriter) AddK8CoverImage(imageID string) {
	w.addRecord(EXTHK8CoverImage, imageID)
}

// AddCreatorSoftware adds a creator software record
func (w *EXTHWriter) AddCreatorSoftware(software string) {
	w.addRecord(EXTHCreatorSoftware, software)
}

// AddReview adds a review record
func (w *EXTHWriter) AddReview(review string) {
	w.addRecord(EXTHReview, review)
}

// AddRetailPrice adds a retail price record
func (w *EXTHWriter) AddRetailPrice(price float32, currency string) {
	// Price is stored as 4-byte float
	priceBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(priceBytes, math.Float32bits(price))

	// Combine with currency
	data := append(priceBytes, []byte(currency)...)
	w.addRecord(EXTHRetailPrice, string(data))
}

// addStringList adds multiple strings as a single record (comma-separated)
func (w *EXTHWriter) AddSubjectList(subjects []string) {
	w.addStringList(EXTHSubject, subjects)
}

// addRecord adds a generic record
func (w *EXTHWriter) addRecord(recordType uint32, data string) {
	w.records = append(w.records, EXTHRecord{
		RecordType: recordType,
		Data:       []byte(data),
	})
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
	w.addRecord(recordType, combined)
}

// Write writes the EXTH header and records
func (w *EXTHWriter) Write(output io.Writer) (int, error) {
	if len(w.records) == 0 {
		return 0, nil
	}

	// Calculate total length
	// Header: 12 bytes (4 identifier + 4 length + 4 count)
	// Each record: 8 bytes overhead (4 type + 4 length) + data length
	totalLength := 12
	for _, record := range w.records {
		totalLength += 8 + len(record.Data)
	}

	// Write header
	header := EXTHHeader{
		Identifier:   [4]byte{'E', 'X', 'T', 'H'},
		HeaderLength: uint32(totalLength),
		RecordCount:  uint32(len(w.records)),
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
		if err := binary.Write(output, binary.BigEndian, uint32(8+len(record.Data))); err != nil {
			return 0, fmt.Errorf("failed to write EXTH record length: %w", err)
		}
		if _, err := output.Write(record.Data); err != nil {
			return 0, fmt.Errorf("failed to write EXTH record data: %w", err)
		}
	}

	return totalLength, nil
}

// GetRecordCount returns the number of records
func (w *EXTHWriter) GetRecordCount() int {
	return len(w.records)
}

// GetTotalLength returns the total EXTH length including header
func (w *EXTHWriter) GetTotalLength() int {
	if len(w.records) == 0 {
		return 0
	}
	totalLength := 12
	for _, record := range w.records {
		totalLength += 8 + len(record.Data)
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
	w.AddCreatorSoftware("fb2c - FB2 to MOBI Converter")
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
