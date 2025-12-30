package index

import (
	"testing"
)

// TestNewINDX tests INDX creation
func TestNewINDX(t *testing.T) {
	indx := NewINDX(0)

	if indx.Header.HeaderLength != INDXHeaderSize {
		t.Errorf("HeaderLength = %d, want %d", indx.Header.HeaderLength, INDXHeaderSize)
	}

	if indx.Header.IndexType != 0 {
		t.Errorf("IndexType = %d, want 0", indx.Header.IndexType)
	}

	if indx.Header.Encoding != 65001 {
		t.Errorf("Encoding = %d, want 65001 (UTF-8)", indx.Header.Encoding)
	}

	if indx.TAGX == nil {
		t.Error("TAGX should not be nil")
	}

	if indx.IDXT == nil {
		t.Error("IDXT should not be nil")
	}

	if indx.CNCX == nil {
		t.Error("CNCX should not be nil")
	}
}

// TestTAGXAddTag tests TAGX tag addition
func TestTAGXAddTag(t *testing.T) {
	tagx := NewTAGX()

	tagx.AddTag(1, 1, 0x01)
	tagx.AddTag(2, 1, 0x02)

	if len(tagx.Entries) != 2 {
		t.Fatalf("Entries count = %d, want 2", len(tagx.Entries))
	}

	if tagx.Entries[0].TagID != 1 {
		t.Errorf("First entry TagID = %d, want 1", tagx.Entries[0].TagID)
	}

	if tagx.Entries[1].TagID != 2 {
		t.Errorf("Second entry TagID = %d, want 2", tagx.Entries[1].TagID)
	}
}

// TestTAGXEncode tests TAGX encoding
func TestTAGXEncode(t *testing.T) {
	tagx := NewTAGX()
	tagx.AddTag(1, 1, 0x01)
	tagx.AddTag(2, 1, 0x02)

	data, err := tagx.Encode()
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	// TAGX: 4 bytes header + N * 4 bytes entries
	expectedLen := 4 + 2*4
	if len(data) != expectedLen {
		t.Errorf("Encoded length = %d, want %d", len(data), expectedLen)
	}

	// First 4 bytes should be number of tags (2)
	if data[0] != 0 || data[1] != 0 || data[2] != 0 || data[3] != 2 {
		t.Errorf("Header = %v, want [0, 0, 0, 2]", data[:4])
	}
}

// TestINDXAddString tests CNCX string addition
func TestINDXAddString(t *testing.T) {
	indx := NewINDX(0)

	idx1 := indx.AddString("Chapter 1")
	if idx1 != 0 {
		t.Errorf("First string index = %d, want 0", idx1)
	}

	idx2 := indx.AddString("Chapter 2")
	if idx2 != 1 {
		t.Errorf("Second string index = %d, want 1", idx2)
	}

	if len(indx.CNCX) != 2 {
		t.Errorf("CNCX length = %d, want 2", len(indx.CNCX))
	}
}

// TestINDXAddEntry tests IDXT entry addition
func TestINDXAddEntry(t *testing.T) {
	indx := NewINDX(0)

	tagValues := map[uint32][]uint32{
		1: {100},
		2: {200},
	}

	indx.AddEntry(1000, 0, tagValues)

	if indx.Header.RecordCount != 1 {
		t.Errorf("RecordCount = %d, want 1", indx.Header.RecordCount)
	}

	if len(indx.IDXT) != 1 {
		t.Errorf("IDXT length = %d, want 1", len(indx.IDXT))
	}

	entry := indx.IDXT[0]
	if entry.Offset != 1000 {
		t.Errorf("Entry offset = %d, want 1000", entry.Offset)
	}

	if entry.RecordIndex != 0 {
		t.Errorf("Entry RecordIndex = %d, want 0", entry.RecordIndex)
	}
}

