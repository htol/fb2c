package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createMinimalMOBI creates a minimal valid MOBI file for testing
func createMinimalMOBI() []byte {
	var buf bytes.Buffer

	// PalmDB header structure:
	// 0-31: Database name (32 bytes)
	// 32-33: Attributes (2 bytes)
	// 34-35: Version (2 bytes)
	// 36-43: Creation date (8 bytes)
	// 44-51: Modification date (8 bytes)
	// 52-59: Last backup date (8 bytes)
	// 60-63: Modification number (4 bytes)
	// 64-67: App info ID (4 bytes)
	// 68-71: Sort info ID (4 bytes)
	// 72-75: Type (4 bytes) - "BOOK"
	// 76-79: Creator (4 bytes) - "MOBI"

	// Write database name (32 bytes)
	name := "Test Book"
	buf.Write([]byte(name))
	for i := len(name); i < 32; i++ {
		buf.WriteByte(0)
	}

	// Write filler to get to offset 72 (Type field)
	for i := 32; i < 72; i++ {
		buf.WriteByte(0)
	}

	// Type: "BOOK" at offset 72
	buf.Write([]byte("BOOK"))

	// Creator: "MOBI" at offset 76
	buf.Write([]byte("MOBI"))

	// Rest of PalmDB header to offset 80
	for i := 80; i < 80; i++ {
		buf.WriteByte(0)
	}

	// MOBI header starts after PalmDB header (offset 80+)
	buf.Write([]byte("MOBI"))                      // Magic

	// Header length (4 bytes)
	headerLen := make([]byte, 4)
	binary.BigEndian.PutUint32(headerLen, 232)
	buf.Write(headerLen)

	// MOBI version (4 bytes)
	version := make([]byte, 4)
	binary.BigEndian.PutUint32(version, 6)
	buf.Write(version)

	// MOBI flags (4 bytes)
	buf.Write([]byte{0, 0, 0, 0})

	// Padding to offset 28 where encoding goes
	for i := 16; i < 28; i++ {
		buf.WriteByte(0)
	}

	// Encoding: 65001 (UTF-8) at offset 28 from MOBI header start
	encoding := make([]byte, 4)
	binary.BigEndian.PutUint32(encoding, 65001)
	buf.Write(encoding)

	// Rest of MOBI header to 232 bytes
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
	buf.Write([]byte("EXTH"))                      // Magic
	exthLength := make([]byte, 4)
	binary.BigEndian.PutUint32(exthLength, 100)    // Length (including header)
	buf.Write(exthLength)
	buf.Write([]byte{0, 0, 0, 1})                 // Record count

	// Author record (100)
	author := "Test Author"
	buf.Write([]byte{0, 0, 0, 100})               // Record type
	authorLen := make([]byte, 4)
	binary.BigEndian.PutUint32(authorLen, uint32(8+len(author))) // Length
	buf.Write(authorLen)
	buf.WriteString(author)                       // Author name

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
	// Change type from "BOOK" to "TEST" at offset 72
	mobi[72] = 'T'
	mobi[73] = 'E'
	mobi[74] = 'S'
	mobi[75] = 'T'

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
	// Change creator from "MOBI" to "TEST" at offset 76
	mobi[76] = 'T'
	mobi[77] = 'E'
	mobi[78] = 'S'
	mobi[79] = 'T'

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
	// Write valid PalmDB header
	buf.Write(createMinimalMOBI()[:78])
	// Write some junk instead of MOBI header
	buf.Write([]byte("JUNK DATA HERE"))

	validator := NewValidator(buf.Bytes())
	validator.Validate()

	if !validator.HasErrors() {
		t.Error("Should have error for missing MOBI header")
	}

	hasMOBIError := false
	for _, err := range validator.Errors() {
		if err == "MOBI header not found" {
			hasMOBIError = true
			break
		}
	}

	if !hasMOBIError {
		t.Errorf("Expected 'MOBI header not found' error, got: %v", validator.Errors())
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
