package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Validator validates MOBI file structure
type Validator struct {
	data     []byte
	errors   []string
	warnings []string
}

// NewValidator creates a new MOBI validator
func NewValidator(data []byte) *Validator {
	return &Validator{
		data:     data,
		errors:   make([]string, 0),
		warnings: make([]string, 0),
	}
}

// Validate performs all validation checks
func (v *Validator) Validate() bool {
	v.errors = make([]string, 0)
	v.warnings = make([]string, 0)

	if len(v.data) < 78 {
		v.addError("File too short to be a valid MOBI")
		return false
	}

	v.validatePalmDBHeader()
	v.validateMOBIHeader()
	v.validateEXTH()

	return len(v.errors) == 0
}

// validatePalmDBHeader validates PalmDB header
func (v *Validator) validatePalmDBHeader() {
	// Check database name (bytes 0-31)
	nameBytes := bytes.TrimRight(v.data[0:32], "\x00")
	name := string(nameBytes)
	if name == "" {
		v.addWarning("Empty database name")
	}

	// The actual PalmDB header has varying offsets based on implementation
	// We'll search for "BOOK" and "MOBI" in the expected range
	// Typically: name(32) + attrs(2) + version(2) + dates(12) + modnum(4) + appInfo(4) + sortInfo(4) = 60
	// But implementations vary

	// Try to find "BOOK" type in the first 100 bytes
	typeOffset := bytes.Index(v.data[:100], []byte("BOOK"))
	if typeOffset == -1 {
		v.addError("Could not find file type 'BOOK'")
		return
	}

	// Creator should be 4 bytes after type
	if typeOffset+8 > len(v.data) {
		v.addError("File too short for creator check")
		return
	}

	creator := string(v.data[typeOffset+4 : typeOffset+8])
	if creator != MOBIIdentifier {
		v.addError(fmt.Sprintf("Invalid creator: %s (expected 'MOBI')", creator))
	}
}

// validateMOBIHeader validates MOBI header
func (v *Validator) validateMOBIHeader() {
	mobiOffset, err := v.findMOBIHeaderOffset()
	if err != nil {
		v.addError(err.Error())
		return
	}

	// Check MOBI header length (offset + 4)
	if len(v.data) < mobiOffset+4 {
		v.addError("File too short for MOBI header length")
		return
	}

	headerLength := binary.BigEndian.Uint32(v.data[mobiOffset+4 : mobiOffset+8])
	if headerLength < 232 {
		v.addError(fmt.Sprintf("Invalid MOBI header length: %d (should be >= 232)", headerLength))
	}

	// Check MOBI version (offset + 8)
	if len(v.data) < mobiOffset+12 {
		v.addError("File too short for MOBI version")
		return
	}

	mobiVersion := binary.BigEndian.Uint32(v.data[mobiOffset+8 : mobiOffset+12])
	if mobiVersion < 2 || mobiVersion > 8 {
		v.addWarning(fmt.Sprintf("Unusual MOBI version: %d (expected 2-8)", mobiVersion))
	}

	// Check encoding (offset + 28, should be 65001 for UTF-8)
	if len(v.data) < mobiOffset+32 {
		v.addError("File too short for encoding check")
		return
	}

	encoding := binary.BigEndian.Uint32(v.data[mobiOffset+28 : mobiOffset+32])
	if encoding != 65001 {
		v.addWarning(fmt.Sprintf("Encoding is not UTF-8: %d (expected 65001)", encoding))
	}
}

func (v *Validator) findMOBIHeaderOffset() (int, error) {
	searchStart := 78
	if len(v.data) <= searchStart {
		return 0, fmt.Errorf("File too short to contain MOBI header")
	}

	mobiOffset := bytes.Index(v.data[searchStart:], []byte(MOBIIdentifier))
	if mobiOffset == -1 {
		return 0, fmt.Errorf("MOBI header not found")
	}

	mobiOffset += searchStart

	for mobiOffset < len(v.data) {
		if mobiOffset+12 <= len(v.data) {
			headerLen := binary.BigEndian.Uint32(v.data[mobiOffset+4 : mobiOffset+8])
			version := binary.BigEndian.Uint32(v.data[mobiOffset+8 : mobiOffset+12])
			if headerLen >= 232 && version >= 2 && version <= 8 {
				return mobiOffset, nil
			}
		}

		next := bytes.Index(v.data[mobiOffset+4:], []byte(MOBIIdentifier))
		if next == -1 {
			return 0, fmt.Errorf("MOBI header not found")
		}
		mobiOffset += 4 + next
	}
	return 0, fmt.Errorf("MOBI header not found")
}

