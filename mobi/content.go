package mobi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/htol/fb2c/opf"
)

var (
	srcAttrRe   = regexp.MustCompile(`src=["']([^"']+)["']`)
	hrefHashRe  = regexp.MustCompile(`href=["']#([^"']+)["']`)
	anchorIDRe  = regexp.MustCompile(`<a\s+id=["']([^"']+)["']`)
	fileposZeRe = regexp.MustCompile(`filepos=["']0000000000["']`)
)

// resolveImageSources replaces src="filename" with src="recindex:N"
// If baseIndex is 0, it uses relative indexing (1, 2, 3...)
// If baseIndex is > 0, it uses absolute 1-based indexing (baseIndex + 1, baseIndex + 2...)
func resolveImageSources(book *opf.OEBBook, hasCover bool, content string) string {
	imageMap := make(map[string]int)
	coverID := book.Metadata.CoverID

	currentOffset := 0

	// 2. Map cover (index 0) and thumbnail (index 1)
	// These are relative to FirstImageIndex, starting at 0
	if hasCover {
		if coverID != "" {
			imageMap[coverID] = currentOffset
		} else {
			imageMap["cover.jpg"] = currentOffset
		}
		currentOffset++ // cover
		currentOffset++ // thumbnail
	}

	// 3. Map other manifest images
	ids := book.GetManifestIDs()
	sort.Strings(ids)

	for _, id := range ids {
		if id == coverID {
			continue
		}
		res, ok := book.GetResource(id)
		if !ok || len(res.MediaType) < 6 || res.MediaType[0:5] != "image" {
			continue
		}
		imageMap[id] = currentOffset
		currentOffset++
	}

	// 4. Perform replacements
	return srcAttrRe.ReplaceAllStringFunc(content, func(match string) string {
		quote := match[4]
		url := match[5 : len(match)-1]
		// Remove # prefix if present
		url = strings.TrimPrefix(url, "#")

		if recIndex, ok := imageMap[url]; ok {
			// MOBI 1-based relative index (relative to FirstImageIndex)
			finalIndex := uint32(recIndex + 1) //nolint:gosec // Limit verified
			// Calibre replaces src with recindex attribute
			return fmt.Sprintf("recindex=%c%05d%c", quote, finalIndex, quote)
		}
		return match
	})
}

// resolveFileposLinks replaces href="#ID" with filepos=OFFSET for Kindle internal navigation
// This is a multi-pass process to ensure correct offset calculation after string length changes
func resolveFileposLinks(content string) string {
	// Pass 1: Collect all href="#ID" matches and their anchor IDs
	hrefMatches := hrefHashRe.FindAllStringSubmatch(content, -1)
	if len(hrefMatches) == 0 {
		return content
	}

	// Pass 2: Replace all href="#X" with placeholder "filepos=Q0000000000Q" (same length for any anchor ID)
	// We use placeholder value 0 first, which keeps consistent length
	placeholderContent := hrefHashRe.ReplaceAllStringFunc(content, func(match string) string {
		quote := match[5]
		return fmt.Sprintf("filepos=%c%010d%c", quote, 0, quote)
	})

	// Pass 3: Find anchor positions in the modified content
	anchorMap := make(map[string]int)
	for _, match := range anchorIDRe.FindAllStringSubmatchIndex(placeholderContent, -1) {
		if len(match) >= 4 {
			anchorID := placeholderContent[match[2]:match[3]]
			position := match[0]
			anchorMap[anchorID] = position
		}
	}

	// Pass 4: Replace placeholder filepos values with actual positions
	idx := 0
	result := fileposZeRe.ReplaceAllStringFunc(placeholderContent, func(match string) string {
		if idx >= len(hrefMatches) {
			return match
		}
		anchorID := hrefMatches[idx][1]
		idx++

		quote := match[8]
		if pos, ok := anchorMap[anchorID]; ok {
			return fmt.Sprintf("filepos=%c%010d%c", quote, pos, quote)
		}
		return match
	})

	return result
}

// splitTextRecords splits text into ~4KB records, preserving UTF-8 character boundaries,
// and adds trailing bytes for ExtraRecordFlags
func splitTextRecords(data []byte) [][]byte {
	var records [][]byte

	const recordSize = 4096
	i := 0
	for i < len(data) {
		end := i + recordSize
		if end > len(data) {
			end = len(data)
		}

		// Adjust end to avoid splitting UTF-8 multibyte characters
		end = findUTF8SafeBoundary(data, i, end)

		record := data[i:end]
		// No trailing bytes - ExtraRecordFlags=0 means no extra data
		records = append(records, record)

		i = end
	}

	return records
}
