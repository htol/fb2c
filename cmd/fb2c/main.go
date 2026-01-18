// fb2c converts FB2 (FictionBook 2.0) files to MOBI or EPUB format.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/htol/fb2c"
	"github.com/htol/fb2c/mobi"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var debugMode bool
	var args []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--debug", "-d":
			debugMode = true
		default:
			args = append(args, arg)
		}
	}

	if debugMode {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "convert":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: convert requires input and output paths\n")
			printUsage()
			os.Exit(1)
		}

		// Check environment variable for compression setting
		// FB2C_COMPRESSION=1 or empty means enabled (default)
		// FB2C_COMPRESSION=0 means disabled
		compressionEnabled := true // Default is enabled
		if env := os.Getenv("FB2C_COMPRESSION"); env == "0" {
			compressionEnabled = false
		}

		// Create write options with proper compression setting
		// Note: We create options manually instead of using SetDebug()
		// because SetDebug() would reset CompressionType to default
		options := mobi.WriteOptions{
			CompressionType: mobi.NoCompression,
			WithEXTH:        true,
			GenerateTOC:     true,
		}
		if compressionEnabled {
			options.CompressionType = mobi.PalmDOCCompression
		}

		for i := 3; i < len(args); i++ {
			switch args[i] {
			case "--debug", "-d":
				debugMode = true
			}
		}

		if debugMode {
			options.Debug = true
		}

		convertCmd(args[1], args[2], options)

	case "metadata":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: metadata requires input path\n")
			printUsage()
			os.Exit(1)
		}
		metadataCmd(args[1])

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		printUsage()
		os.Exit(1)
	}
}

func convertCmd(inputPath, outputPath string, options mobi.WriteOptions) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create output directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Determine compression setting from mobi options
	// If compressionType is PalmDOC, enable compression; otherwise disable
	compressionEnabled := options.CompressionType == mobi.PalmDOCCompression

	err := fb2c.ConvertFileWithOptions(inputPath, outputPath, fb2c.ConvertOptions{
		MobiType:    "old",
		Compression: compressionEnabled,
		NoInlineTOC: !options.GenerateTOC,
		Debug:       options.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: conversion failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Converted %s -> %s\n", inputPath, outputPath)
}

func metadataCmd(inputPath string) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	metadata, err := fb2c.ExtractMetadata(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to extract metadata: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Title: %s\n", metadata.Title)
	fmt.Printf("Authors: %s\n", joinStrings(metadata.Authors, ", "))
	fmt.Printf("Publisher: %s\n", metadata.Publisher)
	fmt.Printf("ISBN: %s\n", metadata.ISBN)
	fmt.Printf("Year: %s\n", metadata.Year)
	fmt.Printf("Language: %s\n", metadata.Language)

	if metadata.Series != "" {
		if metadata.SeriesIndex > 0 {
			fmt.Printf("Series: %s (#%d)\n", metadata.Series, metadata.SeriesIndex)
		} else {
			fmt.Printf("Series: %s\n", metadata.Series)
		}
	}

	if len(metadata.Genres) > 0 {
		fmt.Printf("Genres: %s\n", joinStrings(metadata.Genres, ", "))
	}

	if metadata.Annotation != "" {
		fmt.Printf("\nAnnotation:\n%s\n", metadata.Annotation)
	}

	if len(metadata.Cover) > 0 {
		fmt.Printf("\nCover: %s (%d bytes, %s)\n", metadata.CoverID, len(metadata.Cover), metadata.CoverExt)
	}
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func printUsage() {
	fmt.Println("fb2c - FB2 to MOBI/EPUB Converter")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  fb2c [--debug|-d] convert <input.fb2> <output.(mobi|epub)>   Convert FB2 to MOBI or EPUB")
	fmt.Println("  fb2c [--debug|-d] metadata <input.fb2>                        Extract and display metadata")
	fmt.Println("  fb2c help                                                   Show this help")
	fmt.Println("")
	fmt.Println("Global Options:")
	fmt.Println("  --debug, -d    Enable debug output for troubleshooting (can be placed before or after command)")
	fmt.Println("")
	fmt.Println("Output Format:")
	fmt.Println("  The format is automatically detected from output file extension:")
	fmt.Println("  - .mobi - MOBI format (joint MOBI 6 + KF8 by default)")
	fmt.Println("  - .epub - EPUB format")
	fmt.Println("")
	fmt.Println("Environment:")
	fmt.Println("  FB2C_MOBI_TYPE     MOBI format: old, new, both (default: old)")
	fmt.Println("  FB2C_COMPRESSION   Enable compression: 0, 1 (default: 1)")
	fmt.Println("  FB2C_NO_TOC       Skip inline TOC: 0, 1 (default: 0)")
}
