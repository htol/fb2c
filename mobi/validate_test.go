package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createMinimalMOBI creates a minimal valid MOBI file for testing
func createMinimalMOBI() []byte {
	var buf bytes.Buffer

	// Realistic PalmDB layout (spec §1):
	// 0-31 name, 32-59 attributes/version/dates/modnum/appInfo/sortInfo,
	// 60-63 type "BOOK", 64-67 creator "MOBI", 68-71 uniqueID seed,
	// 72-75 nextRecordListID, 76-77 numRecords, 78+ record index (8 bytes/entry).
	name := "Test Book"
	buf.WriteString(name)
	for i := len(name); i < 60; i++ {
		buf.WriteByte(0)
	}
	buf.WriteString("BOOK")
	buf.WriteString("MOBI")
	for i := 68; i < 76; i++ {
		buf.WriteByte(0)
	}
	_ = binary.Write(&buf, binary.BigEndian, uint16(1)) // numRecords

	// Record index entry 0: data at 86 (78 + 8), attributes 0, uniqueID 0
	_ = binary.Write(&buf, binary.BigEndian, uint32(86))
	buf.Write([]byte{0, 0, 0, 0})

	// Record 0: 16-byte PalmDOC header, then the MOBI header
	for i := 0; i < 16; i++ {
		buf.WriteByte(0)
	}
	buf.WriteString("MOBI")                               // Magic
	_ = binary.Write(&buf, binary.BigEndian, uint32(232)) // Header length
	_ = binary.Write(&buf, binary.BigEndian, uint32(6))   // MOBI version
	buf.Write([]byte{0, 0, 0, 0})                         // Flags
	for i := 16; i < 28; i++ {
		buf.WriteByte(0)
	}
	// Encoding: 65001 (UTF-8) at MOBI header offset 28
	_ = binary.Write(&buf, binary.BigEndian, uint32(65001))
	for i := 32; i < 232; i++ {
		buf.WriteByte(0)
	}

	return buf.Bytes()
}

// createMOBIWithEXTH creates a MOBI file with EXTH header
func createMOBIWithEXTH() []byte {
	mobi := createMinimalMOBI()

	// Find MOBI header (search from offset 80 to skip creator field)
	mobiOffset := bytes.Index(mobi[80:], []byte("MOBI"))
	if mobiOffset != -1 {
		mobiOffset += 80 // Convert to absolute offset
	}

	// Add EXTH header after MOBI header
	exthStart := mobiOffset + 232
	var buf bytes.Buffer
	buf.Write(mobi[:exthStart])

	// EXTH header
	buf.Write([]byte("EXTH")) // Magic
	exthLength := make([]byte, 4)
	binary.BigEndian.PutUint32(exthLength, 100) // Length (including header)
	buf.Write(exthLength)
	buf.Write([]byte{0, 0, 0, 1}) // Record count

	// Author record (100)
	author := "Test Author"
	buf.Write([]byte{0, 0, 0, 100}) // Record type
	authorLen := make([]byte, 4)
	binary.BigEndian.PutUint32(authorLen, uint32(8+len(author))) //nolint:gosec // Test data fits
	buf.Write(authorLen)
	buf.WriteString(author) // Author name

	// Padding to 100 bytes
	for buf.Len()-exthStart < 100 {
		buf.WriteByte(0)
	}

	return buf.Bytes()
}

// TestValidateValidMOBI tests validation of a valid MOBI file
func TestValidateValidMOBI(t *testing.T) {
	mobi := createMinimalMOBI()
	validator := NewValidator(mobi)

	if !validator.Validate() {
		t.Errorf("Valid MOBI failed validation:\n%s", validator.String())
	}
}

// TestValidateInvalidType tests detection of invalid file type
func TestValidateInvalidType(t *testing.T) {
	mobi := createMinimalMOBI()
	// Change type from "BOOK" to "TEST" at offset 60
	mobi[60] = 'T'
	mobi[61] = 'E'
	mobi[62] = 'S'
	mobi[63] = 'T'

	validator := NewValidator(mobi)
	validator.Validate()

	if !validator.HasErrors() {
		t.Error("Should have error for invalid type")
	}

	errors := validator.Errors()
	// The validator searches for "BOOK" and reports it's missing when changed to "TEST"
	if len(errors) == 0 || errors[0] != "Could not find file type 'BOOK'" {
		t.Errorf("Wrong error message: %v", errors)
	}
}

// TestValidateInvalidCreator tests detection of invalid creator
func TestValidateInvalidCreator(t *testing.T) {
	mobi := createMinimalMOBI()
	// Change creator from "MOBI" to "TEST" at offset 64
	mobi[64] = 'T'
	mobi[65] = 'E'
	mobi[66] = 'S'
	mobi[67] = 'T'

	validator := NewValidator(mobi)
	validator.Validate()

	if !validator.HasErrors() {
		t.Error("Should have error for invalid creator")
	}
}

// TestValidateShortFile tests handling of files that are too short
func TestValidateShortFile(t *testing.T) {
	shortFile := []byte("TOO SHORT")
	validator := NewValidator(shortFile)

	validator.Validate()

	if !validator.HasErrors() {
		t.Error("Should have error for short file")
	}
}

// TestValidateWithEXTH tests validation with EXTH header
func TestValidateWithEXTH(t *testing.T) {
	mobi := createMOBIWithEXTH()
	validator := NewValidator(mobi)

	if !validator.Validate() {
		t.Errorf("MOBI with EXTH failed validation:\n%s", validator.String())
	}

	// Should have some warnings about missing metadata
	if validator.HasWarnings() {
		t.Logf("Warnings (expected):\n%s", validator.String())
	}
}

// TestValidateMissingMOBIHeader tests detection of missing MOBI header
func TestValidateMissingMOBIHeader(t *testing.T) {
	var buf bytes.Buffer
	// Full PalmDB header + record index, then junk instead of record 0
	buf.Write(createMinimalMOBI()[:86])
	buf.Write([]byte("JUNK DATA HERE"))

	validator := NewValidator(buf.Bytes())
	validator.Validate()

	if !validator.HasErrors() {
		t.Error("Should have error for missing MOBI header")
	}

	hasMOBIError := false
	for _, err := range validator.Errors() {
		if err == "MOBI header not found in record 0" {
			hasMOBIError = true
			break
		}
	}

	if !hasMOBIError {
		t.Errorf("Expected 'MOBI header not found in record 0' error, got: %v", validator.Errors())
	}
}

// TestValidateWrongEncoding tests detection of non-UTF8 encoding
func TestValidateWrongEncoding(t *testing.T) {
	mobi := createMinimalMOBI()
	// Find MOBI header (search from offset 80 to skip creator field)
	mobiOffset := bytes.Index(mobi[80:], []byte("MOBI"))
	if mobiOffset != -1 {
		mobiOffset += 80 // Convert to absolute offset
		// Change encoding to something other than 65001
		mobi[mobiOffset+28] = 0
		mobi[mobiOffset+29] = 0
		mobi[mobiOffset+30] = 0
		mobi[mobiOffset+31] = 0 // 0 instead of 65001
	}

	validator := NewValidator(mobi)
	validator.Validate()

	if !validator.HasWarnings() {
		t.Error("Should have warning for non-UTF8 encoding")
	}
}

// TestValidatorString tests the String() output
func TestValidatorString(t *testing.T) {
	mobi := createMinimalMOBI()
	validator := NewValidator(mobi)
	validator.Validate()

	report := validator.String()
	if report == "" {
		t.Error("String() should not be empty")
	}

	t.Logf("Validation report:\n%s", report)
}
