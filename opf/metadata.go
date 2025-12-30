// Package opf provides OPF metadata generation.
package opf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
)

// OPFPackage represents an OPF package document
type OPFPackage struct {
	XMLName   xml.Name `xml:"package"`
	Version   string   `xml:"version,attr"`
	XMLNS     string   `xml:"xmlns,attr"`
	UniqueID  string   `xml:"unique-identifier,attr"`
	Metadata  OPFMetadata `xml:"metadata"`
	Manifest  OPFManifest `xml:"manifest"`
	Spine     OPFSpine `xml:"spine"`
	Guide     *OPFGuide `xml:"guide,omitempty"`
}

// OPFMetadata contains Dublin Core metadata
type OPFMetadata struct {
	XMLName      xml.Name `xml:"metadata"`
	XMLNSDC      string   `xml:"xmlns:dc,attr"`
	XMLNSOPF     string   `xml:"xmlns:opf,attr"`
	DCTitle      string   `xml:"dc:title"`
	DCCreators   []OPFDCreator `xml:"dc:creator"`
	DCContributors []string `xml:"dc:contributor"`
	DCPublisher  string   `xml:"dc:publisher,omitempty"`
	DCIdentifier OPFIdentifier `xml:"dc:identifier"`
	DCDate       OPFDate `xml:"dc:date"`
	DCLanguage   string   `xml:"dc:language"`
	DCSubject    []string `xml:"dc:subject"`
	DCDescription string  `xml:"dc:description,omitempty"`
	DCRights     string   `xml:"dc:rights,omitempty"`
	Meta         []OPFMeta `xml:"meta"`
}

