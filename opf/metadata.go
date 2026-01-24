// Package opf provides OPF metadata generation.
package opf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
)

// Package represents an OPF package document
type Package struct {
	XMLName  xml.Name    `xml:"package"`
	Version  string      `xml:"version,attr"`
	XMLNS    string      `xml:"xmlns,attr"`
	UniqueID string      `xml:"unique-identifier,attr"`
	Metadata XMLMetadata `xml:"metadata"`
	Manifest Manifest    `xml:"manifest"`
	Spine    Spine       `xml:"spine"`
	Guide    *Guide      `xml:"guide,omitempty"`
}

// Metadata contains Dublin Core metadata
type XMLMetadata struct {
	XMLName        xml.Name   `xml:"metadata"`
	XMLNSDC        string     `xml:"xmlns:dc,attr"`
	XMLNSOPF       string     `xml:"xmlns:opf,attr"`
	DCTitle        string     `xml:"dc:title"`
	DCCreators     []DCreator `xml:"dc:creator"`
	DCContributors []string   `xml:"dc:contributor"`
	DCPublisher    string     `xml:"dc:publisher,omitempty"`
	DCIdentifier   Identifier `xml:"dc:identifier"`
	DCDate         Date       `xml:"dc:date"`
	DCLanguage     string     `xml:"dc:language"`
	DCSubject      []string   `xml:"dc:subject"`
	DCDescription  string     `xml:"dc:description,omitempty"`
	DCRights       string     `xml:"dc:rights,omitempty"`
	Meta           []Meta     `xml:"meta"`
}

