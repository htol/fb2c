package b64

import (
	"encoding/base64"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard base64",
			input: "SGVsbG8gV29ybGQ=", // "Hello World"
			want:  "Hello World",
		},
		{
			name:  "simple",
			input: "QUJD", // "ABC"
			want:  "ABC",
		},
		{
			name:  "with whitespace",
			input: "QU J D", // "ABC" with spaces (should skip)
			want:  "ABC",
		},
		{
			name:  "with invalid chars",
			input: "QU!@#JD", // "ABC" with invalid chars (should skip)
			want:  "ABC",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "padding",
			input: "QQ==", // "A"
			want:  "A",
		},
		{
			name:  "two padding",
			input: "QUE=", // "AA"
			want:  "AA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("Decode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeRobust(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "spaces everywhere",
			input: " S G V s b G 8 g V 2 9 y b G Q = ",
			want:  "Hello World",
		},
		{
			name:  "mixed invalid",
			input: "QU!@$%^&*()JD",
			want:  "ABC",
		},
		{
			name:  "partial quads",
			input: "AB", // Only 2 valid chars -> 1 byte
			want:  "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode([]byte(tt.input))
			if err != nil {
				t.Errorf("Decode() unexpected error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Decode() = %q (% x), want %q (% x)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple",
			input: "ABC",
			want:  "QUJD",
		},
		{
			name:  "hello",
			input: "Hello World",
			want:  "SGVsbG8gV29ybGQ=",
		},
		{
			name:  "binary",
			input: "\xff\xd8\xff\xe0",
			want:  "/9j/4A==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Encode([]byte(tt.input))
			if got != tt.want {
				t.Errorf("Encode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	testData := []string{
		"Hello World",
		"ABC",
		"Test123!@#",
		"\x00\x01\x02\xff\xfe\xfd", // Binary data
	}

	for _, data := range testData {
		encoded := Encode([]byte(data))
		decoded, err := Decode([]byte(encoded))
		if err != nil {
			t.Errorf("RoundTrip(%q): decode error = %v", data, err)
			continue
		}
		if string(decoded) != data {
			t.Errorf("RoundTrip(%q): got %q", data, decoded)
		}
	}
}

func TestCompareWithStdBase64(t *testing.T) {
	testCases := []string{
		"Hello World",
		"ABCDEF",
		"Test123",
		"\x00\x01\x02\x03\x04",
	}

	for _, data := range testCases {
		// Encode with standard base64
		stdEncoded := base64.StdEncoding.EncodeToString([]byte(data))

		// Decode with our decoder
		decoded, err := Decode([]byte(stdEncoded))
		if err != nil {
			t.Errorf("CompareWithStdBase64(%q): error = %v", data, err)
			continue
		}

		if string(decoded) != data {
			t.Errorf("CompareWithStdBase64(%q): got %q", data, decoded)
		}
	}
}
