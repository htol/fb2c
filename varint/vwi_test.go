package varint

import (
	"reflect"
	"testing"
)

func TestEncodeForward(t *testing.T) {
	tests := []struct {
		name  string
		value uint32
		want  []byte
	}{
		{
			name:  "zero",
			value: 0,
			want:  []byte{0x80},
		},
		{
			name:  "small value",
			value: 0x7F,
			want:  []byte{0xFF}, // 0x7F | 0x80 = 0xFF
		},
		{
			name:  "0x11111 from calibre docs",
			value: 0x11111,
			want:  []byte{0x04, 0x22, 0x91},
		},
		{
			name:  "requires two bytes",
			value: 0x80,
			want:  []byte{0x01, 0x80}, // matches Python output
		},
		{
			name:  "max three byte value",
			value: 0x1FFFFF,
			want:  []byte{0x7F, 0x7F, 0xFF}, // matches Python output
		},
		{
			name:  "four bytes",
			value: 0x10000000,
			want:  []byte{0x01, 0x00, 0x00, 0x00, 0x80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeForward(tt.value)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncodeForward(%#x) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestEncodeBackward(t *testing.T) {
	tests := []struct {
		name  string
		value uint32
		want  []byte
	}{
		{
			name:  "zero",
			value: 0,
			want:  []byte{0x80},
		},
		{
			name:  "small value",
			value: 0x7F,
			want:  []byte{0xFF}, // 0x7F | 0x80 = 0xFF
		},
		{
			name:  "0x11111 from calibre docs",
			value: 0x11111,
			want:  []byte{0x84, 0x22, 0x11},
		},
		{
			name:  "requires two bytes",
			value: 0x80,
			want:  []byte{0x81, 0x00}, // matches Python output
		},
		{
			name:  "max three byte value",
			value: 0x1FFFFF,
			want:  []byte{0xFF, 0x7F, 0x7F}, // matches Python output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeBackward(tt.value)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncodeBackward(%#x) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestDecodeForward(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantValue uint32
		wantBytes int
		wantErr   bool
	}{
		{
			name:      "zero",
			data:      []byte{0x80},
			wantValue: 0,
			wantBytes: 1,
		},
		{
			name:      "small value",
			data:      []byte{0xFF},
			wantValue: 0x7F,
			wantBytes: 1,
		},
		{
			name:      "0x11111",
			data:      []byte{0x04, 0x22, 0x91},
			wantValue: 0x11111,
			wantBytes: 3,
		},
		{
			name:      "two bytes",
			data:      []byte{0x01, 0x80},
			wantValue: 0x80,
			wantBytes: 2,
		},
		{
			name:    "empty",
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotBytes, err := DecodeForward(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeForward(%v) error = %v, wantErr %v", tt.data, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotValue != tt.wantValue {
					t.Errorf("DecodeForward(%v) value = %#x, want %#x", tt.data, gotValue, tt.wantValue)
				}
				if gotBytes != tt.wantBytes {
					t.Errorf("DecodeForward(%v) bytes = %d, want %d", tt.data, gotBytes, tt.wantBytes)
				}
			}
		})
	}
}

func TestDecodeBackward(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantValue uint32
		wantBytes int
		wantErr   bool
	}{
		{
			name:      "zero",
			data:      []byte{0x80},
			wantValue: 0,
			wantBytes: 1,
		},
		{
			name:      "small value",
			data:      []byte{0xFF},
			wantValue: 0x7F,
			wantBytes: 1,
		},
		{
			name:      "0x11111",
			data:      []byte{0x84, 0x22, 0x11},
			wantValue: 0x11111,
			wantBytes: 3,
		},
		{
			name:      "two bytes",
			data:      []byte{0x81, 0x00},
			wantValue: 0x80,
			wantBytes: 2,
		},
		{
			name:    "empty",
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotBytes, err := DecodeBackward(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeBackward(%v) error = %v, wantErr %v", tt.data, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotValue != tt.wantValue {
					t.Errorf("DecodeBackward(%v) value = %#x, want %#x", tt.data, gotValue, tt.wantValue)
				}
				if gotBytes != tt.wantBytes {
					t.Errorf("DecodeBackward(%v) bytes = %d, want %d", tt.data, gotBytes, tt.wantBytes)
				}
			}
		})
	}
}

func TestRoundTripForward(t *testing.T) {
	values := []uint32{
		0, 1, 0x7F, 0x80, 0xFF, 0x100, 0x11111, 0xFFFF, 0x10000,
		0x1FFFFF, 0x1000000, 0xFFFFFFFF,
	}

	for _, v := range values {
		encoded := EncodeForward(v)
		decoded, n, err := DecodeForward(encoded)
		if err != nil {
			t.Errorf("RoundTripForward(%#x): decode error = %v", v, err)
			continue
		}
		if decoded != v {
			t.Errorf("RoundTripForward(%#x): got %#x", v, decoded)
		}
		if n != len(encoded) {
			t.Errorf("RoundTripForward(%#x): bytes consumed %d, encoded length %d", v, n, len(encoded))
		}
	}
}

func TestRoundTripBackward(t *testing.T) {
	values := []uint32{
		0, 1, 0x7F, 0x80, 0xFF, 0x100, 0x11111, 0xFFFF, 0x10000,
		0x1FFFFF, 0x1000000, 0xFFFFFFFF,
	}

	for _, v := range values {
		encoded := EncodeBackward(v)
		decoded, n, err := DecodeBackward(encoded)
		if err != nil {
			t.Errorf("RoundTripBackward(%#x): decode error = %v", v, err)
			continue
		}
		if decoded != v {
			t.Errorf("RoundTripBackward(%#x): got %#x, want %#x", v, decoded, v)
		}
		if n != len(encoded) {
			t.Errorf("RoundTripBackward(%#x): bytes consumed %d, encoded length %d", v, n, len(encoded))
		}
	}
}

func TestSize(t *testing.T) {
	tests := []struct {
		value uint32
		want  int
	}{
		{0, 1},
		{0x7F, 1},
		{0x80, 2},
		{0x11111, 3},
		{0x1FFFFF, 3},
		{0x200000, 4},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := Size(tt.value); got != tt.want {
				t.Errorf("Size(%#x) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
