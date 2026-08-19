package mobi

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/htol/fb2c/opf"
)

// newTestBook returns a minimal OEB book large enough to exercise the reader.
func newTestBook() *opf.OEBBook {
	book := opf.NewOEBBook()
	book.Metadata.Title = "Reader Test"
	book.Metadata.Authors = []opf.Author{{FullName: "Test Author"}}
	book.Content = "<html><body><p>Reader round-trip test</p></body></html>"
	return book
}

func writeTestMOBI(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := ConvertOEBToMOBI(newTestBook(), &buf); err != nil {
		t.Fatalf("ConvertOEBToMOBI failed: %v", err)
	}
	return buf.Bytes()
}

func TestReadDump(t *testing.T) {
	dump, err := ReadDump(writeTestMOBI(t))
	if err != nil {
		t.Fatalf("ReadDump failed: %v", err)
	}

	if dump.PalmDB.Type != "BOOK" || dump.PalmDB.Creator != "MOBI" {
		t.Errorf("PalmDB type/creator = %s/%s, want BOOK/MOBI", dump.PalmDB.Type, dump.PalmDB.Creator)
	}
	if got := len(dump.Records); got != int(dump.PalmDB.NumRecords) {
		t.Errorf("decoded %d records, header claims %d", got, dump.PalmDB.NumRecords)
	}
	if dump.MOBI == nil {
		t.Fatal("MOBI header not decoded")
	}
	if dump.MOBI.Compression != NoCompression || dump.MOBI.CompressionName != "none" {
		t.Errorf("compression = %d (%s), want 1 (none)", dump.MOBI.Compression, dump.MOBI.CompressionName)
	}
	if dump.MOBI.FullName != "Reader Test" {
		t.Errorf("FullName = %q, want %q", dump.MOBI.FullName, "Reader Test")
	}
	if dump.MOBI.FirstContentRec != 1 {
		t.Errorf("FirstContentRec = %d, want 1", dump.MOBI.FirstContentRec)
	}
	if dump.Records[0].Kind != "mobi-header" {
		t.Errorf("record 0 kind = %q, want mobi-header", dump.Records[0].Kind)
	}
	if dump.Records[1].Kind != "text" {
		t.Errorf("record 1 kind = %q, want text", dump.Records[1].Kind)
	}

	// Record unique IDs are sequential 0..n-1 with seed above them all.
	for i, rec := range dump.Records {
		if rec.UniqueID != uint32(i) {
			t.Errorf("record %d uniqueID = %d, want %d", i, rec.UniqueID, i)
		}
	}
	if dump.PalmDB.UniqueIDSeed <= uint32(len(dump.Records)-1) {
		t.Errorf("UniqueIDSeed = %d, must exceed all record unique IDs (max %d)",
			dump.PalmDB.UniqueIDSeed, len(dump.Records)-1)
	}

	if dump.EXTH == nil {
		t.Fatal("EXTH header not decoded")
	}
	var hasTitle bool
	for _, r := range dump.EXTH.Records {
		if r.Type == EXTHTitle && r.Text == "Reader Test" {
			hasTitle = true
		}
	}
	if !hasTitle {
		t.Errorf("EXTH title record with text %q not found", "Reader Test")
	}

	if text := dump.String(); !strings.Contains(text, "MOBI header (record 0)") {
		t.Error("text serialization misses MOBI header section")
	}
}

func TestReadDumpRejectsNonMOBI(t *testing.T) {
	if _, err := ReadDump([]byte("not a mobi file at all, really")); err == nil {
		t.Error("ReadDump accepted garbage input")
	}
	if _, err := ReadDump([]byte{}); err == nil {
		t.Error("ReadDump accepted empty input")
	}
}

func TestExtractRawMLRoundTrip(t *testing.T) {
	data := writeTestMOBI(t)

	rawml, err := ExtractRawML(data)
	if err != nil {
		t.Fatalf("ExtractRawML failed: %v", err)
	}
	if rawml != newTestBook().Content {
		t.Errorf("rawml does not round-trip the written content:\n got: %q\nwant: %q", rawml, newTestBook().Content)
	}
}

func TestExtractRawMLRejectsCompression(t *testing.T) {
	data := writeTestMOBI(t)
	// Patch compression field (record 0, offset 0) to PalmDOC.
	rec0 := dump0Offset(t, data)
	data[rec0] = 0x00
	data[rec0+1] = 0x02

	if _, err := ExtractRawML(data); err == nil {
		t.Error("ExtractRawML accepted PalmDOC compression")
	}
}

