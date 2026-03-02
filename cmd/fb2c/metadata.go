package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/htol/fb2c/fb2"
)

func metadataCmd(args []string) {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)
	fs.Parse(args)

	cmdArgs := fs.Args()
	if len(cmdArgs) < 1 {
		logger.Error("metadata requires input path")
		printUsage()
		os.Exit(1)
	}
	extractMetadataCmd(cmdArgs[0])
}

func extractMetadataCmd(inputPath string) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	metadata, err := fb2.GetMetadataFromFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to extract metadata: %v\n", err)
		os.Exit(1)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n", metadata.Title)
	fmt.Fprintf(&sb, "Authors: %s\n", strings.Join(metadata.Authors, ", "))
	fmt.Fprintf(&sb, "Publisher: %s\n", metadata.Publisher)
	fmt.Fprintf(&sb, "ISBN: %s\n", metadata.ISBN)
	fmt.Fprintf(&sb, "Year: %s\n", metadata.Year)
	fmt.Fprintf(&sb, "Language: %s\n", metadata.Language)
	fmt.Print(sb.String())

	if metadata.Series != "" {
		if metadata.SeriesIndex > 0 {
			fmt.Printf("Series: %s (#%d)\n", metadata.Series, metadata.SeriesIndex)
		} else {
			fmt.Printf("Series: %s\n", metadata.Series)
		}
	}

	if len(metadata.Genres) > 0 {
		fmt.Printf("Genres: %s\n", strings.Join(metadata.Genres, ", "))
	}

	if metadata.Annotation != "" {
		fmt.Printf("\nAnnotation:\n%s\n", metadata.Annotation)
	}

	if len(metadata.Cover) > 0 {
		fmt.Printf("\nCover: %s (%d bytes, %s)\n", metadata.CoverID, len(metadata.Cover), metadata.CoverExt)
	}
}
