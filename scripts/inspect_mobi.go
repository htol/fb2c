package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// Simplified structures for reading for inspection

type PDBHeader struct {
	Name               [32]byte
	Attributes         uint16
	Version            uint16
	CreationTime       uint32
	ModificationTime   uint32
	BackupTime         uint32
	ModificationNumber uint32
	AppInfoID          uint32
	SortInfoID         uint32
	Type               [4]byte
	Creator            [4]byte
	UniqueIDSeed       uint32
	NextRecordListID   uint32
	NumRecords         uint16
}

type RecordEntry struct {
	Offset     uint32
	Attributes byte
	UniqueID   [3]byte
}

type PalmDOCHeader struct {
	Compression          uint16
	Unused1              uint16
	UncompressedTextSize uint32
	RecordCount          uint16
	RecordSize           uint16
	EncryptionType       uint16
	Unused2              uint16
}

type MOBIHeader struct {
	MOBIMarker          [4]byte
	HeaderLength        uint32
	MOBIType            uint32
	TextEncoding        uint32
	UniqueID            uint32
	FileVersion         uint32
	OrthographicIndex   uint32
	InflectionIndex     uint32
	IndexNames          uint32
	IndexKeys           uint32
	ExtraIndex0         uint32
	ExtraIndex1         uint32
	ExtraIndex2         uint32
	ExtraIndex3         uint32
	ExtraIndex4         uint32
	ExtraIndex5         uint32
	FirstNonBookIndex   uint32
	FullNameOffset      uint32
	FullNameLength      uint32
	Locale              uint32
	InputLanguage       uint32
	OutputLanguage      uint32
	MinVersion          uint32
	FirstImageIndex     uint32
	HuffmanRecordOffset uint32
	HuffmanRecordCount  uint32
	HuffmanTableOffset  uint32
	HuffmanTableLength  uint32
	EXTHFlags           uint32
	Unknown1            [32]byte
	Unknown2            uint32
	DRMOffset           uint32
	DRMCount            uint32
	DRMSize             uint32
	DRMFlags            uint32
	Unknown4            [8]byte
	FirstContentRec     uint16
	LastContentRec      uint16
	Unknown5            uint32
	FCISIndex           uint32  // 0xC8
	FCISCount           uint32  // 0xCC
	FLISIndex           uint32  // 0xD0
	FLISCount           uint32  // 0xD4
	Unknown216          [8]byte // 0xD8
	Unknown224          uint32  // 0xE0
	FirstCompilation    uint32  // 0xE4
	NumCompilation      uint32  // 0xE8
	Unknown236          uint32  // 0xEC
	ExtraRecordFlags    uint32  // 0xF0
	INDXRecordOffset    uint32  // 0xF4
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run scripts/inspect_mobi.go <mobi_file>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	r := bytes.NewReader(data)

	// Read PDB Header
	var pdb PDBHeader
	if err := binary.Read(r, binary.BigEndian, &pdb); err != nil {
		panic(err)
	}

	fmt.Printf("=== PDB Header ===\n")
	fmt.Printf("Name: %s\n", string(pdb.Name[:bytes.IndexByte(pdb.Name[:], 0)]))
	fmt.Printf("Version: %d\n", pdb.Version)
	fmt.Printf("NumRecords: %d\n", pdb.NumRecords)
	fmt.Printf("Type: %s\n", string(pdb.Type[:]))
	fmt.Printf("Creator: %s\n", string(pdb.Creator[:]))
	fmt.Printf("UniqueIDSeed: %d (0x%X)\n", pdb.UniqueIDSeed, pdb.UniqueIDSeed)
	fmt.Printf("NextRecordListID: %d\n", pdb.NextRecordListID)

	// Read Record List
	records := make([]RecordEntry, pdb.NumRecords)
	for i := 0; i < int(pdb.NumRecords); i++ {
		entry := RecordEntry{}
		if err := binary.Read(r, binary.BigEndian, &entry.Offset); err != nil {
			panic(err)
		}
		if err := binary.Read(r, binary.BigEndian, &entry.Attributes); err != nil {
			panic(err)
		}
		// Read 3 bytes
		uidBytes := make([]byte, 3)
		if err := binary.Read(r, binary.BigEndian, &uidBytes); err != nil {
			panic(err)
		}
		copy(entry.UniqueID[:], uidBytes)
		records[i] = entry
	}

	fmt.Printf("\n=== Record List (First 5 and Special) ===\n")
	for i := 0; i < len(records); i++ {
		if i < 5 || i == int(pdb.NumRecords)-1 {
			uidVal := uint32(records[i].UniqueID[0])<<16 | uint32(records[i].UniqueID[1])<<8 | uint32(records[i].UniqueID[2])
			fmt.Printf("Record %d: Offset %d (0x%X), Attr %d, UID %d\n", i, records[i].Offset, records[i].Offset, records[i].Attributes, uidVal)
		}
	}

	// Read Record 0 (MOBI Header)
	if len(records) > 0 {
		r0Offset := records[0].Offset
		// Skip over PDB header and record list to be safe? No, offset is absolute.
		// We can just seek or slice data.
		if int(r0Offset) >= len(data) {
			fmt.Printf("Error: Record 0 offset out of bounds\n")
			return
		}

		// PDB header is 78 bytes + record list entries (8 bytes each) + 2 padding bytes usually
		// But offsets are absolute.

		recData := data[r0Offset:]

		var palmDoc PalmDOCHeader
		pr := bytes.NewReader(recData)
		if err := binary.Read(pr, binary.BigEndian, &palmDoc); err != nil {
			fmt.Printf("Error reading PalmDOC header: %v\n", err)
		} else {
			fmt.Printf("\n=== PalmDOC Header ===\n")
			fmt.Printf("Compression: %d (1=No, 2=PalmDOC)\n", palmDoc.Compression)
			fmt.Printf("TextLength: %d\n", palmDoc.UncompressedTextSize)
			fmt.Printf("RecordCount: %d\n", palmDoc.RecordCount)
			fmt.Printf("RecordSize: %d\n", palmDoc.RecordSize)
			fmt.Printf("Encryption: %d\n", palmDoc.EncryptionType)
		}

		var mobiHeader MOBIHeader
		// standard MOBI header starts at +16 from record start (after PalmDOC)
		// But our struct includes the 16 bytes shift implicit in the reads if we just continue?
		// No, PalmDOCHeader is 16 bytes. So we can just continue reading from pr.

		if err := binary.Read(pr, binary.BigEndian, &mobiHeader); err != nil {
			fmt.Printf("Error reading MOBI header: %v\n", err)
		} else {
			fmt.Printf("\n=== MOBI Header ===\n")
			fmt.Printf("Marker: %s\n", string(mobiHeader.MOBIMarker[:]))
			fmt.Printf("HeaderLength: %d\n", mobiHeader.HeaderLength)
			fmt.Printf("Type: %d (2=Book)\n", mobiHeader.MOBIType)
			fmt.Printf("Encoding: %d (65001=UTF8)\n", mobiHeader.TextEncoding)
			fmt.Printf("Details:\n")
			if len(records) > 1 {
				fmt.Printf("  Rec 0 Offset: %d\n", records[0].Offset)
				fmt.Printf("  Rec 1 Offset: %d\n", records[1].Offset)
			}
			fmt.Printf("  Record 0 Size: %d\n", len(data))
			fmt.Printf("  Header Length: %d\n", mobiHeader.HeaderLength)
			// Dump last 64 bytes of header for analysis
			startDump := len(data) - 64
			if startDump < 0 {
				startDump = 0
			}
			fmt.Printf("  Last 64 bytes: %X\n", data[startDump:])
			fmt.Printf("  FileVersion: %d\n", mobiHeader.FileVersion)
			fmt.Printf("  MinVersion: %d\n", mobiHeader.MinVersion)
			fmt.Printf("  FullNameOffset: %d\n", mobiHeader.FullNameOffset)
			fmt.Printf("  FullNameLength: %d\n", mobiHeader.FullNameLength)
			fmt.Printf("  FirstImageIndex: %d (0x%X)\n", mobiHeader.FirstImageIndex, mobiHeader.FirstImageIndex)
			fmt.Printf("  FirstNonBookIndex: %d (0x%X)\n", mobiHeader.FirstNonBookIndex, mobiHeader.FirstNonBookIndex)
			fmt.Printf("  EXTH Flags: 0x%X\n", mobiHeader.EXTHFlags)
			fmt.Printf("  FirstContentRec: %d\n", mobiHeader.FirstContentRec)
			fmt.Printf("  LastContentRec: %d\n", mobiHeader.LastContentRec)
			fmt.Printf("  FCISIndex: %d (0x%X)\n", mobiHeader.FCISIndex, mobiHeader.FCISIndex)
			fmt.Printf("  FLISIndex: %d (0x%X)\n", mobiHeader.FLISIndex, mobiHeader.FLISIndex)
			fmt.Printf("  ExtraRecordFlags: %d\n", mobiHeader.ExtraRecordFlags)
			fmt.Printf("  INDXRecordOffset: %d (0x%X)\n", mobiHeader.INDXRecordOffset, mobiHeader.INDXRecordOffset)
			fmt.Printf("  IndexKeys: %d (0x%X)\n", mobiHeader.IndexKeys, mobiHeader.IndexKeys)

			// Dump EXTH
			if mobiHeader.EXTHFlags&0x40 != 0 {
				exthOffset := int64(16 + mobiHeader.HeaderLength)
				if int64(len(recData)) > exthOffset+4 {
					exthParams := recData[exthOffset:]
					if string(exthParams[0:4]) == "EXTH" {
						exthLen := binary.BigEndian.Uint32(exthParams[4:8])
						exthCount := binary.BigEndian.Uint32(exthParams[8:12])
						fmt.Printf("\n=== EXTH Header ===\n")
						fmt.Printf("Identifier: EXTH\n")
						fmt.Printf("Length: %d\n", exthLen)
						fmt.Printf("Count: %d\n", exthCount)

						// Dump records
						pos := 12
						for i := 0; i < int(exthCount); i++ {
							if pos+8 > len(exthParams) {
								break
							}
							recType := binary.BigEndian.Uint32(exthParams[pos : pos+4])
							recLen := binary.BigEndian.Uint32(exthParams[pos+4 : pos+8])
							if pos+int(recLen) > len(exthParams) {
								break
							}
							recData := exthParams[pos+8 : pos+int(recLen)]
							fmt.Printf("  Type: %d, Len: %d", recType, recLen-8)
							// Print string if it looks like ASCII
							isASCII := true
							for _, b := range recData {
								if b < 32 || b > 126 {
									isASCII = false
									break
								}
							}
							if isASCII && len(recData) > 0 {
								fmt.Printf(", Val: %s", string(recData))
							} else if len(recData) <= 8 {
								// Print hex for short binary
								fmt.Printf(", Val: %X", recData)
							}
							fmt.Println()
							pos += int(recLen)
						}

					}
				}
			}
		}

		// FCIS Check
		if mobiHeader.FCISIndex != 0xFFFFFFFF && int(mobiHeader.FCISIndex) < len(records) {
			fcisRec := records[mobiHeader.FCISIndex]
			if int(fcisRec.Offset) < len(data) {
				fcisData := data[fcisRec.Offset:]
				if len(fcisData) > 44 {
					fmt.Printf("\n=== FCIS Record (%d) ===\n", mobiHeader.FCISIndex)
					if string(fcisData[:4]) == "FCIS" {
						fmt.Printf("Signature: FCIS\n")
						fmt.Printf("TextLength: %d\n", binary.BigEndian.Uint32(fcisData[20:24]))
					} else {
						fmt.Printf("Signature: %X %X %X %X (Expected FCIS)\n", fcisData[0], fcisData[1], fcisData[2], fcisData[3])
					}
				}
			}

			// FLIS Check
			if mobiHeader.FLISIndex != 0xFFFFFFFF && int(mobiHeader.FLISIndex) < len(records) {
				flisRec := records[mobiHeader.FLISIndex]
				if int(flisRec.Offset) < len(data) {
					flisData := data[flisRec.Offset:]
					if len(flisData) > 36 {
						fmt.Printf("\n=== FLIS Record (%d) ===\n", mobiHeader.FLISIndex)
						fmt.Printf("Signature: %s\n", string(flisData[:4]))
						fmt.Printf("Fixed value 65: %d\n", binary.BigEndian.Uint16(flisData[8:10]))
					}
				}
			}

			// INDX Check
			if mobiHeader.INDXRecordOffset != 0xFFFFFFFF && int(mobiHeader.INDXRecordOffset) < len(records) {
				// INDXRecordOffset is a RECORD INDEX, despite the name in my struct being Offset?
				// In MOBI header struct provided in code: INDXRecordOffset uint32 // 0xF4
				// Let's assume it is an index like others.
				indxRecIdx := mobiHeader.INDXRecordOffset
				if int(indxRecIdx) < len(records) {
					indxRec := records[indxRecIdx]
					if int(indxRec.Offset) < len(data) {
						indxData := data[indxRec.Offset:]
						if len(indxData) > 20 {
							fmt.Printf("\n=== INDX Record (%d) ===\n", indxRecIdx)
							fmt.Printf("Record Size: %d bytes\n", len(indxData))
							if string(indxData[:4]) == "INDX" {
								fmt.Printf("Signature: INDX\n")
								fmt.Printf("Length: %d\n", binary.BigEndian.Uint32(indxData[4:8]))
								fmt.Printf("Type: %d\n", binary.BigEndian.Uint32(indxData[8:12]))
								fmt.Printf("IDXT Offset: %d\n", binary.BigEndian.Uint32(indxData[20:24]))
								fmt.Printf("Count: %d\n", binary.BigEndian.Uint32(indxData[24:28]))
								fmt.Printf("Encoding: %d\n", binary.BigEndian.Uint32(indxData[28:32]))
								fmt.Printf("Language: %d\n", binary.BigEndian.Uint32(indxData[32:36]))
								fmt.Printf("Total Entries: %d\n", binary.BigEndian.Uint32(indxData[36:40]))
								fmt.Printf("ORDT Offset: %d\n", binary.BigEndian.Uint32(indxData[40:44]))
								fmt.Printf("LIGT Offset: %d\n", binary.BigEndian.Uint32(indxData[44:48]))
								// Look for TAGX in header?
								// Standard INDX header is ~192 bytes?
								if len(indxData) > 180 {
									tagxOff := binary.BigEndian.Uint32(indxData[172:176])
									fmt.Printf("TAGX Offset (maybe): %d\n", tagxOff)
								}
							} else {
								fmt.Printf("Signature: %X %X %X %X (Expected INDX)\n", indxData[0], indxData[1], indxData[2], indxData[3])
							}
						}
					}
				}
			}

			// Check for gap records (between INDX and Images)
			if mobiHeader.FirstNonBookIndex != 0xFFFFFFFF && mobiHeader.FirstImageIndex != 0xFFFFFFFF {
				start := int(mobiHeader.FirstNonBookIndex) + 1
				end := int(mobiHeader.FirstImageIndex)
				if start < end {
					fmt.Printf("\n=== Gap Records (%d to %d) ===\n", start, end-1)
					for i := start; i < end; i++ {
						if i < len(records) {
							rec := records[i]
							if int(rec.Offset) < len(data) {
								recData := data[rec.Offset:]
								fmt.Printf("Record %d: Len %d\n", i, len(recData))
								if len(recData) > 4 {
									fmt.Printf("  Signature: %X %X %X %X\n", recData[0], recData[1], recData[2], recData[3])
									if string(recData[:4]) == "INDX" {
										fmt.Println("  Type: INDX")
									} else if string(recData[:4]) == "FLIS" {
										fmt.Println("  Type: FLIS")
									} else if string(recData[:4]) == "FCIS" {
										fmt.Println("  Type: FCIS")
									}
								}
							}
						}
					}
				}
			}

			// Check FirstImageIndex
			if mobiHeader.FirstImageIndex != 0xFFFFFFFF {
				idx := int(mobiHeader.FirstImageIndex)
				if idx < len(records) {
					rec := records[idx]
					if int(rec.Offset) < len(data) {
						imgData := data[rec.Offset:]
						fmt.Printf("\n=== First Image Record (%d) ===\n", idx)
						fmt.Printf("Offset: %d\n", rec.Offset)

						// Check next record offset to determine true length
						if idx+1 < len(records) {
							nextOff := records[idx+1].Offset
							fmt.Printf("Next Record (%d) Offset: %d\n", idx+1, nextOff)
							trueLen := nextOff - rec.Offset
							fmt.Printf("Calculated Length: %d\n", trueLen)
						}

						// Check for common signatures
						if len(imgData) > 10 {
							// JPEG: FF D8 FF
							// PNG: 89 50 4E 47
							fmt.Printf("Signature: %X %X %X %X\n", imgData[0], imgData[1], imgData[2], imgData[3])
							if imgData[0] == 0xFF && imgData[1] == 0xD8 && imgData[2] == 0xFF {
								fmt.Println("Type: JPEG")
							} else if string(imgData[1:4]) == "PNG" {
								fmt.Println("Type: PNG")
							} else {
								fmt.Println("Type: Unknown")
							}
							fmt.Printf("Length: %d\n", len(imgData))
						}
					}
				}
			}
		}
	}
}