func TestDiff(t *testing.T) {
	data := writeTestMOBI(t)

	report, err := Diff(data, data)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if report.FirstDivergence != nil {
		t.Errorf("identical files reported divergence at offset %d", report.FirstDivergence.Offset)
	}

	// Flip one byte inside the first text record (record 1).
	changed := bytes.Clone(data)
	changed[recordOffset(t, changed, 1)+10] ^= 0xFF

	report, err = Diff(data, changed)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if report.FirstDivergence == nil {
		t.Fatal("differing files reported as identical")
	}
	wantOffset := recordOffset(t, data, 1) + 10
	if report.FirstDivergence.Offset != uint32(wantOffset) {
		t.Errorf("divergence offset = %d, want %d", report.FirstDivergence.Offset, wantOffset)
	}
	if report.FirstDivergence.RecordA != 1 || report.FirstDivergence.RecordB != 1 {
		t.Errorf("divergence records = A#%d B#%d, want A#1 B#1",
			report.FirstDivergence.RecordA, report.FirstDivergence.RecordB)
	}
	if report.Records[1].BytesEqual {
		t.Error("record 1 reported as equal despite flipped byte")
	}
	if text := report.String(); !strings.Contains(text, "first divergence") {
		t.Error("text serialization misses first divergence")
	}
}

func TestDiffDifferentRecordCounts(t *testing.T) {
	a := writeTestMOBI(t)
	if len(a) < 78+8 {
		t.Fatal("test file too short")
	}

	// Claim one record fewer in B: the header field shrinks the parsed list.
	b := bytes.Clone(a)
	numRecords := len(mustRecords(t, a))
	b[76] = byte((numRecords - 1) >> 8)
	b[77] = byte(numRecords - 1)

	report, err := Diff(a, b)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if report.NumRecordsA != numRecords || report.NumRecordsB != numRecords-1 {
		t.Fatalf("record counts = A:%d B:%d, want A:%d B:%d",
			report.NumRecordsA, report.NumRecordsB, numRecords, numRecords-1)
	}
	last := report.Records[len(report.Records)-1]
	if last.LengthB != -1 {
		t.Errorf("missing record reported length %d, want -1", last.LengthB)
	}
	if text := report.String(); !strings.Contains(text, "only in A") {
		t.Error("text serialization misses 'only in A' status")
	}
}

// helpers

func mustRecords(t *testing.T, data []byte) []RecordDump {
	t.Helper()
	_, records, err := parsePalmDB(data)
	if err != nil {
		t.Fatalf("parsePalmDB failed: %v", err)
	}
	return records
}

func dump0Offset(t *testing.T, data []byte) uint32 {
	t.Helper()
	return mustRecords(t, data)[0].Offset
}

func recordOffset(t *testing.T, data []byte, index int) uint32 {
	t.Helper()
	return mustRecords(t, data)[index].Offset
}

// TestReadDumpShortHeader pins the short-header contract: fields beyond
// HeaderLength do not exist in an old file's record 0 (the record physically
// ends with its header) and must decode as zero instead of panicking on an
// out-of-range slice read.
func TestReadDumpShortHeader(t *testing.T) {
	full := createMinimalMOBI()
	// Record 0 starts at 86. Shrink HeaderLength to the 0x84 minimum and cut
	// the record to that physical length; declare uncompressed text so
	// ExtractRawML proceeds past the compression check.
	binary.BigEndian.PutUint32(full[86+16+4:86+16+8], 0x84)
	binary.BigEndian.PutUint16(full[86:86+2], uint16(NoCompression))
	data := append(full[:86:86], full[86:86+16+0x84]...)

	dump, err := ReadDump(data)
	if err != nil {
		t.Fatalf("ReadDump failed: %v", err)
	}
	if dump.MOBI == nil {
		t.Fatal("MOBI header not decoded")
	}
	// Everything past 0x84 (FirstContentRec at 0xC0, INDXRecordOffset at
	// 0xF4, ...) is absent and reads as zero.
	if dump.MOBI.FirstContentRec != 0 || dump.MOBI.LastContentRec != 0 {
		t.Errorf("content record range = %d..%d, want 0..0 (absent in a 0x84 header)",
			dump.MOBI.FirstContentRec, dump.MOBI.LastContentRec)
	}
	if dump.MOBI.FCISIndex != 0 || dump.MOBI.FLISIndex != 0 || dump.MOBI.INDXRecordOffset != 0 {
		t.Errorf("structural indices = fcis %d flis %d indx %d, want all 0 (absent fields)",
			dump.MOBI.FCISIndex, dump.MOBI.FLISIndex, dump.MOBI.INDXRecordOffset)
	}
	if dump.MOBI.ExtraRecordFlags != 0 {
		t.Errorf("ExtraRecordFlags = 0x%X, want 0 (field absent)", dump.MOBI.ExtraRecordFlags)
	}

	// Text extraction fails with an explicit error (no valid content range),
	// not a panic.
	if _, err := ExtractRawML(data); err == nil {
		t.Error("ExtractRawML accepted a header-only file without a content range")
	}
}
