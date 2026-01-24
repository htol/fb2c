package fb2

import (
	"testing"
)

// MockImageProvider is a mock implementation of ImageProvider
type MockImageProvider struct {
	data  map[string][]byte
	types map[string]string
}

func (m *MockImageProvider) GetImageData() map[string][]byte {
	return m.data
}

func (m *MockImageProvider) GetImageType(binaryID string) string {
	if t, ok := m.types[binaryID]; ok {
		return t
	}
	return "image/jpeg"
}

func TestExtractMetadataIsolated(t *testing.T) {
	// Setup simple FB2 structure
	fb2 := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: "Test Title",
				Author: []Author{
					{FirstName: "John", LastName: "Doe"},
				},
				Genre:    []string{"fantasy"},
				Language: "en",
				Annotation: &TextContainer{
					Text: "Test annotation",
				},
			},
		},
	}

	// Setup mock provider
	mockIP := &MockImageProvider{
		data:  make(map[string][]byte),
		types: make(map[string]string),
	}

	// 1. Basic Metadata Extraction
	meta, err := ExtractMetadata(fb2, mockIP)
	if err != nil {
		t.Fatalf("ExtractMetadata failed: %v", err)
	}

	if meta.Title != "Test Title" {
		t.Errorf("Title = %s, want 'Test Title'", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "John Doe" {
		t.Errorf("Authors = %v, want ['John Doe']", meta.Authors)
	}
	if meta.Annotation != "Test annotation" {
		t.Errorf("Annotation = '%s', want 'Test annotation'", meta.Annotation)
	}

	// 2. Cover Image Extraction
	// Add cover info to FB2
	fb2.Description.TitleInfo.Coverpage = Coverpage{
		PrimaryImage: ImageRef{
			Href: "#cover.jpg",
		},
	}
	// Add cover binary to mock provider
	mockIP.data["cover.jpg"] = []byte("dummy image data")
	mockIP.types["cover.jpg"] = "image/jpeg"

	meta, err = ExtractMetadata(fb2, mockIP)
	if err != nil {
		t.Fatalf("ExtractMetadata with cover failed: %v", err)
	}

	if meta.CoverID != "cover.jpg" {
		t.Errorf("CoverID = %s, want 'cover.jpg'", meta.CoverID)
	}
	if string(meta.Cover) != "dummy image data" {
		t.Error("Cover content mismatch")
	}
	if meta.CoverExt != ".jpg" {
		t.Errorf("CoverExt = %s, want '.jpg'", meta.CoverExt)
	}
}

func TestFormatAuthorNameDetailed(t *testing.T) {
	tests := []struct {
		name   string
		author Author
		want   string
	}{
		{"Full", Author{FirstName: "First", MiddleName: "M", LastName: "Last"}, "First M Last"},
		{"NoMiddle", Author{FirstName: "First", LastName: "Last"}, "First Last"},
		{"OnlyNick", Author{Nickname: "Nick"}, "Nick"},
		{"Mixed", Author{FirstName: "First", Nickname: "Nick"}, "First"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAuthorName(tt.author)
			if got != tt.want {
				t.Errorf("formatAuthorName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseKeywordsDetailed(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"  key1 , key2; key3\nkey4  ", []string{"key1", "key2", "key3", "key4"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseKeywords(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseKeywords() length = %d, want %d", len(got), len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("keyword[%d] = %s, want %s", i, v, tt.want[i])
				}
			}
		})
	}
}