// DCreator represents a creator (author, translator, etc.)
type DCreator struct {
	XMLName xml.Name `xml:"dc:creator"`
	Role    string   `xml:"opf:role,attr,omitempty"`
	FileAs  string   `xml:"opf:file-as,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// Identifier represents a unique identifier
type Identifier struct {
	XMLName xml.Name `xml:"dc:identifier"`
	ID      string   `xml:"id,attr"`
	Scheme  string   `xml:"opf:scheme,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// Date represents a date
type Date struct {
	XMLName xml.Name `xml:"dc:date"`
	Event   string   `xml:"opf:event,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// Meta represents a meta element
type Meta struct {
	XMLName xml.Name `xml:"meta"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:"content,attr"`
}

// Manifest contains all resources
type Manifest struct {
	XMLName xml.Name `xml:"manifest"`
	Items   []Item   `xml:"item"`
}

// Item represents a resource in the manifest
type Item struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// Spine defines the reading order
type Spine struct {
	XMLName  xml.Name  `xml:"spine"`
	TOC      string    `xml:"toc,attr"`
	ItemRefs []ItemRef `xml:"itemref"`
}

// ItemRef references a manifest item in the spine
type ItemRef struct {
	IDREF  string `xml:"idref,attr"`
	Linear string `xml:"linear,attr,omitempty"`
}

// Guide contains special locations (cover, TOC, etc.)
type Guide struct {
	XMLName xml.Name   `xml:"guide"`
	Refs    []GuideRef `xml:"reference"`
}

// GuideRef represents a guide reference
type GuideRef struct {
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
	Href  string `xml:"href,attr"`
}

// GenerateOPF creates an OPF XML document from OEBBook
func (b *OEBBook) GenerateOPF() ([]byte, error) {
	// Create unique ID for the book
	uniqueID := "book_id"
	if b.Metadata.ISBN != "" {
		uniqueID = "isbn_id"
	}

	pkg := Package{
		Version:  "2.0",
		XMLNS:    "http://www.idpf.org/2007/opf",
		UniqueID: uniqueID,
		Metadata: b.buildXMLMetadata(uniqueID),
		Manifest: b.buildManifest(),
		Spine:    b.buildSpine(),
		Guide:    b.buildGuide(),
	}

	// Marshal to XML with indentation
	data, err := xml.MarshalIndent(pkg, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OPF: %w", err)
	}

	// Add XML declaration
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.Write(data)

	return buf.Bytes(), nil
}

// buildXMLMetadata builds OPF metadata from book metadata
func (b *OEBBook) buildXMLMetadata(uniqueID string) XMLMetadata {
	m := XMLMetadata{
		XMLNSDC:       "http://purl.org/dc/elements/1.1/",
		XMLNSOPF:      "http://www.idpf.org/2007/opf",
		DCTitle:       b.Metadata.Title,
		DCLanguage:    b.Metadata.Language,
		DCPublisher:   b.Metadata.Publisher,
		DCDescription: b.Metadata.Annotation,
		DCRights:      b.Metadata.Rights,
		DCSubject:     b.Metadata.Genres,
	}

	// Creators (authors, translators, etc.)
	for _, author := range b.Metadata.Authors {
		creator := DCreator{
			Role:   author.Role,
			FileAs: author.SortName,
			Text:   author.FullName,
		}
		if creator.Role == "" {
			creator.Role = "aut" // Default to author
		}
		m.DCCreators = append(m.DCCreators, creator)
	}

	m.DCContributors = append(m.DCContributors, b.Metadata.Contributors...)

	// Identifier (ISBN or UUID)
	identifier := Identifier{
		ID:     uniqueID,
		Scheme: "ISBN",
		Text:   b.Metadata.ISBN,
	}
	if b.Metadata.ISBN == "" {
		identifier.Scheme = "UUID"
		identifier.ID = "uuid_id"
		// Would generate UUID here in production
		identifier.Text = "urn:uuid:generated-uuid"
	}
	m.DCIdentifier = identifier

	// Publication date
	if !b.Metadata.PubDate.IsZero() {
		m.DCDate = Date{
			Event: "publication",
			Text:  b.Metadata.PubDate.Format("2006-01-02"),
		}
	} else if b.Metadata.Year != "" {
		m.DCDate = Date{
			Event: "publication",
			Text:  b.Metadata.Year + "-01-01",
		}
	}

	// Meta elements
	if b.Metadata.Series != "" {
		m.Meta = append(m.Meta, Meta{
			Name:    "calibre:series",
			Content: b.Metadata.Series,
		})
		if b.Metadata.SeriesIndex > 0 {
			m.Meta = append(m.Meta, Meta{
				Name:    "calibre:series_index",
				Content: fmt.Sprintf("%d", b.Metadata.SeriesIndex),
			})
		}
	}

	// Cover meta
	if b.Metadata.CoverID != "" {
		m.Meta = append(m.Meta, Meta{
			Name:    "cover",
			Content: b.Metadata.CoverID,
		})
	}

	// Author sort
	if len(b.Metadata.Authors) > 0 && b.Metadata.Authors[0].SortName != "" {
		m.Meta = append(m.Meta, Meta{
			Name:    "author_sort",
			Content: b.Metadata.Authors[0].SortName,
		})
	}

	// Title sort (could be customized)
	m.Meta = append(m.Meta, Meta{
		Name:    "title_sort",
		Content: b.Metadata.Title,
	})

	return m
}

// buildManifest builds the manifest from book resources
func (b *OEBBook) buildManifest() Manifest {
	manifest := Manifest{}

	// Get sorted IDs for consistent output
	ids := b.GetManifestIDs()

	for _, id := range ids {
		res := b.Manifest[id]
		item := Item{
			ID:        res.ID,
			Href:      res.Href,
			MediaType: res.MediaType,
		}
		manifest.Items = append(manifest.Items, item)
	}

	return manifest
}

// buildSpine builds the spine (reading order)
func (b *OEBBook) buildSpine() Spine {
	spine := Spine{
		TOC: "ncx",
	}

	for _, id := range b.Spine {
		spine.ItemRefs = append(spine.ItemRefs, ItemRef{
			IDREF: id,
		})
	}

	return spine
}

// buildGuide builds the guide (special locations)
func (b *OEBBook) buildGuide() *Guide {
	guide := &Guide{}

	// Cover reference
	if b.Metadata.CoverID != "" {
		if res, ok := b.Manifest[b.Metadata.CoverID]; ok {
			guide.Refs = append(guide.Refs, GuideRef{
				Type:  "cover",
				Title: "Cover",
				Href:  res.Href,
			})
		}
	}

	// TOC reference (if we have a TOC resource)
	if ncx, ok := b.Manifest["ncx"]; ok {
		guide.Refs = append(guide.Refs, GuideRef{
			Type:  "toc",
			Title: "Table of Contents",
			Href:  ncx.Href,
		})
	}

	// Title page (if exists)
	if title, ok := b.Manifest["titlepage"]; ok {
		guide.Refs = append(guide.Refs, GuideRef{
			Type:  "title-page",
			Title: "Title Page",
			Href:  title.Href,
		})
	}

	// If no guide entries, return nil
	if len(guide.Refs) == 0 {
		return nil
	}

	return guide
}

// ConvertMetadataFromFB2 converts FB2 metadata to OEB metadata
func ConvertMetadataFromFB2(
	title string,
	authors []string,
	authorSort string,
	publisher, isbn, year, language string,
	pubDate time.Time,
	series string,
	seriesIndex int,
	genres, keywords []string,
	annotation string,
	cover []byte,
	coverID string,
	coverExt string,
) Metadata {
	m := Metadata{
		Title:       title,
		Publisher:   publisher,
		ISBN:        isbn,
		Year:        year,
		PubDate:     pubDate,
		Language:    language,
		Languages:   []string{language},
		Series:      series,
		SeriesIndex: seriesIndex,
		Genres:      genres,
		Keywords:    keywords,
		Annotation:  annotation,
		Comments:    annotation,
		Cover:       cover,
		CoverID:     coverID,
		CoverExt:    coverExt,
	}

	// Parse authors
	for _, authorStr := range authors {
		// Try to parse "First Middle Last" format
		// For now, use full name
		a := Author{
			FullName: authorStr,
			SortName: authorSort,
			Role:     "aut",
		}
		m.Authors = append(m.Authors, a)
	}

	return m
}
