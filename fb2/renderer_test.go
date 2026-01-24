package fb2

import (
	"strings"
	"testing"
)

// TestRenderSection tests rendering of a section
func TestRenderSection(t *testing.T) {
	transformer := NewTransformer()
	transformer.MOBIMode = true

	section := Section{
		ID: "test-section",
		Title: &Title{
			P: []P{{Text: "Section Title"}},
		},
		Paragraphs: []P{
			{Text: "Paragraph 1"},
			{Text: "Paragraph 2"},
		},
	}

	html := transformer.renderSection(section, 1)

	if !strings.Contains(html, "<a id=\"test-section\"></a>") {
		t.Error("HTML missing section ID anchor")
	}

	if !strings.Contains(html, "<h2>Section Title<br/>\n</h2>") {
		t.Errorf("HTML missing title. Got: %s", html)
	}

	if !strings.Contains(html, "<p class=\"paragraph\">Paragraph 1</p>") {
		t.Error("HTML missing paragraph 1")
	}
}

// TestRenderTable tests rendering of a table
func TestRenderTable(t *testing.T) {
	transformer := NewTransformer()
	transformer.MOBIMode = true

	table := Table{
		Rows: []TR{
			{
				Cells: []TableCell{
					{Content: "Cell 1"},
					{Content: "Cell 2"},
				},
			},
		},
	}

	html := transformer.renderTable(table)

	if !strings.Contains(html, "<table>") {
		t.Error("HTML missing table tag")
	}

	if !strings.Contains(html, "<td>Cell 1</td>") {
		t.Error("HTML missing Cell 1")
	}
}

// TestRenderEpigraph tests rendering of an epigraph
func TestRenderEpigraph(t *testing.T) {
	transformer := NewTransformer()
	transformer.MOBIMode = true

	epigraph := Epigraph{
		Authors: []Author{{LastName: "Author"}},
		Content: []ContentNode{{Content: "Quote text"}},
	}

	html := transformer.renderEpigraph(epigraph)

	if !strings.Contains(html, "<blockquote class=\"epigraph\">") {
		t.Error("HTML missing blockquote class")
	}

	if !strings.Contains(html, "Quote text") {
		t.Error("HTML missing content")
	}

	if !strings.Contains(html, "<em>Author</em>") {
		t.Error("HTML missing author")
	}
}

// TestRenderImage tests rendering of an image
func TestRenderImage(t *testing.T) {
	parser := NewParser()
	parser.imageData["img1.jpg"] = []byte("fake data")
	parser.imageTypes["img1.jpg"] = "image/jpeg"

	transformer := NewTransformer()
	transformer.SetParser(parser)
	transformer.MOBIMode = true

	// Test regular image (href)
	img1 := Image{Href: "#img1.jpg", Alt: "Image Alt"}
	html1 := transformer.renderImage(img1)

	if !strings.Contains(html1, "src=\"img1.jpg\"") {
		t.Errorf("Expected src to be 'img1.jpg', got: %s", html1)
	}
	if !strings.Contains(html1, "alt=\"Image Alt\"") {
		t.Error("Missing alt tag")
	}

	// Test Data URL mode
	transformer.UseDataURLs = true
	html2 := transformer.renderImage(img1)

	if !strings.Contains(html2, "data:image/jpeg;base64,ZmFrZSBkYXRh") { // "fake data" base64
		t.Errorf("Expected data URL, got: %s", html2)
	}
}
