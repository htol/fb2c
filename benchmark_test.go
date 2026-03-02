package fb2c

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/internal/mapper"
	"github.com/htol/fb2c/mobi"
)

func TestPerformanceBreakdown(t *testing.T) {
	// 1. Generate a large FB2 content (approx 2MB of text)
	fb2Content := generateLargeFB2(5000)
	t.Logf("Generated FB2 size: %d bytes", len(fb2Content))

	// 2. Parse
	start := time.Now()
	parser := fb2.NewParser()
	fb2Doc, err := parser.ParseBytes(fb2Content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	parseDuration := time.Since(start)
	t.Logf("Parse Duration: %v", parseDuration)

	// 3. Metadata & TOC
	start = time.Now()
	metadata, _ := fb2.ExtractMetadata(fb2Doc, parser)
	tocData, _ := parser.ExtractTOC(fb2Doc)
	metaDuration := time.Since(start)
	t.Logf("Metadata/TOC Extraction Duration: %v", metaDuration)

	// 4. Transform to HTML
	start = time.Now()
	transformer := fb2.NewTransformer()
	transformer.MOBIMode = true
	transformer.SetParser(parser)
	result, err := transformer.Transform(fb2Doc)
	html := result.HTML
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	transformDuration := time.Since(start)
	t.Logf("Transform Duration: %v", transformDuration)

	// 5. Create OPF Book
	start = time.Now()
	// converter := NewConverter()
	book, err := mapper.FromFB2(metadata, html, tocData, fb2Doc)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	createBookDuration := time.Since(start)
	t.Logf("Create OPF Book Duration: %v", createBookDuration)

	// 6. Write MOBI 6 (palmDOC)
	start = time.Now()
	var buf bytes.Buffer
	mobiOpts := mobi.DefaultWriteOptions()
	mobiOpts.CompressionType = mobi.PalmDOCCompression
	err = mobi.ConvertOEBToMOBIWithOptions(book, &buf, mobiOpts)
	if err != nil {
		t.Fatalf("Write MOBI failed: %v", err)
	}
	writeDuration := time.Since(start)
	t.Logf("Write MOBI6 Duration: %v", writeDuration)
	t.Logf("Output MOBI size: %d bytes", buf.Len())
}

func generateLargeFB2(paragraphs int) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description>
	<title-info>
		<genre>fantasy</genre>
		<author><first-name>Test</first-name><last-name>Author</last-name></author>
		<book-title>Large Test Book</book-title>
		<lang>en</lang>
	</title-info>
</description>
<body>
<section>
	<title><p>Chapter 1</p></title>
`)

	lorem := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

	for i := 0; i < paragraphs; i++ {
		buf.WriteString(fmt.Sprintf("<p>%d %s</p>\n", i, lorem))
	}

	buf.WriteString(`
</section>
</body>
</FictionBook>`)
	return buf.Bytes()
}

// Add createOPFBook visibility via export for test if needed, or copy/paste logic.
// Since createOPFBook is private, I'll access it because I am in package fb2c_test or fb2c?

func TestPerformanceRealFile(t *testing.T) {
	// Use the constant that already exists
	filePath := testRefFB2
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Skipf("Skipping test: file %s not found", filePath)
	}

	t.Logf("Reading file: %s", filePath)
	fb2Content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("File size: %d bytes", len(fb2Content))

	// 2. Parse
	start := time.Now()
	parser := fb2.NewParser()
	fb2Doc, err := parser.ParseBytes(fb2Content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	parseDuration := time.Since(start)
	t.Logf("Parse Duration: %v", parseDuration)

	// 3. Metadata & TOC
	start = time.Now()
	metadata, _ := fb2.ExtractMetadata(fb2Doc, parser)
	tocData, _ := parser.ExtractTOC(fb2Doc)
	metaDuration := time.Since(start)
	t.Logf("Metadata/TOC Extraction Duration: %v", metaDuration)

	// 4. Transform to HTML
	start = time.Now()
	transformer := fb2.NewTransformer()
	transformer.MOBIMode = true
	transformer.SetParser(parser)
	result, err := transformer.Transform(fb2Doc)
	html := result.HTML
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	transformDuration := time.Since(start)
	t.Logf("Transform Duration: %v", transformDuration)

	// 5. Create OPF Book
	start = time.Now()
	// converter := NewConverter()
	book, err := mapper.FromFB2(metadata, html, tocData, fb2Doc)
	if err != nil {
		t.Fatalf("Map failed: %v", err)
	}
	createBookDuration := time.Since(start)
	t.Logf("Create OPF Book Duration: %v", createBookDuration)

	// 6. Write MOBI 6 (palmDOC)
	start = time.Now()
	var buf bytes.Buffer
	mobiOpts := mobi.DefaultWriteOptions()
	mobiOpts.CompressionType = mobi.PalmDOCCompression
	err = mobi.ConvertOEBToMOBIWithOptions(book, &buf, mobiOpts)
	if err != nil {
		t.Fatalf("Write MOBI failed: %v", err)
	}
	writeDuration := time.Since(start)
	t.Logf("Write MOBI6 Duration: %v", writeDuration)
	t.Logf("Output MOBI size: %d bytes", buf.Len())
}
