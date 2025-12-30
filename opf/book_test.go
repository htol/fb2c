package opf

import (
	"testing"
	"time"
)

func TestNewOEBBook(t *testing.T) {
	book := NewOEBBook()

	if book == nil {
		t.Fatal("NewOEBBook() returned nil")
	}

	if book.Manifest == nil {
		t.Error("Manifest should be initialized")
	}

	if book.Spine == nil {
		t.Error("Spine should be initialized")
	}

	if len(book.Manifest) != 0 {
		t.Error("New book should have empty manifest")
	}

	if len(book.Spine) != 0 {
		t.Error("New book should have empty spine")
	}
}

func TestAddResource(t *testing.T) {
	book := NewOEBBook()

	// Add a resource
	data := []byte("test content")
	res := book.AddResource("test_id", "test.html", "text/html", data)

	if res == nil {
		t.Fatal("AddResource() returned nil")
	}

	if res.ID != "test_id" {
		t.Errorf("ID = %v, want 'test_id'", res.ID)
	}

	if res.Href != "test.html" {
		t.Errorf("Href = %v, want 'test.html'", res.Href)
	}

	if res.MediaType != "text/html" {
		t.Errorf("MediaType = %v, want 'text/html'", res.MediaType)
	}

	if string(res.Data) != "test content" {
		t.Errorf("Data = %v, want 'test content'", string(res.Data))
	}

	// Check it's in manifest
	if len(book.Manifest) != 1 {
		t.Errorf("Manifest length = %v, want 1", len(book.Manifest))
	}

	retrieved, ok := book.GetResource("test_id")
	if !ok {
		t.Error("Resource not found in manifest")
	}

	if retrieved.ID != res.ID {
		t.Errorf("Retrieved ID = %v, want %v", retrieved.ID, res.ID)
	}
}

func TestAddToSpine(t *testing.T) {
	book := NewOEBBook()

	book.AddToSpine("res1")
	book.AddToSpine("res2")

	if len(book.Spine) != 2 {
		t.Fatalf("Spine length = %v, want 2", len(book.Spine))
	}

	if book.Spine[0] != "res1" {
		t.Errorf("Spine[0] = %v, want 'res1'", book.Spine[0])
	}

	if book.Spine[1] != "res2" {
		t.Errorf("Spine[1] = %v, want 'res2'", book.Spine[1])
	}
}

func TestNewAuthor(t *testing.T) {
	tests := []struct {
		name     string
		first    string
		middle   string
		last     string
		nickname string
		wantFull string
		wantSort string
	}{
		{
			name:     "full name",
			first:    "John",
			middle:   "Q",
			last:     "Doe",
			nickname: "",
			wantFull: "John Q Doe",
			wantSort: "Doe, John Q",
		},
		{
			name:     "first and last only",
			first:    "Jane",
			middle:   "",
			last:     "Smith",
			nickname: "",
			wantFull: "Jane Smith",
			wantSort: "Smith, Jane",
		},
		{
			name:     "nickname only",
			first:    "",
			middle:   "",
			last:     "",
			nickname: "Writer",
			wantFull: "Writer",
			wantSort: "Writer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			author := NewAuthor(tt.first, tt.middle, tt.last, tt.nickname)

			if author.FullName != tt.wantFull {
				t.Errorf("FullName = %v, want %v", author.FullName, tt.wantFull)
			}

			if author.SortName != tt.wantSort {
				t.Errorf("SortName = %v, want %v", author.SortName, tt.wantSort)
			}
		})
	}
}

func TestTOCEntry(t *testing.T) {
	root := &TOCEntry{
		ID:    "root",
		Label: "Root",
		Href:  "root.html",
		Level: 0,
	}

	child1 := root.AddChild("child1", "Chapter 1", "ch1.html")
	child2 := root.AddChild("child2", "Chapter 2", "ch2.html")

	if len(root.Children) != 2 {
		t.Fatalf("Children length = %v, want 2", len(root.Children))
	}

	if child1.ID != "child1" {
		t.Errorf("Child1 ID = %v, want 'child1'", child1.ID)
	}

	if child1.Level != 1 {
		t.Errorf("Child1 Level = %v, want 1", child1.Level)
	}

	if child2.Label != "Chapter 2" {
		t.Errorf("Child2 Label = %v, want 'Chapter 2'", child2.Label)
	}

	// Test flatten
	flat := root.Flatten()
	if len(flat) != 3 {
		t.Errorf("Flatten() length = %v, want 3", len(flat))
	}

	// Test max depth
	maxDepth := root.MaxDepth()
	if maxDepth != 1 {
		t.Errorf("MaxDepth() = %v, want 1", maxDepth)
	}

	// Add nested child
	child1.AddChild("child1_1", "Section 1.1", "ch1s1.html")
	maxDepth = root.MaxDepth()
	if maxDepth != 2 {
		t.Errorf("MaxDepth() with nested = %v, want 2", maxDepth)
	}
}

