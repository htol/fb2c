package kf8

import (
	"testing"
)

func TestNewSkeleton(t *testing.T) {
	skel := NewSkeleton()

	if skel == nil {
		t.Fatal("NewSkeleton() returned nil")
	}

	if skel.Chunks == nil {
		t.Error("Chunks not initialized")
	}

	if len(skel.Chunks) != 0 {
		t.Error("New skeleton should have no chunks")
	}

	if skel.AIDCounter != 0 {
		t.Errorf("AIDCounter = %v, want 0", skel.AIDCounter)
	}
}

func TestChunkHTML(t *testing.T) {
	skel := NewSkeleton()

	tests := []struct {
		name    string
		html    string
		wantErr bool
		minChunks int
	}{
		{
			name:    "empty",
			html:    "",
			wantErr: false,
			minChunks: 0,
		},
		{
			name:    "small",
			html:    "<html><body>Hello World</body></html>",
			wantErr: false,
			minChunks: 1,
		},
		{
			name:    "large",
			html:    generateLargeHTML(20000),
			wantErr: false,
			minChunks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := skel.ChunkHTML(tt.html)
			if (err != nil) != tt.wantErr {
				t.Errorf("ChunkHTML() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(skel.Chunks) < tt.minChunks {
				t.Errorf("Got %v chunks, want at least %v", len(skel.Chunks), tt.minChunks)
			}

			// Verify chunks have valid properties
			for i, chunk := range skel.Chunks {
				if chunk.ID != i {
					t.Errorf("Chunk %v has ID %v", i, chunk.ID)
				}
				if chunk.AID == "" {
					t.Errorf("Chunk %v has empty AID", i)
				}
				if len(chunk.Content) == 0 {
					t.Errorf("Chunk %v has empty content", i)
				}
			}
		})
	}
}

func TestGenerateAID(t *testing.T) {
	skel := NewSkeleton()

	// Generate several AIDs and verify uniqueness
	aids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		aid := skel.generateAID()
		if aid == "" {
			t.Error("generateAID() returned empty string")
		}
		if aids[aid] {
			t.Errorf("Duplicate AID generated: %s", aid)
		}
		aids[aid] = true
	}
}

func TestGetChunkByAID(t *testing.T) {
	skel := NewSkeleton()
	skel.ChunkHTML("<html><body>Test</body></html>")

	if len(skel.Chunks) == 0 {
		t.Fatal("No chunks generated")
	}

	chunk := skel.Chunks[0]

	found, ok := skel.GetChunkByAID(chunk.AID)
	if !ok {
		t.Error("GetChunkByAID() failed to find chunk")
	}

	if found.ID != chunk.ID {
		t.Errorf("Found chunk ID = %v, want %v", found.ID, chunk.ID)
	}

	// Try to find non-existent AID
	_, ok = skel.GetChunkByAID("nonexistent")
	if ok {
		t.Error("GetChunkByAID() found non-existent chunk")
	}
}

func TestGetChunkByOffset(t *testing.T) {
	skel := NewSkeleton()
	skel.ChunkHTML("<html><body>Test content here</body></html>")

	if len(skel.Chunks) == 0 {
		t.Fatal("No chunks generated")
	}

	chunk := skel.Chunks[0]

	// Test with offset within chunk
	found, ok := skel.GetChunkByOffset(chunk.Offset + 1)
	if !ok {
		t.Error("GetChunkByOffset() failed to find chunk")
	}

	if found.ID != chunk.ID {
		t.Errorf("Found chunk ID = %v, want %v", found.ID, chunk.ID)
	}

	// Test with offset outside any chunk
	_, ok = skel.GetChunkByOffset(999999)
	if ok {
		t.Error("GetChunkByOffset() found chunk at invalid offset")
	}
}

func TestBuildHierarchy(t *testing.T) {
	skel := NewSkeleton()
	skel.ChunkHTML(generateLargeHTML(20000))

	skel.BuildHierarchy()

	// Check that hierarchy was built
	for _, chunk := range skel.Chunks {
		depth := skel.GetChunkDepth(chunk)
		if depth < 0 {
			t.Errorf("Invalid depth: %v", depth)
		}
	}
}

func TestNewFDST(t *testing.T) {
	fdst := NewFDST()

	if fdst == nil {
		t.Fatal("NewFDST() returned nil")
	}

	if fdst.Entries == nil {
		t.Error("Entries not initialized")
	}
}

