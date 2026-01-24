package kf8

import (
	"fmt"
)

// prepareContent prepares the HTML content for KF8 (including chunking if enabled)
func (w *Writer) prepareContent(originalContent string) (string, error) {
	if w.options.EnableChunking {
		if err := w.skeleton.ChunkHTML(originalContent); err != nil {
			return "", fmt.Errorf("failed to chunk HTML: %w", err)
		}
		w.skeleton.BuildHierarchy()
		kf8Content := w.skeleton.AssignAIDAttributes()

		// Generate FDST from skeleton
		if w.options.GenerateFDST {
			w.fdst.GenerateFromSkeleton(w.skeleton)
		}
		return kf8Content, nil
	}
	return originalContent, nil
}
