package fb2

import (
	"strings"
	"testing"
)

func TestParseSimpleFB2(t *testing.T) {
	// Simple FB2 document
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
	<description>
		<title-info>
			<genre>sf_history</genre>
			<author>
				<first-name>John</first-name>
				<last-name>Doe</last-name>
			</author>
			<book-title>Test Book</book-title>
			<lang>en</lang>
		</title-info>
	</description>
	<body>
		<section>
			<title>
				<p>Chapter 1</p>
			</title>
			<p>This is a test paragraph.</p>
		</section>
	</body>
</FictionBook>`

	parser := NewParser()
	fb2, err := parser.ParseBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if fb2.Description.TitleInfo.BookTitle != "Test Book" {
		t.Errorf("BookTitle = %v, want 'Test Book'", fb2.Description.TitleInfo.BookTitle)
	}

	if len(fb2.Description.TitleInfo.Author) != 1 {
		t.Fatalf("Author count = %v, want 1", len(fb2.Description.TitleInfo.Author))
	}

	author := fb2.Description.TitleInfo.Author[0]
	if author.FirstName != "John" || author.LastName != "Doe" {
		t.Errorf("Author = %v %v, want 'John Doe'", author.FirstName, author.LastName)
	}
}

func TestParseFB2WithBinary(t *testing.T) {
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
	<description>
		<title-info>
			<book-title>Test Book</book-title>
			<lang>en</lang>
		</title-info>
	</description>
	<body>
		<section>
			<p>Text before image</p>
			<image l:href="#cover.jpg"/>
			<p>Text after image</p>
		</section>
	</body>
	<binary id="cover.jpg" content-type="image/jpeg">
		iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==
	</binary>
</FictionBook>`

	parser := NewParser()
	fb2, err := parser.ParseBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if len(fb2.Binaries) != 1 {
		t.Fatalf("Binary count = %v, want 1", len(fb2.Binaries))
	}

	binary := fb2.Binaries[0]
	if binary.ID != "cover.jpg" {
		t.Errorf("Binary ID = %v, want 'cover.jpg'", binary.ID)
	}

	if binary.ContentType != "image/jpeg" {
		t.Errorf("Binary ContentType = %v, want 'image/jpeg'", binary.ContentType)
	}
}

func TestExtractMetadata(t *testing.T) {
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
	<description>
		<title-info>
			<genre>sf_history</genre>
			<genre>adventure</genre>
			<author>
				<first-name>John</first-name>
				<middle-name>Q</middle-name>
				<last-name>Doe</last-name>
			</author>
			<book-title>The Great Adventure</book-title>
			<annotation>
				<p>This is a great book about adventure.</p>
			</annotation>
			<keywords>adventure, fiction, action</keywords>
			<date value="2020-01-15"/>
			<lang>en</lang>
			<sequence name="Adventures" number="1"/>
		</title-info>
		<publish-info>
			<publisher>Test Publishers</publisher>
			<year>2020</year>
			<isbn>978-0-123456-78-9</isbn>
		</publish-info>
	</description>
	<body>
		<section>
			<title>
				<p>Chapter 1</p>
			</title>
			<p>Content</p>
		</section>
	</body>
</FictionBook>`

	parser := NewParser()
	fb2, err := parser.ParseBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	metadata, err := parser.ExtractMetadata(fb2)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}

	if metadata.Title != "The Great Adventure" {
		t.Errorf("Title = %v, want 'The Great Adventure'", metadata.Title)
	}

	if len(metadata.Authors) != 1 {
		t.Fatalf("Authors count = %v, want 1", len(metadata.Authors))
	}

	expectedAuthor := "John Q Doe"
	if metadata.Authors[0] != expectedAuthor {
		t.Errorf("Author = %v, want %v", metadata.Authors[0], expectedAuthor)
	}

	if metadata.Publisher != "Test Publishers" {
		t.Errorf("Publisher = %v, want 'Test Publishers'", metadata.Publisher)
	}

	if metadata.ISBN != "978-0-123456-78-9" {
		t.Errorf("ISBN = %v, want '978-0-123456-78-9'", metadata.ISBN)
	}

	if metadata.Series != "Adventures" {
		t.Errorf("Series = %v, want 'Adventures'", metadata.Series)
	}

	if metadata.SeriesIndex != 1 {
		t.Errorf("SeriesIndex = %v, want 1", metadata.SeriesIndex)
	}

	if len(metadata.Genres) != 2 {
		t.Fatalf("Genres count = %v, want 2", len(metadata.Genres))
	}

	if metadata.Annotation != "This is a great book about adventure." {
		t.Errorf("Annotation = %v, want 'This is a great book about adventure.'", metadata.Annotation)
	}

	// Keywords
	if len(metadata.Keywords) != 3 {
		t.Fatalf("Keywords count = %v, want 3", len(metadata.Keywords))
	}

	expectedKeywords := []string{"adventure", "fiction", "action"}
	for i, kw := range metadata.Keywords {
		if kw != expectedKeywords[i] {
			t.Errorf("Keyword[%d] = %v, want %v", i, kw, expectedKeywords[i])
		}
	}
}

func TestTransformToHTML(t *testing.T) {
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
	<description>
		<title-info>
			<author>
				<first-name>John</first-name>
				<last-name>Doe</last-name>
			</author>
			<book-title>Test Book</book-title>
			<lang>en</lang>
		</title-info>
	</description>
	<body>
		<section>
			<title>
				<p>Chapter 1</p>
			</title>
			<p>This is the first chapter.</p>
		</section>
	</body>
</FictionBook>`

	transformer := NewTransformer()
	html, _, metadata, err := transformer.ConvertBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ConvertBytes() error = %v", err)
	}

	if html == "" {
		t.Error("HTML is empty")
	}

	if !strings.Contains(html, "<html") {
		t.Error("HTML doesn't contain <html> tag")
	}

	if !strings.Contains(html, "Chapter 1") {
		t.Error("HTML doesn't contain chapter title")
	}

	if !strings.Contains(html, "This is the first chapter") {
		t.Error("HTML doesn't contain chapter content")
	}

	if metadata == nil {
		t.Error("Metadata is nil")
	} else if metadata.Title != "Test Book" {
		t.Errorf("Metadata title = %v, want 'Test Book'", metadata.Title)
	}
}

