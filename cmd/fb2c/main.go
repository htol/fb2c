// fb2c converts FB2 (FictionBook 2.0) files to MOBI or EPUB format.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/htol/fb2c"
	"github.com/htol/fb2c/mobi"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.BoolVar(&debug, "d", false, "Enable debug output (shorthand)")

	flag.Usage = printUsage
	flag.Parse()

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)

	switch cmd {
	case "convert":
		fs := flag.NewFlagSet("convert", flag.ExitOnError)
		if err := fs.Parse(flag.Args()[1:]); err != nil {
			os.Exit(1)
		}

		args := fs.Args()
		if len(args) < 2 {
			logger.Error("convert requires input and output paths")
			printUsage()
			os.Exit(1)
		}

		options := mobi.WriteOptions{
			CompressionType: mobi.NoCompression,
			WithEXTH:        true,
			GenerateTOC:     true,
			Logger:          logger,
		}

		convertCmd(args[0], args[1], options)

	case "metadata":
		fs := flag.NewFlagSet("metadata", flag.ExitOnError)
		if err := fs.Parse(flag.Args()[1:]); err != nil {
			os.Exit(1)
		}

		args := fs.Args()
		if len(args) < 1 {
			logger.Error("metadata requires input path")
			printUsage()
			os.Exit(1)
		}
		extractMetadataCmd(args[0])

	case "help", "-h", "--help":
		printUsage()

	default:
		logger.Error("unknown command", "command", cmd)
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

	compression := options.CompressionType == mobi.PalmDOCCompression

	converter := fb2c.NewConverter()
	converter.SetOptions(fb2c.ConvertOptions{
		MobiType:    "old",
		Compression: compression,
		Logger:      options.Logger,
	})

	err := converter.Convert(inputPath, outputPath)
	if err != nil {
		options.Logger.Error("conversion failed", "error", err)
		os.Exit(1)
	}

	options.Logger.Info("Converted file", "input", inputPath, "output", outputPath)
}

func extractMetadataCmd(inputPath string) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	metadata, err := fb2c.ExtractMetadata(inputPath)
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

func printUsage() {
	fmt.Print(`fb2c - FB2 to MOBI/EPUB Converter

Usage:
  fb2c [--debug|-d] convert <input.fb2> <output.(mobi|epub)>   Convert FB2 to MOBI or EPUB
  fb2c [--debug|-d] metadata <input.fb2>                        Extract and display metadata
  fb2c help                                                   Show this help

Global Options:
  --debug, -d    Enable debug output for troubleshooting (can be placed before or after command)

Output Format:
  The format is automatically detected from output file extension:
  - .mobi - MOBI format (joint MOBI 6 + KF8 by default)
  - .epub - EPUB format

`)
}
