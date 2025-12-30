// Package kf8 provides flow management for KF8.
package kf8

import (
	"fmt"
	"strings"
)

// FlowType represents the type of content flow
type FlowType int

const (
	FlowTypeHTML FlowType = iota
	FlowTypeCSS
	FlowTypeSVG
	FlowTypeFont
	FlowTypeImage
)

// Flow represents a content flow in KF8
type Flow struct {
	ID          string
	Type        FlowType
	Content     string
	Resources   []*ResourceRef
	Index       int // Flow index number
}

// ResourceRef represents a reference to a resource in a flow
type ResourceRef struct {
	ID       string // Resource ID
	Href     string // Original href
	KindleRef string // Converted reference (kindle:embed: or kindle:flow:)
}

// FlowManager manages multiple flows in a KF8 document
type FlowManager struct {
	flows     []*Flow
	flowMap   map[string]*Flow // ID -> Flow
	resourceIndex int
}

// NewFlowManager creates a new flow manager
func NewFlowManager() *FlowManager {
	return &FlowManager{
		flows:   make([]*Flow, 0),
		flowMap: make(map[string]*Flow),
		resourceIndex: 1,
	}
}

// CreateFlow creates a new content flow
func (fm *FlowManager) CreateFlow(id string, flowType FlowType, content string) *Flow {
	flow := &Flow{
		ID:        id,
		Type:      flowType,
		Content:   content,
		Index:     len(fm.flows),
		Resources: make([]*ResourceRef, 0),
	}

	fm.flows = append(fm.flows, flow)
	fm.flowMap[id] = flow

	return flow
}

// GetFlow retrieves a flow by ID
func (fm *FlowManager) GetFlow(id string) (*Flow, bool) {
	flow, ok := fm.flowMap[id]
	return flow, ok
}

// GetFlows returns all flows
func (fm *FlowManager) GetFlows() []*Flow {
	return fm.flows
}

// GetPrimaryFlow returns the primary HTML flow (usually first flow)
func (fm *FlowManager) GetPrimaryFlow() (*Flow, bool) {
	if len(fm.flows) == 0 {
		return nil, false
	}
	return fm.flows[0], true
}

// AddResourceToFlow adds a resource reference to a flow
func (fm *FlowManager) AddResourceToFlow(flowID, resourceID, href string) error {
	flow, ok := fm.flowMap[flowID]
	if !ok {
		return fmt.Errorf("flow not found: %s", flowID)
	}

	ref := &ResourceRef{
		ID:       resourceID,
		Href:     href,
		KindleRef: fm.convertResourceRef(resourceID),
	}

	flow.Resources = append(flow.Resources, ref)
	return nil
}

// convertResourceRef converts a resource reference to Kindle format
func (fm *FlowManager) convertResourceRef(resourceID string) string {
	// Check if this is an embedded resource or flow reference
	// For now, use kindle:embed: for all resources
	return fmt.Sprintf("kindle:embed:%s", resourceID)
}

// ConvertLinks converts all links in flows to Kindle format
func (fm *FlowManager) ConvertLinks() {
	for _, flow := range fm.flows {
		if flow.Type == FlowTypeHTML {
			flow.Content = fm.convertHTMLLinks(flow.Content, flow.ID)
		}
	}
}

// convertHTMLLinks converts href links to Kindle format
func (fm *FlowManager) convertHTMLLinks(html, flowID string) string {
	// Convert href links to kindle:flow: or kindle:embed:
	// This is a simplified implementation

	// Convert CSS links (<link rel="stylesheet" href="...">)
	html = convertCSSLinks(html)

	// Convert image links (<img src="...">)
	html = convertImageLinks(html)

	// Convert anchor links (<a href="...">)
	html = convertAnchorLinks(html)

	return html
}

// convertCSSLinks converts CSS stylesheet links
func convertCSSLinks(html string) string {
	// Find <link rel="stylesheet" href="...">
	// Replace href with kindle:flow: reference
	// Simplified implementation

	// For inline styles, keep them
	// For external files, convert to kindle:flow:

	return html
}

// convertImageLinks converts image src attributes
func convertImageLinks(html string) string {
	// Convert <img src="image.jpg"> to <img src="kindle:embed:image.jpg">

	// This is a simplified regex-based conversion
	// In production, would use proper HTML parser

	return html
}

// convertAnchorLinks converts anchor href attributes
func convertAnchorLinks(html string) string {
	// Convert internal links to kindle:flow:
	// Keep external links as-is

	return html
}

// FlowTable represents the flow table (resource index)
type FlowTable struct {
	Entries []*FlowTableEntry
}

// FlowTableEntry represents an entry in the flow table
type FlowTableEntry struct {
	FlowID   string
	FlowNum  int
	Offset   uint32
	Length   uint32
	Encoding uint32
}

// NewFlowTable creates a new flow table
func NewFlowTable() *FlowTable {
	return &FlowTable{
		Entries: make([]*FlowTableEntry, 0),
	}
}

// AddEntry adds an entry to the flow table
func (ft *FlowTable) AddEntry(flowID string, flowNum int, content string) {
	entry := &FlowTableEntry{
		FlowID:   flowID,
		FlowNum:  flowNum,
		Length:   uint32(len(content)),
		Encoding: 65001, // UTF-8
	}
	ft.Entries = append(ft.Entries, entry)
}

// GenerateResourceLinks generates kindle:embed: and kindle:flow: links
func GenerateResourceLinks(resourceID, resourceType string) string {
	switch resourceType {
	case "css", "stylesheet":
		return fmt.Sprintf("kindle:flow:%s", resourceID)
	case "image", "font":
		return fmt.Sprintf("kindle:embed:%s", resourceID)
	default:
		return resourceID
	}
}

// ParseResourceType determines the resource type from href
func ParseResourceType(href string) string {
	ext := strings.ToLower(href)
	if strings.HasSuffix(ext, ".css") {
		return "css"
	}
	if strings.HasSuffix(ext, ".jpg") || strings.HasSuffix(ext, ".jpeg") ||
	   strings.HasSuffix(ext, ".png") || strings.HasSuffix(ext, ".gif") {
		return "image"
	}
	if strings.HasSuffix(ext, ".ttf") || strings.HasSuffix(ext, ".otf") ||
	   strings.HasSuffix(ext, ".woff") {
		return "font"
	}
	if strings.HasSuffix(ext, ".svg") {
		return "svg"
	}
	return "unknown"
}

// IsInternalLink checks if a link is internal to the document
func IsInternalLink(href string) bool {
	// External links start with http://, https://, etc.
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return false
	}
	if strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "ftp://") {
		return false
	}
	// Anchor links (#section) are internal
	if strings.HasPrefix(href, "#") {
		return true
	}
	// Relative links are internal
	return true
}