// TestTOCIndexBuilder tests TOC index builder
func TestTOCIndexBuilder(t *testing.T) {
	builder := NewTOCIndexBuilder()

	// Add some entries
	builder.AddEntry("Chapter 1", "#ch1", 1, 0)
	builder.AddEntry("Chapter 2", "#ch2", 1, 100)
	builder.AddEntry("Section 2.1", "#ch2-1", 2, 150)

	entries := builder.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("Entries count = %d, want 3", len(entries))
	}

	// Check parent indices
	if entries[0].ParentIndex != -1 {
		t.Errorf("Entry 0 ParentIndex = %d, want -1", entries[0].ParentIndex)
	}

	if entries[1].ParentIndex != -1 {
		t.Errorf("Entry 1 ParentIndex = %d, want -1", entries[1].ParentIndex)
	}

	if entries[2].ParentIndex != 1 {
		t.Errorf("Entry 2 ParentIndex = %d, want 1", entries[2].ParentIndex)
	}
}

// TestCalculateRecordOffset tests record offset calculation
func TestCalculateRecordOffset(t *testing.T) {
	builder := NewTOCIndexBuilder()

	// Set up text records (4 records of varying sizes)
	records := [][]byte{
		make([]byte, 100),  // Record 0: 0-99
		make([]byte, 200),  // Record 1: 100-299
		make([]byte, 150),  // Record 2: 300-449
		make([]byte, 300),  // Record 3: 450-749
	}
	builder.SetTextRecords(records)

	tests := []struct {
		offset           uint32
		wantRecordIndex  int
		wantRecordOffset uint32
	}{
		{0, 0, 0},       // Start of first record
		{50, 0, 50},     // Middle of first record
		{99, 0, 99},     // Last byte of first record
		{100, 1, 0},     // Start of second record
		{200, 1, 100},   // Middle of second record
		{299, 1, 199},   // Last byte of second record
		{300, 2, 0},     // Start of third record
		{400, 2, 100},   // Middle of third record
		{449, 2, 149},   // Last byte of third record
		{450, 3, 0},     // Start of fourth record
		{600, 3, 150},   // Middle of fourth record
		{749, 3, 299},   // Last byte of fourth record
		{1000, 3, 250},  // Beyond all records (offset - total length = 1000 - 750)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			recordIndex, recordOffset := builder.CalculateRecordOffset(tt.offset)
			if recordIndex != tt.wantRecordIndex {
				t.Errorf("CalculateRecordOffset(%d) recordIndex = %d, want %d",
					tt.offset, recordIndex, tt.wantRecordIndex)
			}
			if recordOffset != tt.wantRecordOffset {
				t.Errorf("CalculateRecordOffset(%d) recordOffset = %d, want %d",
					tt.offset, recordOffset, tt.wantRecordOffset)
			}
		})
	}
}

// TestCalculateRecordOffsetEmpty tests record offset with no records
func TestCalculateRecordOffsetEmpty(t *testing.T) {
	builder := NewTOCIndexBuilder()

	recordIndex, recordOffset := builder.CalculateRecordOffset(100)

	if recordIndex != 0 {
		t.Errorf("RecordIndex = %d, want 0", recordIndex)
	}

	if recordOffset != 0 {
		t.Errorf("RecordOffset = %d, want 0", recordOffset)
	}
}

// TestTOCIndexBuilderBuild tests TOC INDX building
func TestTOCIndexBuilderBuild(t *testing.T) {
	builder := NewTOCIndexBuilder()

	// Set up text records
	records := [][]byte{
		make([]byte, 1000),
		make([]byte, 1000),
	}
	builder.SetTextRecords(records)

	// Add TOC entries
	builder.AddEntry("Chapter 1", "#ch1", 1, 0)
	builder.AddEntry("Chapter 2", "#ch2", 1, 500)

	// Build INDX
	indx, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify TAGX entries (4 tags: name, offset, level, parent)
	if len(indx.TAGX.Entries) != 4 {
		t.Errorf("TAGX entries count = %d, want 4", len(indx.TAGX.Entries))
	}

	// Verify IDXT entries
	if len(indx.IDXT) != 2 {
		t.Errorf("IDXT entries count = %d, want 2", len(indx.IDXT))
	}

	// Verify CNCX strings
	if len(indx.CNCX) != 2 {
		t.Errorf("CNCX strings count = %d, want 2", len(indx.CNCX))
	}

	if indx.CNCX[0] != "Chapter 1" {
		t.Errorf("CNCX[0] = %s, want 'Chapter 1'", indx.CNCX[0])
	}

	if indx.CNCX[1] != "Chapter 2" {
		t.Errorf("CNCX[1] = %s, want 'Chapter 2'", indx.CNCX[1])
	}
}

