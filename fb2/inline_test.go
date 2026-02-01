package fb2

import (
	"testing"
)

func TestParseInlineContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "emphasis",
			input: "<emphasis>italic</emphasis>",
			want:  "<em>italic</em>",
		},
		{
			name:  "strong",
			input: "<strong>bold</strong>",
			want:  "<strong>bold</strong>",
		},
		{
			name:  "strikethrough",
			input: "<strikethrough>strike</strikethrough>",
			want:  "<del>strike</del>",
		},
		{
			name:  "empty-line",
			input: "Line 1<empty-line/>Line 2",
			want:  "Line 1<br/>Line 2",
		},
		{
			name:  "empty-line space",
			input: "Line 1<empty-line />Line 2",
			want:  "Line 1<br/>Line 2",
		},
		{
			name:  "note link",
			input: `Text <a type="note" l:href="#note_1">[1]</a>`,
			want:  `Text <a id="noteref_note_1" name="noteref_note_1"></a><a href="#note_1" class="noteref">[1]</a>`,
		},
		{
			name:  "nested formatting",
			input: `<emphasis>Italic <a l:href="#link">link</a></emphasis>`,
			want:  `<em>Italic <a href="#link">link</a></em>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseInlineContent(tt.input); got != tt.want {
				t.Errorf("ParseInlineContent() = %v, want %v", got, tt.want)
			}
		})
	}
}
