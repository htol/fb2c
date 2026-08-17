package mobi

import (
	"hash/fnv"
	"strings"
)

// timestampToPalmTime converts Unix timestamp to Palm OS time
// Palm OS time = seconds since Jan 1, 1904
// Unix time = seconds since Jan 1, 1970
// Difference = 2082844800 seconds (66 years)
func timestampToPalmTime(unix int64) uint32 {
	// Use 0 as "now" for now
	// In production, would use time.Now().Unix()
	const unixToPalmOffset = 2082844800
	return uint32(unix + unixToPalmOffset) //nolint:gosec // 2106 issue acknowledged
}

// generateUniqueIDSeed returns the PalmDB unique-ID seed. Record unique IDs are
// assigned sequentially 0..numRecords-1, so numRecords+1 is always greater
// than every ID in use, as the PalmDB spec requires.
func generateUniqueIDSeed(numRecords int) uint32 {
	return uint32(numRecords) + 1 //nolint:gosec // Record count is bounded by the 16-bit NumRecords field
}

// generateUniqueID derives the MOBI header unique ID from the book name via
// FNV-1a: deterministic output (same book -> same ID), distinct between books.
func generateUniqueID(name string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return h.Sum32()
}

// transliterateName converts Cyrillic characters to Latin transliteration
// This ensures the PalmDB name field contains only ASCII characters as required by the PalmDB spec
func transliterateName(name string) string {
	result := &strings.Builder{}

	for _, r := range name {
		if r < 128 {
			// ASCII - keep as is (but avoid null bytes)
			if r != 0 {
				result.WriteRune(r)
			}
		} else {
			// Cyrillic - map to Latin approximation
			result.WriteString(transliterateRune(r))
		}
	}

	resultStr := result.String()
	// Truncate to 31 chars max (for PalmDB name field)
	if len(resultStr) > 31 {
		resultStr = resultStr[:31]
	}

	return resultStr
}

// transliterationMap maps Cyrillic characters to their Latin approximation
var transliterationMap = map[rune]string{
	0x0410: "A", 0x0411: "B", 0x0412: "V", 0x0413: "G", 0x0414: "D",
	0x0415: "E", 0x0401: "Yo", 0x0416: "Zh", 0x0417: "Z", 0x0418: "I",
	0x0419: "Y", 0x041A: "K", 0x041B: "L", 0x041C: "M", 0x041D: "N",
	0x041E: "O", 0x041F: "P", 0x0420: "R", 0x0421: "S", 0x0422: "T",
	0x0423: "U", 0x0424: "F", 0x0425: "Kh", 0x0426: "Ts", 0x0427: "Ch",
	0x0428: "Sh", 0x0429: "Shch", 0x042A: "\"", 0x042B: "'", 0x042C: "'",
	0x042D: "E", 0x042E: "Yu", 0x042F: "Ya",
	0x0430: "a", 0x0431: "b", 0x0432: "v", 0x0433: "g", 0x0434: "d",
	0x0435: "e", 0x0451: "yo", 0x0436: "zh", 0x0437: "z", 0x0438: "i",
	0x0439: "y", 0x043A: "k", 0x043B: "l", 0x043C: "m", 0x043D: "n",
	0x043E: "o", 0x043F: "p", 0x0440: "r", 0x0441: "s", 0x0442: "t",
	0x0443: "u", 0x0444: "f", 0x0445: "kh", 0x0446: "ts", 0x0447: "ch",
	0x0448: "sh", 0x0449: "shch", 0x044A: "\"", 0x044B: "'", 0x044C: "'",
	0x044D: "e", 0x044E: "yu", 0x044F: "ya",
}

// transliterateRune maps a single Cyrillic character to its Latin approximation
func transliterateRune(r rune) string {
	if val, ok := transliterationMap[r]; ok {
		return val
	}
	return "?"
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