// TestSortTOC tests TOC sorting
func TestSortTOC(t *testing.T) {
	entries := []TOCEntry{
		{Label: "Chapter 3", Offset: 300},
		{Label: "Chapter 1", Offset: 100},
		{Label: "Chapter 2", Offset: 200},
	}

	SortTOC(entries)

	if entries[0].Offset != 100 {
		t.Errorf("After sort, entries[0].Offset = %d, want 100", entries[0].Offset)
	}

	if entries[1].Offset != 200 {
		t.Errorf("After sort, entries[1].Offset = %d, want 200", entries[1].Offset)
	}

	if entries[2].Offset != 300 {
		t.Errorf("After sort, entries[2].Offset = %d, want 300", entries[2].Offset)
	}
}

// TestFindOffsetForHref tests finding href offsets in HTML
func TestFindOffsetForHref(t *testing.T) {
	builder := NewTOCIndexBuilder()

	html := `<html>
<body>
	<h1 id="title">Book Title</h1>
	<p>Some text</p>
	<h2 id="chapter1">Chapter 1</h2>
	<p>More text</p>
	<h2 id="chapter2">Chapter 2</h2>
	<p>Even more text</p>
	<a name="anchor1">Anchor</a>
</body>
</html>`

	tests := []struct {
		href        string
		wantOffset  uint32
		description string
	}{
		{"#title", 15, "ID attribute"},
		{"#chapter1", 65, "First chapter"},
		{"#chapter2", 117, "Second chapter"},
		{"#anchor1", 174, "Name attribute"},
		{"#nonexistent", 0, "Non-existent ID (should return 0)"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			offset := builder.FindOffsetForHref(html, tt.href)
			if offset != tt.wantOffset {
				t.Errorf("FindOffsetForHref(%q) = %d, want %d", tt.href, offset, tt.wantOffset)
			}
		})
	}
}

// TestCalculateOffsetsFromHTML tests automatic offset calculation
func TestCalculateOffsetsFromHTML(t *testing.T) {
	builder := NewTOCIndexBuilder()

	// Add some entries first
	builder.AddEntry("Chapter 1", "#ch1", 1, 0)
	builder.AddEntry("Chapter 2", "#ch2", 1, 0)

	html := `<html>
<body>
	<p>Intro</p>
	<h2 id="ch1">Chapter 1</h2>
	<p>Content 1</p>
	<h2 id="ch2">Chapter 2</h2>
	<p>Content 2</p>
</body>
</html>`

	err := builder.CalculateOffsetsFromHTML(html)
	if err != nil {
		t.Fatalf("CalculateOffsetsFromHTML() failed: %v", err)
	}

	entries := builder.GetEntries()

	// Should have at least 2 entries (the ones we added)
	if len(entries) < 2 {
		t.Fatalf("Expected at least 2 entries, got %d", len(entries))
	}

	// Check that offsets were updated
	// Entry 0 should be "Chapter 1" with offset somewhere in the HTML
	if entries[0].Label != "Chapter 1" {
		t.Errorf("entries[0].Label = %s, want 'Chapter 1'", entries[0].Label)
	}

	if entries[0].Offset == 0 {
		t.Errorf("entries[0].Offset was not updated (still 0)")
	}

	// Entry 1 should be "Chapter 2" with offset > Chapter 1's offset
	if entries[1].Label != "Chapter 2" {
		t.Errorf("entries[1].Label = %s, want 'Chapter 2'", entries[1].Label)
	}

	if entries[1].Offset <= entries[0].Offset {
		t.Errorf("entries[1].Offset = %d, should be > entries[0].Offset = %d",
			entries[1].Offset, entries[0].Offset)
	}
}
