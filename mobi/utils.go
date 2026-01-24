package mobi

import (
	"crypto/rand"
	"math/big"
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

// generateRandomUniqueIDSeed generates a random unique ID seed
func generateRandomUniqueIDSeed() uint32 {
	// Generate random number between 1 and 2^32-1
	n, _ := rand.Int(rand.Reader, big.NewInt(0xFFFFFFFF))
	return uint32(n.Uint64()) + 1 //nolint:gosec // Range is guaranteed by big.NewInt
}

// generateRandomID generates a random MOBI ID
func generateRandomID() uint32 {
	// Generate random number between 1 and 2^32-1
	n, _ := rand.Int(rand.Reader, big.NewInt(0xFFFFFFFF))
	return uint32(n.Uint64()) + 1 //nolint:gosec // Range is guaranteed by big.NewInt
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

// transliterateRune maps a single Cyrillic character to its Latin approximation
func transliterateRune(r rune) string {
	// Uppercase Cyrillic
	switch r {
	case 0x0410: // А
		return "A"
	case 0x0411: // Б
		return "B"
	case 0x0412: // В
		return "V"
	case 0x0413: // Г
		return "G"
	case 0x0414: // Д
		return "D"
	case 0x0415: // Е
		return "E"
	case 0x0401: // Ё
		return "Yo"
	case 0x0416: // Ж
		return "Zh"
	case 0x0417: // З
		return "Z"
	case 0x0418: // И
		return "I"
	case 0x0419: // Й
		return "Y"
	case 0x041A: // К
		return "K"
	case 0x041B: // Л
		return "L"
	case 0x041C: // М
		return "M"
	case 0x041D: // Н
		return "N"
	case 0x041E: // О
		return "O"
	case 0x041F: // П
		return "P"
	case 0x0420: // Р
		return "R"
	case 0x0421: // С
		return "S"
	case 0x0422: // Т
		return "T"
	case 0x0423: // У
		return "U"
	case 0x0424: // Ф
		return "F"
	case 0x0425: // Х
		return "Kh"
	case 0x0426: // Ц
		return "Ts"
	case 0x0427: // Ч
		return "Ch"
	case 0x0428: // Ш
		return "Sh"
	case 0x0429: // Щ
		return "Shch"
	case 0x042A: // Ъ
		return "\""
	case 0x042B: // Ы
		return "'"
	case 0x042C: // Ь
		return "'"
	case 0x042D: // Э
		return "E"
	case 0x042E: // Ю
		return "Yu"
	case 0x042F: // Я
		return "Ya"
	// Lowercase Cyrillic
	case 0x0430: // а
		return "a"
	case 0x0431: // б
		return "b"
	case 0x0432: // в
		return "v"
	case 0x0433: // г
		return "g"
	case 0x0434: // д
		return "d"
	case 0x0435: // е
		return "e"
	case 0x0451: // ё
		return "yo"
	case 0x0436: // ж
		return "zh"
	case 0x0437: // з
		return "z"
	case 0x0438: // и
		return "i"
	case 0x0439: // й
		return "y"
	case 0x043A: // к
		return "k"
	case 0x043B: // л
		return "l"
	case 0x043C: // м
		return "m"
	case 0x043D: // н
		return "n"
	case 0x043E: // о
		return "o"
	case 0x043F: // п
		return "p"
	case 0x0440: // р
		return "r"
	case 0x0441: // с
		return "s"
	case 0x0442: // т
		return "t"
	case 0x0443: // у
		return "u"
	case 0x0444: // ф
		return "f"
	case 0x0445: // х
		return "kh"
	case 0x0446: // ц
		return "ts"
	case 0x0447: // ч
		return "ch"
	case 0x0448: // ш
		return "sh"
	case 0x0449: // щ
		return "shch"
	case 0x044A: // ъ
		return "\""
	case 0x044B: // ы
		return "'"
	case 0x044C: // ь
		return "'"
	case 0x044D: // э
		return "e"
	case 0x044E: // ю
		return "yu"
	case 0x044F: // я
		return "ya"
	default:
		// Unknown character, use replacement
		return "?"
	}
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
