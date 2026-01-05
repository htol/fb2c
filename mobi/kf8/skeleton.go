// Package kf8 provides KF8 (MOBI 8) specific functionality.
package kf8

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// KF8 constants
	KF8Version        = 8
	TargetChunkSize   = 8192  // Target size for HTML chunks (~8KB)
	MaxChunkSize      = 10240 // Maximum chunk size (10KB)
	MinChunkSize      = 6144  // Minimum chunk size (6KB)

	// AID (Anchor ID) format
	AIDBase32 = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// Chunk represents a chunk of HTML content
type Chunk struct {
	ID        int    // Sequential chunk number
	AID       string // Anchor ID (base32 encoded)
	Offset    int    // Byte offset in original HTML
	Length    int    // Length of this chunk
	Content   string // HTML content
	Parent    *Chunk // Parent chunk (if nested)
	Children  []*Chunk // Child chunks
	OpenTags  []TagPosition // Tags opened in this chunk
	CloseTags []TagPosition // Tags closed in this chunk
}

// TagPosition represents a tag position within a chunk
type TagPosition struct {
	Tag      string // Tag name (e.g., "div", "p")
	Position int    // Position within chunk
	IsOpen   bool   // True for opening, false for closing
	SelfClose bool // True for self-closing tags
}

// Skeleton represents the chunked HTML structure
type Skeleton struct {
	Chunks       []*Chunk
	TotalLength  int
	MaxDepth     int
	AIDCounter   int
}

// NewSkeleton creates a new skeleton
func NewSkeleton() *Skeleton {
	return &Skeleton{
		Chunks:     make([]*Chunk, 0),
		AIDCounter: 0,
	}
}