func TestGenerateOPF(t *testing.T) {
	book := NewOEBBook()

	// Set metadata
	book.Metadata = Metadata{
		Title:       "Test Book",
		Language:    "en",
		Publisher:   "Test Publisher",
		ISBN:        "978-0-123456-78-9",
		Year:        "2024",
		PubDate:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Series:      "Test Series",
		SeriesIndex: 1,
		Genres:      []string{"Fiction", "Adventure"},
		Annotation:  "A test book annotation",
		CoverID:     "cover.jpg",
	}
	book.Metadata.Authors = []Author{
		NewAuthor("John", "Q", "Doe", ""),
	}

	// Add resources
	book.AddResource("html", "content.html", "application/xhtml+xml", []byte("<html></html>"))
	book.AddResource("cover", "cover.jpg", "image/jpeg", []byte("fake image"))
	book.AddResource("ncx", "toc.ncx", "application/x-dtbncx+xml", []byte("ncx"))

	book.AddToSpine("html")

	// Generate OPF
	opf, err := book.GenerateOPF()
	if err != nil {
		t.Fatalf("GenerateOPF() error = %v", err)
	}

	opfStr := string(opf)

	// Check for required elements
	requiredStrings := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<package`,
		`version="2.0"`,
		`<metadata`,
		`<dc:title>Test Book</dc:title>`,
		`<dc:creator`,
		`<dc:publisher>Test Publisher</dc:publisher>`,
		`<dc:language>en</dc:language>`,
		`<manifest>`,
		`<spine`,
		`<item id="html"`,
		`<itemref idref="html"`,
	}

	for _, required := range requiredStrings {
		if !contains(opfStr, required) {
			t.Errorf("OPF missing required string: %s", required)
		}
	}

	t.Logf("Generated OPF:\n%s", opfStr)
}

func TestGenerateNCX(t *testing.T) {
	book := NewOEBBook()
	book.Metadata = Metadata{
		Title: "Test Book",
	}

	// Build TOC
	book.TOC = TOCEntry{
		ID:    "root",
		Label: "Root",
		Level: 0,
		Children: []*TOCEntry{
			{
				ID:    "ch1",
				Label: "Chapter 1",
				Href:  "ch1.html",
				Level: 1,
			},
			{
				ID:    "ch2",
				Label: "Chapter 2",
				Href:  "ch2.html",
				Level: 1,
				Children: []*TOCEntry{
					{
						ID:    "ch2s1",
						Label: "Section 2.1",
						Href:  "ch2s1.html",
						Level: 2,
					},
				},
			},
		},
	}

	// Generate NCX
	ncx, err := book.GenerateNCX()
	if err != nil {
		t.Fatalf("GenerateNCX() error = %v", err)
	}

	ncxStr := string(ncx)

	// Check for required elements
	requiredStrings := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<ncx`,
		`version="2005-1"`,
		`<head>`,
		`<docTitle>`,
		`<text>Test Book</text>`,
		`<navMap>`,
		`<navPoint id="ch1"`,
		`<text>Chapter 1</text>`,
		`content src="ch1.html"`,
	}

	for _, required := range requiredStrings {
		if !contains(ncxStr, required) {
			t.Errorf("NCX missing required string: %s", required)
		}
	}

	t.Logf("Generated NCX:\n%s", ncxStr)
}

func TestHTMLProcessor(t *testing.T) {
	processor := NewHTMLProcessor()

	tests := []struct {
		name     string
		input    string
		contains []string
		notContains []string
	}{
		{
			name: "remove XML declaration",
			input: `<?xml version="1.0"?><html><body>Test</body></html>`,
			contains: []string{"<html>", "Test"},
			notContains: []string{"<?xml"},
		},
		{
			name: "convert paragraph divs",
			input: `<div class="paragraph">Test paragraph</div>`,
			contains: []string{`<p>Test paragraph</p>`},
			notContains: []string{`<div class="paragraph">`},
		},
		{
			name: "remove empty elements and convert div to p",
			input: `<p></p><div>Content</div><p></p>`,
			contains: []string{`<p>Content</p>`},
			notContains: []string{`<p></p>`, `<div>Content</div>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.Process(tt.input)

			for _, required := range tt.contains {
				if !contains(result, required) {
					t.Errorf("Result missing required string: %s\nGot: %s", required, result)
				}
			}

			for _, notWanted := range tt.notContains {
				if contains(result, notWanted) {
					t.Errorf("Result contains unwanted string: %s\nGot: %s", notWanted, result)
				}
			}
		})
	}
}

func TestGenerateTitlePage(t *testing.T) {
	processor := NewHTMLProcessor()

	metadata := Metadata{
		Title:       "Test Book",
		Publisher:   "Test Publisher",
		Year:        "2024",
		ISBN:        "978-0-123456-78-9",
		Series:      "Test Series",
		SeriesIndex: 3,
	}
	metadata.Authors = []Author{
		NewAuthor("John", "", "Doe", ""),
		NewAuthor("Jane", "", "Smith", ""),
	}

	titlePage := processor.GenerateTitlePage(metadata)

	// Check for required elements
	requiredStrings := []string{
		`<div style="text-align: center`,
		`<h1>Test Book</h1>`,
		`<h2>John Doe</h2>`,
		`<h2>Jane Smith</h2>`,
		`<h3>Test Series (#3)</h3>`,
		`<p>Test Publisher</p>`,
		`<p>2024</p>`,
		`<p>ISBN: 978-0-123456-78-9</p>`,
	}

	for _, required := range requiredStrings {
		if !contains(titlePage, required) {
			t.Errorf("Title page missing required string: %s\nGot: %s", required, titlePage)
		}
	}

	t.Logf("Generated title page:\n%s", titlePage)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findInString(s, substr) >= 0
}

func findInString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
