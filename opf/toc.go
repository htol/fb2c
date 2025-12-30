// Package opf provides TOC (Table of Contents) generation.
package opf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// NCXDoc represents a Navigation Control XML (NCX) document
type NCXDoc struct {
	XMLName   xml.Name `xml:"ncx"`
	Version   string   `xml:"version,attr"`
	XMLNS     string   `xml:"xmlns,attr"`
	Head      NCXHead `xml:"head"`
	DocTitle  NCXText `xml:"docTitle"`
	NavMap    NCXNavMap `xml:"navMap"`
}

// NCXHead contains NCX metadata
type NCXHead struct {
	XMLName    xml.Name `xml:"head"`
	MetaItems []NCXMeta `xml:"meta"`
}

// NCXMeta represents a meta element in NCX head
type NCXMeta struct {
	XMLName xml.Name `xml:"meta"`
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// NCXText represents text content
type NCXText struct {
	Text string `xml:"text"`
}

// NCXNavMap represents the navigation map
type NCXNavMap struct {
	NavPoints []NCXNavPoint `xml:"navPoint"`
}

// NCXNavPoint represents a navigation point (TOC entry)
type NCXNavPoint struct {
	ID    string   `xml:"id,attr"`
	PlayOrder int  `xml:"playOrder,attr"`
	NavLabel  NCXText `xml:"navLabel>text"`
	Content   NCXContent `xml:"content"`
	Children  []NCXNavPoint `xml:"navPoint"`
}

// NCXContent represents content reference
type NCXContent struct {
	Src string `xml:"src,attr"`
}

// GenerateNCX creates an NCX document from OEBBook TOC
func (b *OEBBook) GenerateNCX() ([]byte, error) {
	// Flatten TOC to get ordered list
	flatEntries := b.TOC.Flatten()

	// Build nav map recursively from TOC
	navMap := b.buildNCXNavMap(&b.TOC, flatEntries)

	// Create NCX document
	ncx := NCXDoc{
		Version: "2005-1",
		XMLNS:   "http://www.daisy.org/z3986/2005/ncx/",
		Head: NCXHead{
			MetaItems: []NCXMeta{
				{Name: "dtb:uid", Content: "book_id"},
				{Name: "dtb:depth", Content: fmt.Sprintf("%d", b.TOC.MaxDepth() + 1)},
				{Name: "dtb:totalPageCount", Content: "0"},
				{Name: "dtb:maxPageNumber", Content: "0"},
			},
		},
		DocTitle: NCXText{
			Text: b.Metadata.Title,
		},
		NavMap: navMap,
	}

	// Marshal to XML with indentation
	data, err := xml.MarshalIndent(ncx, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal NCX: %w", err)
	}

	// Add XML declaration
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
`)
	buf.Write(data)

	return buf.Bytes(), nil
}

// buildNCXNavMap builds NCX navigation map from TOC
func (b *OEBBook) buildNCXNavMap(toc *TOCEntry, flatEntries []*TOCEntry) NCXNavMap {
	playOrder := 1
	return NCXNavMap{
		NavPoints: b.buildNCXNavPoints(toc, &playOrder, flatEntries),
	}
}

// buildNCXNavPoints recursively builds navigation points
func (b *OEBBook) buildNCXNavPoints(toc *TOCEntry, playOrder *int, flatEntries []*TOCEntry) []NCXNavPoint {
	points := []NCXNavPoint{}

	// Only add non-root entries or root if it has a real label
	if toc.ID != "root" && toc.ID != "merged_root" {
		point := b.buildNCXNavPoint(toc, playOrder)
		points = append(points, point)
		(*playOrder)++
	}

	// Process children
	for _, child := range toc.Children {
		childPoint := b.buildNCXNavPoint(child, playOrder)
		points = append(points, childPoint)
		(*playOrder)++
	}

	return points
}

// buildNCXNavPoint builds a single navigation point
func (b *OEBBook) buildNCXNavPoint(toc *TOCEntry, playOrder *int) NCXNavPoint {
	// Generate ID
	id := toc.ID
	if id == "" {
		id = fmt.Sprintf("navpoint_%d", *playOrder)
	}

	// Build href
	href := toc.Href
	if href == "" {
		href = "#"
	}

	point := NCXNavPoint{
		ID:        id,
		PlayOrder: *playOrder,
		NavLabel:  NCXText{Text: toc.Label},
		Content:   NCXContent{Src: href},
	}

	// Add children recursively
	for _, child := range toc.Children {
		(*playOrder)++
		childPoint := b.buildNCXNavPoint(child, playOrder)
		point.Children = append(point.Children, childPoint)
	}

	return point
}

// BuildTOCFromSections builds a TOC from section data
func BuildTOCFromSections(sections []SectionData) TOCEntry {
	root := &TOCEntry{
		ID:    "root",
		Label: "Root",
		Level: 0,
	}

	for _, section := range sections {
		addSectionToTOC(root, section)
	}

	return *root
}

// SectionData represents section information for TOC building
type SectionData struct {
	ID       string
	Title    string
	Href     string
	Level    int
	Children []SectionData
}

// addSectionToTOC adds a section to the TOC tree
func addSectionToTOC(root *TOCEntry, section SectionData) {
	entry := &TOCEntry{
		ID:    section.ID,
		Label: section.Title,
		Href:  section.Href,
		Level: section.Level,
	}

	// Find the right parent based on level
	parent := findParentForLevel(root, section.Level)
	parent.Children = append(parent.Children, entry)

	// Recursively add children
	for _, child := range section.Children {
		addSectionToTOC(entry, child)
	}
}

// findParentForLevel finds the appropriate parent for a given level
func findParentForLevel(root *TOCEntry, level int) *TOCEntry {
	if level <= root.Level+1 {
		return root
	}

	// Search in children
	for i := len(root.Children) - 1; i >= 0; i-- {
		if root.Children[i].Level == level-1 {
			return root.Children[i]
		}
	}

	// If no exact match, use the last child at highest level
	if len(root.Children) > 0 {
		return root.Children[len(root.Children)-1]
	}

	return root
}

// GenerateHTMLTOC generates an inline HTML table of contents
func (b *OEBBook) GenerateHTMLTOC() string {
	var buf bytes.Buffer

	buf.WriteString(`<div class="toc">
<h2>Table of Contents</h2>
<ul>
`)

	b.writeHTMLTOC(&buf, &b.TOC)

	buf.WriteString(`</ul>
</div>
`)

	return buf.String()
}

// writeHTMLTOC writes TOC entries as HTML list
func (b *OEBBook) writeHTMLTOC(buf *bytes.Buffer, toc *TOCEntry) {
	if toc.Label != "" || toc.ID != "root" {
		buf.WriteString(fmt.Sprintf(`<li><a href="%s">%s</a>`,
			htmlEscape(toc.Href), htmlEscape(toc.Label)))

		if len(toc.Children) > 0 {
			buf.WriteString("\n<ul>\n")
			for _, child := range toc.Children {
				b.writeHTMLTOC(buf, child)
			}
			buf.WriteString("</ul>\n")
		}

		buf.WriteString("</li>\n")
	} else {
		// Root node - just write children
		for _, child := range toc.Children {
			b.writeHTMLTOC(buf, child)
		}
	}
}

// SortTOC sorts TOC entries by label
func (b *OEBBook) SortTOC() {
	sortTOCRecursive(&b.TOC)
}

// sortTOCRecursive recursively sorts TOC entries
func sortTOCRecursive(toc *TOCEntry) {
	if len(toc.Children) <= 1 {
		return
	}

	// Sort children by label
	sort.Slice(toc.Children, func(i, j int) bool {
		// Compare labels naturally (case-insensitive)
		return strings.ToLower(toc.Children[i].Label) < strings.ToLower(toc.Children[j].Label)
	})

	// Recursively sort each child
	for _, child := range toc.Children {
		sortTOCRecursive(child)
	}
}

// MergeTOCs merges multiple TOCs into one
func MergeTOCs(tocs ...TOCEntry) TOCEntry {
	if len(tocs) == 0 {
		return TOCEntry{}
	}
	if len(tocs) == 1 {
		return tocs[0]
	}

	root := TOCEntry{
		ID:    "merged_root",
		Label: "Contents",
		Level: 0,
		Children: make([]*TOCEntry, 0),
	}

	for _, toc := range tocs {
		if toc.ID != "root" && toc.ID != "merged_root" {
			root.Children = append(root.Children, &toc)
		} else {
			// Merge children
			root.Children = append(root.Children, toc.Children...)
		}
	}

	return root
}
