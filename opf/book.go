// Package opf provides Open eBook (OPF) representation for conversion.
package opf

import (
	"sort"
	"time"
)

// OEBBook represents an Open eBook publication
type OEBBook struct {
	// Metadata
	Metadata Metadata

	// Manifest contains all resources (HTML, images, fonts, styles)
	Manifest map[string]*Resource

	// Spine defines the reading order (IDs of resources in manifest)
	Spine []string

	// TOC is the table of contents
	TOC TOCEntry

	// The primary content HTML
	Content string
}

// Resource represents a file in the publication
type Resource struct {
	ID       string
	Href     string
	MediaType string
	Data     []byte
}

// NewOEBBook creates a new OEBBook
func NewOEBBook() *OEBBook {
	return &OEBBook{
		Manifest: make(map[string]*Resource),
		Spine:    []string{},
	}
}

// AddResource adds a resource to the manifest
func (b *OEBBook) AddResource(id, href, mediaType string, data []byte) *Resource {
	res := &Resource{
		ID:        id,
		Href:      href,
		MediaType: mediaType,
		Data:      data,
	}
	b.Manifest[id] = res
	return res
}

// GetResource retrieves a resource by ID
func (b *OEBBook) GetResource(id string) (*Resource, bool) {
	res, ok := b.Manifest[id]
	return res, ok
}

// AddToSpine adds a resource ID to the spine (reading order)
func (b *OEBBook) AddToSpine(id string) {
	b.Spine = append(b.Spine, id)
}

// GetManifestIDs returns sorted manifest IDs
func (b *OEBBook) GetManifestIDs() []string {
	ids := make([]string, 0, len(b.Manifest))
	for id := range b.Manifest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// HasImages returns true if the book has any image resources
func (b *OEBBook) HasImages() bool {
	for _, res := range b.Manifest {
		if len(res.MediaType) >= 6 && res.MediaType[0:5] == "image" {
			return true
		}
	}
	return false
}

// Metadata represents OPF metadata
type Metadata struct {
	Title       string
	Authors     []Author
	Translator  []Author // For translated works
	Contributors []string
	Publisher   string
	ISBN        string
	ASIN        string // Amazon ASIN
	DOI         string
	Year        string
	PubDate     time.Time
	Language    string
	Languages   []string
	Series      string
	SeriesIndex int
	Genres      []string
	Keywords    []string
	Annotation  string
	Comments    string

	// Cover image
	Cover     []byte
	CoverID   string // Resource ID in manifest
	CoverExt  string // jpg, png, etc.

	// Additional metadata
	Source      string // Original file path
	Rights      string // Copyright info
	Subject     string // DC:subject
	Description string // DC:description
}

// Author represents an author with structured name parts
type Author struct {
	FirstName  string
	MiddleName string
	LastName   string
	Nickname   string
	Role       string // e.g., "aut", "trl"

	// Formatted names
	FullName    string // "First Middle Last"
	SortName    string // "Last, First Middle"
}

// NewAuthor creates an author from name parts
func NewAuthor(first, middle, last, nickname string) Author {
	a := Author{
		FirstName:  first,
		MiddleName: middle,
		LastName:   last,
		Nickname:   nickname,
	}

	// Build full name
	parts := []string{}
	if first != "" {
		parts = append(parts, first)
	}
	if middle != "" {
		parts = append(parts, middle)
	}
	if last != "" {
		parts = append(parts, last)
	}
	fullName := joinNonEmpty(" ", parts...)

	// If no name parts, use nickname
	if fullName == "" {
		a.FullName = nickname
	} else {
		a.FullName = fullName
	}

	// Build sort name
	if last != "" {
		a.SortName = last
		if first != "" {
			a.SortName += ", " + first
			if middle != "" {
				a.SortName += " " + middle
			}
		}
	} else {
		a.SortName = a.FullName
	}

	return a
}

// TOCEntry represents a table of contents entry
type TOCEntry struct {
	ID       string
	Label    string
	Href     string
	Level    int
	Children []*TOCEntry
}

// AddChild adds a child TOC entry
func (e *TOCEntry) AddChild(id, label, href string) *TOCEntry {
	child := &TOCEntry{
		ID:    id,
		Label: label,
		Href:  href,
		Level: e.Level + 1,
	}
	e.Children = append(e.Children, child)
	return child
}

// Flatten returns a flattened list of all TOC entries in depth-first order
func (e *TOCEntry) Flatten() []*TOCEntry {
	entries := []*TOCEntry{e}
	for _, child := range e.Children {
		entries = append(entries, child.Flatten()...)
	}
	return entries
}

// MaxDepth returns the maximum depth of the TOC tree
func (e *TOCEntry) MaxDepth() int {
	if len(e.Children) == 0 {
		return e.Level
	}
	max := e.Level
	for _, child := range e.Children {
		d := child.MaxDepth()
		if d > max {
			max = d
		}
	}
	return max
}

// joinNonEmpty joins non-empty strings with a separator
func joinNonEmpty(sep string, parts ...string) string {
	result := []string{}
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return ""
	}
	// Simple join - in real code would use strings.Join
	s := result[0]
	for i := 1; i < len(result); i++ {
		s += sep + result[i]
	}
	return s
}
