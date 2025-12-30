package fb2encoding

import (
	"bytes"
	"testing"
)

func TestDetectBOM(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantEnc  string
		wantBOM  bool
		wantConf float64
	}{
		{
			name:     "UTF-8 BOM",
			input:    append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello")...),
			wantEnc:  "utf-8",
			wantBOM:  true,
			wantConf: 1.0,
		},
		{
			name:     "UTF-16 LE BOM",
			input:    append([]byte{0xFF, 0xFE}, []byte("H")...),
			wantEnc:  "utf-16le",
			wantBOM:  true,
			wantConf: 1.0,
		},
		{
			name:     "UTF-16 BE BOM",
			input:    append([]byte{0xFE, 0xFF}, []byte{0, 'H'}...),
			wantEnc:  "utf-16be",
			wantBOM:  true,
			wantConf: 1.0,
		},
		{
			name:     "no BOM",
			input:    []byte("Hello World"),
			wantEnc:  "utf-8",
			wantBOM:  false,
			wantConf: 0.8, // Valid UTF-8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.input)
			if got.Encoding != tt.wantEnc {
				t.Errorf("Detect() encoding = %v, want %v", got.Encoding, tt.wantEnc)
			}
			if got.BOM != tt.wantBOM {
				t.Errorf("Detect() BOM = %v, want %v", got.BOM, tt.wantBOM)
			}
			if got.Confidence != tt.wantConf {
				t.Errorf("Detect() confidence = %v, want %v", got.Confidence, tt.wantConf)
			}
		})
	}
}

func TestFindEncodingDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "XML declaration",
			input: `<?xml version="1.0" encoding="windows-1251"?>`,
			want:  "windows-1251",
		},
		{
			name:  "XML declaration with quotes",
			input: `<?xml version='1.0' encoding="UTF-8"?>`,
			want:  "UTF-8",
		},
		{
			name:  "HTML5 charset",
			input: `<meta charset="utf-8">`,
			want:  "utf-8",
		},
		{
			name:  "HTML4 pragma",
			input: `<meta http-equiv="Content-Type" content="text/html; charset=iso-8859-1">`,
			want:  "iso-8859-1",
		},
		{
			name:  "no declaration",
			input: `<html><body>Hello</body></html>`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findEncodingDeclaration([]byte(tt.input))
			if got != tt.want {
				t.Errorf("findEncodingDeclaration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeEncoding(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"utf-8", "utf-8"},
		{"UTF-8", "utf-8"},
		{"utf8", "utf-8"},
		{"UTF8", "utf-8"},
		{"windows-1251", "windows-1251"},
		{"macintosh", "mac-roman"},
		{"ascii", "utf-8"},
		{"gb2312", "gbk"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeEncoding(tt.input)
			if got != tt.want {
				t.Errorf("normalizeEncoding(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToUTF8(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{
			name:  "plain UTF-8",
			input: []byte("Hello World"),
			want:  "Hello World",
		},
		{
			name:  "UTF-8 with BOM",
			input: append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello")...),
			want:  "Hello",
		},
		{
			name:  "UTF-8 Cyrillic",
			input: []byte{0xD0, 0xBF, 0xD1, 0x80, 0xD0, 0xB8, 0xD0, 0xB2, 0xD0, 0xB5, 0xD1, 0x82}, // "привет"
			want:  "привет",
		},
		{
			name:    "UTF-16 LE with BOM",
			input:   []byte{0xFF, 0xFE, 0x3C, 0x00}, // BOM + "<"
			want:    "<",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToUTF8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToUTF8() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToUTF8() = %q (% x), want %q", got, got, tt.want)
			}
		})
	}
}

func TestStripEncodingDeclarations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "XML declaration",
			input: `<?xml version="1.0" encoding="windows-1251"?><root/>`,
			want:  `<root/>`,
		},
		{
			name:  "HTML meta",
			input: `<html><meta charset="utf-8"><body>Test</body></html>`,
			want:  `<html><body>Test</body></html>`,
		},
		{
			name:  "no declaration",
			input: `<root/>`,
			want:  `<root/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripEncodingDeclarations(tt.input)
			if got != tt.want {
				t.Errorf("stripEncodingDeclarations() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplaceEncodingInDeclaration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		newEncoding string
		want       string
		changed    bool
	}{
		{
			name:       "change encoding",
			input:      `<?xml version="1.0" encoding="windows-1251"?>`,
			newEncoding: "utf-8",
			want:       `<?xml version="1.0" encoding="utf-8"?>`,
			changed:    true,
		},
		{
			name:       "same encoding",
			input:      `<?xml version="1.0" encoding="utf-8"?>`,
			newEncoding: "utf-8",
			want:       `<?xml version="1.0" encoding="utf-8"?>`,
			changed:    false,
		},
		{
			name:       "no declaration",
			input:      `<root/>`,
			newEncoding: "utf-8",
			want:       `<root/>`,
			changed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := ReplaceEncodingInDeclaration(tt.input, tt.newEncoding)
			if got != tt.want {
				t.Errorf("ReplaceEncodingInDeclaration() = %q, want %q", got, tt.want)
			}
			if changed != tt.changed {
				t.Errorf("ReplaceEncodingInDeclaration() changed = %v, want %v", changed, tt.changed)
			}
		})
	}
}

func TestLooksLikeUTF16(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantLE   bool
		wantBE   bool
	}{
		{
			name:   "ASCII text",
			input:  []byte("Hello World"),
			wantLE: false,
			wantBE: false,
		},
		{
			name:   "UTF-16 LE ASCII",
			input:  []byte{'H', 0, 'e', 0, 'l', 0, 'l', 0, 'o', 0},
			wantLE: true,
			wantBE: false,
		},
		{
			name:   "UTF-16 BE ASCII",
			input:  []byte{0, 'H', 0, 'e', 0, 'l', 0, 'l', 0, 'o'},
			wantLE: false,
			wantBE: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLE := looksLikeUTF16LE(tt.input)
			gotBE := looksLikeUTF16BE(tt.input)
			if gotLE != tt.wantLE {
				t.Errorf("looksLikeUTF16LE() = %v, want %v", gotLE, tt.wantLE)
			}
			if gotBE != tt.wantBE {
				t.Errorf("looksLikeUTF16BE() = %v, want %v", gotBE, tt.wantBE)
			}
		})
	}
}

func TestToUTF8WithStrip(t *testing.T) {
	// Use UTF-8 data which we can decode
	input := []byte(`<?xml version="1.0" encoding="utf-8"?><root>Hello World</root>`)

	result, enc, err := ToUTF8WithStrip(input, true)
	if err != nil {
		t.Fatalf("ToUTF8WithStrip() error = %v", err)
	}

	// Check that encoding declaration was stripped
	if bytes.Contains([]byte(result), []byte("encoding")) {
		t.Error("ToUTF8WithStrip() did not strip encoding declaration")
	}

	// Check that we got some result
	if len(result) == 0 {
		t.Error("ToUTF8WithStrip() returned empty result")
	}

	t.Logf("Encoding: %s, Result: %s", enc, result)
}