// OPFDCreator represents a creator (author, translator, etc.)
type OPFDCreator struct {
	XMLName xml.Name `xml:"dc:creator"`
	Role    string   `xml:"opf:role,attr,omitempty"`
	FileAs  string   `xml:"opf:file-as,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// OPFIdentifier represents a unique identifier
type OPFIdentifier struct {
	XMLName xml.Name `xml:"dc:identifier"`
	ID      string   `xml:"id,attr"`
	Scheme  string   `xml:"opf:scheme,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// OPFDate represents a date
type OPFDate struct {
	XMLName xml.Name `xml:"dc:date"`
	Event   string   `xml:"opf:event,attr,omitempty"`
	Text    string   `xml:",chardata"`
}

// OPFMeta represents a meta element
type OPFMeta struct {
	XMLName xml.Name `xml:"meta"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:"content,attr"`
}

// OPFManifest contains all resources
type OPFManifest struct {
	XMLName  xml.Name `xml:"manifest"`
	Items    []OPFItem `xml:"item"`
}

// OPFItem represents a resource in the manifest
type OPFItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// OPFSpine defines the reading order
type OPFSpine struct {
	XMLName xml.Name `xml:"spine"`
	TOC     string   `xml:"toc,attr"`
	ItemRefs []OPFItemRef `xml:"itemref"`
}

// OPFItemRef references a manifest item in the spine
type OPFItemRef struct {
	IDREF      string `xml:"idref,attr"`
	Linear     string `xml:"linear,attr,omitempty"`
}

// OPFGuide contains special locations (cover, TOC, etc.)
type OPFGuide struct {
	XMLName xml.Name `xml:"guide"`
	Refs    []OPFGuideRef `xml:"reference"`
}

// OPFGuideRef represents a guide reference
type OPFGuideRef struct {
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

	pkg := OPFPackage{
		Version:  "2.0",
		XMLNS:    "http://www.idpf.org/2007/opf",
		UniqueID: uniqueID,
		Metadata: b.buildOPFMetadata(uniqueID),
		Manifest: b.buildOPFManifest(),
		Spine:    b.buildOPFSpine(),
		Guide:    b.buildOPFGuide(),
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

// buildOPFMetadata builds OPF metadata from book metadata
func (b *OEBBook) buildOPFMetadata(uniqueID string) OPFMetadata {
	m := OPFMetadata{
		XMLNSDC:      "http://purl.org/dc/elements/1.1/",
		XMLNSOPF:     "http://www.idpf.org/2007/opf",
		DCTitle:      b.Metadata.Title,
		DCLanguage:   b.Metadata.Language,
		DCPublisher:  b.Metadata.Publisher,
		DCDescription: b.Metadata.Annotation,
		DCRights:     b.Metadata.Rights,
		DCSubject:    b.Metadata.Genres,
	}

	// Creators (authors, translators, etc.)
	for _, author := range b.Metadata.Authors {
		creator := OPFDCreator{
			Role:   author.Role,
			FileAs: author.SortName,
			Text:   author.FullName,
		}
		if creator.Role == "" {
			creator.Role = "aut" // Default to author
		}
		m.DCCreators = append(m.DCCreators, creator)
	}

	// Contributors
	for _, contributor := range b.Metadata.Contributors {
		m.DCContributors = append(m.DCContributors, contributor)
	}

	// Identifier (ISBN or UUID)
	identifier := OPFIdentifier{
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
		m.DCDate = OPFDate{
			Event: "publication",
			Text:  b.Metadata.PubDate.Format("2006-01-02"),
		}
	} else if b.Metadata.Year != "" {
		m.DCDate = OPFDate{
			Event: "publication",
			Text:  b.Metadata.Year + "-01-01",
		}
	}

	// Meta elements
	if b.Metadata.Series != "" {
		m.Meta = append(m.Meta, OPFMeta{
			Name:    "calibre:series",
			Content: b.Metadata.Series,
		})
		if b.Metadata.SeriesIndex > 0 {
			m.Meta = append(m.Meta, OPFMeta{
				Name:    "calibre:series_index",
				Content: fmt.Sprintf("%d", b.Metadata.SeriesIndex),
			})
		}
	}

	// Cover meta
	if b.Metadata.CoverID != "" {
		m.Meta = append(m.Meta, OPFMeta{
			Name:    "cover",
			Content: b.Metadata.CoverID,
		})
	}

	// Author sort
	if len(b.Metadata.Authors) > 0 && b.Metadata.Authors[0].SortName != "" {
		m.Meta = append(m.Meta, OPFMeta{
			Name:    "author_sort",
			Content: b.Metadata.Authors[0].SortName,
		})
	}

	// Title sort (could be customized)
	m.Meta = append(m.Meta, OPFMeta{
		Name:    "title_sort",
		Content: b.Metadata.Title,
	})

	return m
}

// buildOPFManifest builds the manifest from book resources
func (b *OEBBook) buildOPFManifest() OPFManifest {
	manifest := OPFManifest{}

	// Get sorted IDs for consistent output
	ids := b.GetManifestIDs()

	for _, id := range ids {
		res := b.Manifest[id]
		item := OPFItem{
			ID:        res.ID,
			Href:      res.Href,
			MediaType: res.MediaType,
		}
		manifest.Items = append(manifest.Items, item)
	}

	return manifest
}

// buildOPFSpine builds the spine (reading order)
func (b *OEBBook) buildOPFSpine() OPFSpine {
	spine := OPFSpine{
		TOC: "ncx",
	}

	for _, id := range b.Spine {
		spine.ItemRefs = append(spine.ItemRefs, OPFItemRef{
			IDREF: id,
		})
	}

	return spine
}

// buildOPFGuide builds the guide (special locations)
func (b *OEBBook) buildOPFGuide() *OPFGuide {
	guide := &OPFGuide{}

	// Cover reference
	if b.Metadata.CoverID != "" {
		if res, ok := b.Manifest[b.Metadata.CoverID]; ok {
			guide.Refs = append(guide.Refs, OPFGuideRef{
				Type:  "cover",
				Title: "Cover",
				Href:  res.Href,
			})
		}
	}

	// TOC reference (if we have a TOC resource)
	if ncx, ok := b.Manifest["ncx"]; ok {
		guide.Refs = append(guide.Refs, OPFGuideRef{
			Type:  "toc",
			Title: "Table of Contents",
			Href:  ncx.Href,
		})
	}

	// Title page (if exists)
	if title, ok := b.Manifest["titlepage"]; ok {
		guide.Refs = append(guide.Refs, OPFGuideRef{
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