// ChunkHTML chunks HTML content into manageable pieces
func (s *Skeleton) ChunkHTML(html string) error {
	// Parse HTML to identify tag positions
	tagPositions, err := parseHTMLTags(html)
	if err != nil {
		return fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Create chunks based on target size
	currentOffset := 0
	depth := 0
	openTags := make(map[string]int) // tag -> count

	for currentOffset < len(html) {
		// Determine end of this chunk
		endOffset := currentOffset + TargetChunkSize
		if endOffset > len(html) {
			endOffset = len(html)
		}

		// Try to find a good break point
		breakPoint := findBreakPoint(html, currentOffset, endOffset, tagPositions, openTags)

		// Create chunk
		chunk := &Chunk{
			ID:       len(s.Chunks),
			AID:      s.generateAID(),
			Offset:   currentOffset,
			Length:   breakPoint - currentOffset,
			Content:  html[currentOffset:breakPoint],
			OpenTags: make([]TagPosition, 0),
			CloseTags: make([]TagPosition, 0),
		}

		// Track tags opened/closed in this chunk
		for _, pos := range tagPositions {
			if pos.Position < currentOffset {
				continue
			}
			if pos.Position >= breakPoint {
				break
			}

			if pos.IsOpen {
				chunk.OpenTags = append(chunk.OpenTags, pos)
				depth++
				openTags[pos.Tag]++
			} else {
				chunk.CloseTags = append(chunk.CloseTags, pos)
				if openTags[pos.Tag] > 0 {
					openTags[pos.Tag]--
				}
				depth--
			}
		}

		s.Chunks = append(s.Chunks, chunk)

		// Update depth tracking
		if depth > s.MaxDepth {
			s.MaxDepth = depth
		}

		currentOffset = breakPoint
	}

	s.TotalLength = len(html)

	return nil
}

// parseHTMLTags parses HTML and returns tag positions
func parseHTMLTags(html string) ([]TagPosition, error) {
	positions := make([]TagPosition, 0)

	// Regex to find HTML tags
	// Matches: <tag>, </tag>, <tag/>, <tag attr="value">
	re := regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)(?:\s[^>]*)?(/?)>`)

	matches := re.FindAllStringSubmatchIndex(html, -1)
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		tagStart := match[0]
		_ = match[1] // tagEnd

			// Extract tag name
		tagNameStart := match[2]
		tagNameEnd := match[3]
		tagName := html[tagNameStart:tagNameEnd]

		// Check if closing tag
		isClose := match[4] != match[5] && (match[4] >= 0 && html[match[4]] == '/')

		// Check if self-closing
		selfClose := match[4] != match[5] && (match[4] >= 0 && html[match[4]] == '/')

		positions = append(positions, TagPosition{
			Tag:       strings.ToLower(tagName),
			Position:  tagStart,
			IsOpen:    !isClose && !selfClose,
			SelfClose: selfClose,
		})
	}

	return positions, nil
}

// findBreakPoint finds a good place to break the HTML
func findBreakPoint(html string, start, end int, positions []TagPosition, openTags map[string]int) int {
	// If we're at the end, return length
	if end >= len(html) {
		return len(html)
	}

	// Look for tag boundaries near the target size
	bestBreak := end
	minDistance := 0

	for _, pos := range positions {
		if pos.Position < start {
			continue
		}
		if pos.Position > end+MinChunkSize {
			break
		}

		// Prefer closing tags (ends a block)
		if !pos.IsOpen && !pos.SelfClose {
			distance := abs(pos.Position - end)
			if bestBreak == end || distance < minDistance {
				bestBreak = pos.Position + len(html[pos.Position:]) // Actually we want position after tag
				// Find the end of the tag
				tagEnd := strings.Index(html[pos.Position:], ">")
				if tagEnd >= 0 {
					bestBreak = pos.Position + tagEnd + 1
				}
				minDistance = distance
			}
		}
	}

	// Constrain to min/max sizes
	if bestBreak < start+MinChunkSize && bestBreak < len(html) {
		return start + MinChunkSize
	}
	if bestBreak > start+MaxChunkSize {
		return start + MaxChunkSize
	}

	return bestBreak
}

// abs returns absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// generateAID generates a unique anchor ID (base32 encoded)
func (s *Skeleton) generateAID() string {
	aid := encodeBase32(s.AIDCounter)
	s.AIDCounter++
	return aid
}

// encodeBase32 encodes an integer to base32
func encodeBase32(n int) string {
	if n == 0 {
		return "0"
	}

	chars := []byte{}
	base := len(AIDBase32)

	for n > 0 {
		chars = append(chars, AIDBase32[n%base])
		n /= base
	}

	// Reverse
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}

	return string(chars)
}

// GetChunkByAID retrieves a chunk by its AID
func (s *Skeleton) GetChunkByAID(aid string) (*Chunk, bool) {
	for _, chunk := range s.Chunks {
		if chunk.AID == aid {
			return chunk, true
		}
	}
	return nil, false
}

// GetChunkByOffset retrieves the chunk containing a given offset
func (s *Skeleton) GetChunkByOffset(offset int) (*Chunk, bool) {
	for _, chunk := range s.Chunks {
		if offset >= chunk.Offset && offset < chunk.Offset+chunk.Length {
			return chunk, true
		}
	}
	return nil, false
}

// BuildHierarchy builds parent-child relationships between chunks
func (s *Skeleton) BuildHierarchy() {
	for i := 0; i < len(s.Chunks); i++ {
		chunk := s.Chunks[i]

		// Find parent by looking at depth
		// A chunk's parent is the previous chunk that has fewer open tags
		depth := len(chunk.OpenTags)
		for j := i - 1; j >= 0; j-- {
			parent := s.Chunks[j]
			parentDepth := len(parent.OpenTags) - len(parent.CloseTags)
			if parentDepth < depth {
				chunk.Parent = parent
				parent.Children = append(parent.Children, chunk)
				break
			}
		}
	}
}

// GetChunkDepth calculates the nesting depth of a chunk
func (s *Skeleton) GetChunkDepth(chunk *Chunk) int {
	depth := 0
	current := chunk.Parent
	for current != nil {
		depth++
		current = current.Parent
	}
	return depth
}

// AssignAIDAttributes adds aid attributes to HTML elements
func (s *Skeleton) AssignAIDAttributes() string {
	var result strings.Builder

	for _, chunk := range s.Chunks {
		content := chunk.Content

		// Add aid attribute to the first element in the chunk
		// Skip DOCTYPE and other special tags
		tagStart := strings.Index(content, "<")
		if tagStart >= 0 && !strings.HasPrefix(content[tagStart:], "</") {
			// Skip DOCTYPE, comments, etc.
			if strings.HasPrefix(content[tagStart:], "<!DOCTYPE") ||
				strings.HasPrefix(content[tagStart:], "<!--") ||
				strings.HasPrefix(content[tagStart:], "<?xml") {
				// Don't add aid to these tags
			} else {
				// Find the tag name end
				tagEnd := strings.Index(content[tagStart:], " ")
				if tagEnd < 0 {
					tagEnd = strings.Index(content[tagStart:], ">")
				}
				if tagEnd > 0 {
					// Insert aid attribute
					insertPos := tagStart + tagEnd
					aidAttr := fmt.Sprintf(` aid="%s"`, chunk.AID)
					content = content[:insertPos] + aidAttr + content[insertPos:]
				}
			}
		}

		result.WriteString(content)
	}

	return result.String()
}
