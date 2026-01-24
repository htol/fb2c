package mobi

// splitAndCompressRecords splits text data into 4096-byte chunks and compresses them
func (w *Writer) splitAndCompressRecords(textData []byte) [][]byte {
	var textRecords [][]byte
	const recordSize = 4096

	for i := 0; i < len(textData); i += recordSize {
		end := i + recordSize
		if end > len(textData) {
			end = len(textData)
		}

		chunk := textData[i:end]
		record := chunk
		if w.options.CompressionType == PalmDOCCompression {
			record = compressRecord(chunk)
		}

		// In MOBI with ExtraRecordFlags=0x02 (TBS indexing), we add a 4-byte trailer.
		// The last byte 0x84 indicates 4 bytes of extra data (including the size byte).
		// We use 00 00 00 84 to keep it safe. Reference uses 86 80 02 84.
		trailingRecord := make([]byte, len(record)+4)
		copy(trailingRecord, record)
		trailingRecord[len(record)+3] = 0x84
		textRecords = append(textRecords, trailingRecord)
	}
	return textRecords
}
