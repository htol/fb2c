package mobi

import (
	"fmt"
	"strings"
)

// String renders the dump as human-readable text (the default `fb2c dump`
// serialization; --json is the alternative serializer of the same model).
func (d *Dump) String() string {
	var b strings.Builder

	p := &d.PalmDB
	fmt.Fprintf(&b, "PalmDB header:\n")
	fmt.Fprintf(&b, "  Name:              %s\n", p.Name)
	fmt.Fprintf(&b, "  Type/Creator:      %s/%s\n", p.Type, p.Creator)
	fmt.Fprintf(&b, "  Attributes:        %d\n", p.Attributes)
	fmt.Fprintf(&b, "  Version:           %d\n", p.Version)
	fmt.Fprintf(&b, "  CreationDate:      %d\n", p.CreationDate)
	fmt.Fprintf(&b, "  ModificationDate:  %d\n", p.ModificationDate)
	fmt.Fprintf(&b, "  UniqueIDSeed:      %d\n", p.UniqueIDSeed)
	fmt.Fprintf(&b, "  NextRecordListID:  %d\n", p.NextRecordListID)
	fmt.Fprintf(&b, "  NumRecords:        %d\n", p.NumRecords)

	fmt.Fprintf(&b, "\nRecords (%d):\n", len(d.Records))
	fmt.Fprintf(&b, "  %-4s %-10s %-8s %-5s %-6s %s\n", "#", "offset", "length", "attr", "uid", "kind")
	for _, r := range d.Records {
		fmt.Fprintf(&b, "  %-4d 0x%08X %-8d %-5d %-6d %s\n",
			r.Index, r.Offset, r.Length, r.Attributes, r.UniqueID, r.Kind)
		if r.INDX != nil {
			fmt.Fprintf(&b, "      INDX: type=%d entries=%d total=%d encoding=%d language=%d cncx=%d idxtOffset=%d\n",
				r.INDX.IndexType, r.INDX.Count, r.INDX.TotalRecordCount,
				r.INDX.Encoding, r.INDX.Language, r.INDX.CNCXCount, r.INDX.IDXTOffset)
		}
	}

	if d.MOBI != nil {
		h := d.MOBI
		fmt.Fprintf(&b, "\nMOBI header (record 0):\n")
		fmt.Fprintf(&b, "  Compression:          %d (%s)\n", h.Compression, h.CompressionName)
		fmt.Fprintf(&b, "  UncompressedTextSize: %d\n", h.UncompressedTextSize)
		fmt.Fprintf(&b, "  RecordCount:          %d\n", h.RecordCount)
		fmt.Fprintf(&b, "  RecordSize:           %d\n", h.RecordSize)
		fmt.Fprintf(&b, "  TextEncoding:         %d\n", h.TextEncoding)
		fmt.Fprintf(&b, "  UniqueID:             %d\n", h.UniqueID)
		fmt.Fprintf(&b, "  FileVersion:          %d\n", h.FileVersion)
		fmt.Fprintf(&b, "  Locale:               %d\n", h.Locale)
		fmt.Fprintf(&b, "  FirstImageIndex:      %d\n", h.FirstImageIndex)
		fmt.Fprintf(&b, "  FirstNonBookIndex:    %d\n", h.FirstNonBookIndex)
		fmt.Fprintf(&b, "  FirstContentRec:      %d\n", h.FirstContentRec)
		fmt.Fprintf(&b, "  LastContentRec:       %d\n", h.LastContentRec)
		fmt.Fprintf(&b, "  FCISIndex:            %d\n", h.FCISIndex)
		fmt.Fprintf(&b, "  FLISIndex:            %d\n", h.FLISIndex)
		fmt.Fprintf(&b, "  INDXRecordOffset:     %d\n", h.INDXRecordOffset)
		fmt.Fprintf(&b, "  EXTHFlags:            0x%X\n", h.EXTHFlags)
		fmt.Fprintf(&b, "  ExtraRecordFlags:     0x%X\n", h.ExtraRecordFlags)
		fmt.Fprintf(&b, "  FullName:             %s\n", h.FullName)
	}

	if d.EXTH != nil {
		fmt.Fprintf(&b, "\nEXTH (%d records, length %d):\n", d.EXTH.RecordCount, d.EXTH.HeaderLength)
		for _, r := range d.EXTH.Records {
			name := r.Name
			if name == "" {
				name = "?"
			}
			value := r.Text
			if value == "" {
				value = "0x" + r.Hex
			}
			fmt.Fprintf(&b, "  %-6d %-16s %s\n", r.Type, name, value)
		}
	}

	return b.String()
}

// String renders a diff report as human-readable text. Exit-code semantics
// (differing files) are the caller's concern.
func (r *DiffReport) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "A: %d records, B: %d records\n", r.NumRecordsA, r.NumRecordsB)
	for _, rd := range r.Records {
		status := "equal"
		switch {
		case rd.LengthA == -1:
			status = "only in B"
		case rd.LengthB == -1:
			status = "only in A"
		case !rd.BytesEqual:
			status = "DIFFER"
		}
		fmt.Fprintf(&b, "  record %-4d A=%dB uid=%-6d B=%dB uid=%-6d %s\n",
			rd.Index, rd.LengthA, rd.UniqueIDA, rd.LengthB, rd.UniqueIDB, status)
	}

	if r.FirstDivergence == nil {
		fmt.Fprintf(&b, "files are identical\n")
	} else {
		fmt.Fprintf(&b, "first divergence: offset 0x%X (%d)", r.FirstDivergence.Offset, r.FirstDivergence.Offset)
		fmt.Fprintf(&b, " in record A %s / record B %s\n",
			recordRef(r.FirstDivergence.RecordA), recordRef(r.FirstDivergence.RecordB))
	}

	return b.String()
}

func recordRef(i int) string {
	if i < 0 {
		return "(header area)"
	}
	return fmt.Sprintf("#%d", i)
}