// validateEXTH validates EXTH header
func (v *Validator) validateEXTH() {
	// Find MOBI header first
	mobiOffset := bytes.Index(v.data, []byte(MOBIIdentifier))
	if mobiOffset == -1 {
		return // Already reported in validateMOBIHeader
	}

	// MOBI header length is at offset + 4
	if len(v.data) < mobiOffset+8 {
		return
	}

	headerLength := binary.BigEndian.Uint32(v.data[mobiOffset+4 : mobiOffset+8])

	// EXTH header starts after MOBI header
	exthOffset := mobiOffset + int(headerLength)
	if len(v.data) < exthOffset+4 {
		return // No EXTH header
	}

	// Check for EXTH magic
	exthMagic := string(v.data[exthOffset : exthOffset+4])
	if exthMagic != EXTHIdentifier {
		v.addWarning("No EXTH header found (metadata may be limited)")
		return
	}

	// EXTH header length (offset + 4)
	if len(v.data) < exthOffset+8 {
		v.addError("File too short for EXTH header length")
		return
	}

	exthLength := binary.BigEndian.Uint32(v.data[exthOffset+4 : exthOffset+8])
	if exthLength < 12 {
		v.addError(fmt.Sprintf("Invalid EXTH header length: %d (should be >= 12)", exthLength))
		return
	}

	// Record count (offset + 8)
	if len(v.data) < exthOffset+12 {
		v.addError("File too short for EXTH record count")
		return
	}

	recordCount := binary.BigEndian.Uint32(v.data[exthOffset+8 : exthOffset+12])
	if recordCount == 0 {
		v.addWarning("EXTH header has no records")
	}

	// Check for essential metadata records
	v.checkEXTHRecords(exthOffset + 12)
}

// checkEXTHRecords checks for essential EXTH records
func (v *Validator) checkEXTHRecords(offset int) {
	// Scan records looking for essential ones
	hasAuthor := false
	hasTitle := false
	hasPublisher := false

	pos := offset
	for pos < len(v.data)-8 {
		recordType := binary.BigEndian.Uint32(v.data[pos : pos+4])
		recordLength := binary.BigEndian.Uint32(v.data[pos+4 : pos+8])

		if recordLength < 8 {
			break // Invalid record
		}

		switch recordType {
		case 100: // Author
			hasAuthor = true
		case 101: // Publisher
			hasPublisher = true
		case 103: // Description
			// Optional but good to have
		case 106: // Published date
			// Optional
		case 503: // Updated title
			hasTitle = true
		}

		if recordLength > uint32(len(v.data)-pos) { //nolint:gosec // Length fits
			break
		}

		pos += int(recordLength)
	}

	if !hasAuthor {
		v.addWarning("EXTH missing author record (100)")
	}
	if !hasTitle {
		v.addWarning("EXTH missing title record (503)")
	}
	if !hasPublisher {
		v.addWarning("EXTH missing publisher record (101)")
	}
}

// addError adds an error
func (v *Validator) addError(msg string) {
	v.errors = append(v.errors, msg)
}

// addWarning adds a warning
func (v *Validator) addWarning(msg string) {
	v.warnings = append(v.warnings, msg)
}

// Errors returns all errors
func (v *Validator) Errors() []string {
	return v.errors
}

// Warnings returns all warnings
func (v *Validator) Warnings() []string {
	return v.warnings
}

// HasErrors returns true if there are errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// HasWarnings returns true if there are warnings
func (v *Validator) HasWarnings() bool {
	return len(v.warnings) > 0
}

// String returns a formatted validation report
func (v *Validator) String() string {
	var buf bytes.Buffer

	buf.WriteString("MOBI Validation Report\n")
	buf.WriteString("=====================\n\n")

	if len(v.errors) == 0 && len(v.warnings) == 0 {
		buf.WriteString("✓ File is valid and Kindle-compatible\n")
		return buf.String()
	}

	if len(v.errors) > 0 {
		buf.WriteString("Errors:\n")
		for _, err := range v.errors {
			buf.WriteString(fmt.Sprintf("  ✗ %s\n", err))
		}
		buf.WriteString("\n")
	}

	if len(v.warnings) > 0 {
		buf.WriteString("Warnings:\n")
		for _, warn := range v.warnings {
			buf.WriteString(fmt.Sprintf("  ⚠ %s\n", warn))
		}
	}

	if len(v.errors) > 0 {
		buf.WriteString("\n✗ File is NOT valid\n")
	} else {
		buf.WriteString("\n✓ File is valid (with warnings)\n")
	}

	return buf.String()
}