func TestTransformWithNoTOC(t *testing.T) {
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
	<description>
		<title-info>
			<book-title>Test Book</book-title>
			<lang>en</lang>
		</title-info>
	</description>
	<body>
		<section>
			<p>Content</p>
		</section>
	</body>
</FictionBook>`

	transformer := NewTransformer()
	transformer.NoInlineTOC = true

	html, _, _, err := transformer.ConvertBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ConvertBytes() error = %v", err)
	}

	if strings.Contains(html, "<ul>") {
		// Should not have TOC
		t.Error("HTML contains TOC when NoInlineTOC is true")
	}
}

func TestHTMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello & goodbye", "Hello &amp; goodbye"},
		{"<tag>", "&lt;tag&gt;"},
		{"\"quoted\"", "&quot;quoted&quot;"},
		{"'apostrophe'", "&apos;apostrophe&apos;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := htmlEscape(tt.input)
			if got != tt.want {
				t.Errorf("htmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal_file.txt", "normal_file.txt"},
		{"file:with:bad:chars.txt", "file_with_bad_chars.txt"},
		{"file<>with\"bad|chars?.txt", "file_with_bad_chars_.txt"},
		{"file/with\\slashes.txt", "file_with_slashes.txt"},
		{"...leading...", "leading"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatAuthorName(t *testing.T) {
	tests := []struct {
		author Author
		want    string
	}{
		{
			author: Author{FirstName: "John", LastName: "Doe"},
			want:    "John Doe",
		},
		{
			author: Author{FirstName: "John", MiddleName: "Q", LastName: "Doe"},
			want:    "John Q Doe",
		},
		{
			author: Author{Nickname: "JDoe"},
			want:    "JDoe",
		},
		{
			author: Author{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatAuthorName(tt.author)
			if got != tt.want {
				t.Errorf("formatAuthorName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"one", []string{"one"}},
		{"one, two", []string{"one", "two"}},
		{"one; two", []string{"one", "two"}},
		{"one, two; three", []string{"one", "two", "three"}},
		{"  one  ,  two  ", []string{"one", "two"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseKeywords(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseKeywords(%q) length = %v, want %v", tt.input, len(got), len(tt.want))
				return
			}
			for i, kw := range got {
				if kw != tt.want[i] {
					t.Errorf("parseKeywords(%q)[%d] = %v, want %v", tt.input, i, kw, tt.want[i])
				}
			}
		})
	}
}

// Test with actual file I/O would require sample files
// For now, we test the in-memory operations

func TestImageDataToDataURL(t *testing.T) {
	// Simple FB2 document with embedded image
	fb2Data := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
    <description>
        <title-info>
            <book-title>Test Book</book-title>
            <lang>en</lang>
        </title-info>
    </description>
    <body>
        <section>
            <p>Text before image</p>
            <image l:href="#cover.jpg"/>
            <p>Text after image</p>
        </section>
    </body>
    <binary id="cover.jpg" content-type="image/jpeg">
        /9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAALCAACAgBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAACv/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AT//Z
    </binary>
</FictionBook>`

	transformer := NewTransformer()
	html, _, _, err := transformer.ConvertBytes([]byte(fb2Data))
	if err != nil {
		t.Fatalf("ConvertBytes() error = %v", err)
	}

	// Verify data URL is used
	if !strings.Contains(html, "data:image/jpeg;base64,") {
		t.Error("HTML doesn't contain data URL")
	}

	// Verify original filename is NOT used
	if strings.Contains(html, "src=\"cover.jpg\"") {
		t.Error("HTML should use data URL, not filename")
	}

	// Verify img tag is present
	if !strings.Contains(html, "<img") {
		t.Error("HTML doesn't contain img tag")
	}
}