func TestFDSTAddEntry(t *testing.T) {
	fdst := NewFDST()

	fdst.AddEntry(0, 100)
	fdst.AddEntry(100, 200)

	if fdst.Header.NumEntries != 2 {
		t.Errorf("NumEntries = %v, want 2", fdst.Header.NumEntries)
	}

	if len(fdst.Entries) != 2 {
		t.Errorf("Entries length = %v, want 2", len(fdst.Entries))
	}
}

func TestFDSTValidate(t *testing.T) {
	tests := []struct {
		name    string
		entries []FDSTEntry
		wantErr bool
	}{
		{
			name:    "valid",
			entries: []FDSTEntry{{0, 100}, {100, 200}},
			wantErr: false,
		},
		{
			name:    "overlapping",
			entries: []FDSTEntry{{0, 150}, {100, 200}},
			wantErr: true,
		},
		{
			name:    "inverted",
			entries: []FDSTEntry{{100, 50}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fdst := NewFDST()
			fdst.Entries = tt.entries
			fdst.Header.NumEntries = uint32(len(tt.entries))

			err := fdst.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFDSTMergeEntries(t *testing.T) {
	fdst := NewFDST()
	fdst.AddEntry(0, 100)
	fdst.AddEntry(105, 200) // Small gap (5 bytes)
	fdst.AddEntry(210, 300) // Gap (10 bytes)
	fdst.AddEntry(310, 400)

	initialCount := len(fdst.Entries)

	// Merge with max gap of 8
	fdst.MergeEntries(8)

	if len(fdst.Entries) >= initialCount {
		t.Errorf("MergeEntries() didn't reduce count: %v -> %v", initialCount, len(fdst.Entries))
	}
}

func TestNewFlowManager(t *testing.T) {
	fm := NewFlowManager()

	if fm == nil {
		t.Fatal("NewFlowManager() returned nil")
	}

	if fm.flows == nil {
		t.Error("Flows not initialized")
	}

	if fm.flowMap == nil {
		t.Error("FlowMap not initialized")
	}
}

func TestCreateFlow(t *testing.T) {
	fm := NewFlowManager()

	flow := fm.CreateFlow("test", FlowTypeHTML, "<html>test</html>")

	if flow == nil {
		t.Fatal("CreateFlow() returned nil")
	}

	if flow.ID != "test" {
		t.Errorf("Flow ID = %v, want 'test'", flow.ID)
	}

	if flow.Type != FlowTypeHTML {
		t.Errorf("Flow type = %v, want %v", flow.Type, FlowTypeHTML)
	}

	if flow.Index != 0 {
		t.Errorf("Flow index = %v, want 0", flow.Index)
	}

	// Verify flow is in map
	found, ok := fm.GetFlow("test")
	if !ok {
		t.Error("Flow not found in map")
	}

	if found.ID != flow.ID {
		t.Errorf("Found flow ID = %v, want %v", found.ID, flow.ID)
	}
}

func TestParseResourceType(t *testing.T) {
	tests := []struct {
		href string
		want string
	}{
		{"style.css", "css"},
		{"image.jpg", "image"},
		{"image.jpeg", "image"},
		{"image.png", "image"},
		{"image.gif", "image"},
		{"font.ttf", "font"},
		{"font.otf", "font"},
		{"graphic.svg", "svg"},
		{"document.html", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			got := ParseResourceType(tt.href)
			if got != tt.want {
				t.Errorf("ParseResourceType(%q) = %v, want %v", tt.href, got, tt.want)
			}
		})
	}
}

func TestIsInternalLink(t *testing.T) {
	tests := []struct {
		link string
		want bool
	}{
		{"#chapter1", true},
		{"chapter2.html", true},
		{"http://example.com", false},
		{"https://example.com", false},
		{"mailto:test@example.com", false},
		{"ftp://example.com/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.link, func(t *testing.T) {
			got := IsInternalLink(tt.link)
			if got != tt.want {
				t.Errorf("IsInternalLink(%q) = %v, want %v", tt.link, got, tt.want)
			}
		})
	}
}

// Helper function to generate large HTML
func generateLargeHTML(size int) string {
	html := "<html><body>"
	for len(html) < size {
		html += "<p>This is a test paragraph with some content to fill space.</p>"
	}
	html += "</body></html>"
	return html[:size]
}
