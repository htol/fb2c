package fb2c

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestDeterministicOutput verifies that two conversions of the same input are
// byte-identical, which is what makes byte-level golden tests possible.
func TestDeterministicOutput(t *testing.T) {
	tests := []struct {
		name string
		ext  string
	}{
		{name: "mobi6", ext: ".mobi"},
		{name: "epub", ext: ".epub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			var outputs [2][]byte
			for i := range outputs {
				out := filepath.Join(dir, "out"+tt.ext)
				converter := NewConverter()
				if err := converter.Convert(filepath.Join(fixtureDir, "src_ref.fb2"), out); err != nil {
					t.Fatalf("conversion %d failed: %v", i, err)
				}
				data, err := os.ReadFile(out)
				if err != nil {
					t.Fatalf("failed to read output %d: %v", i, err)
				}
				outputs[i] = data
			}

			if !bytes.Equal(outputs[0], outputs[1]) {
				t.Errorf("two conversions of the same input differ (%d vs %d bytes)",
					len(outputs[0]), len(outputs[1]))
			}
		})
	}
}
