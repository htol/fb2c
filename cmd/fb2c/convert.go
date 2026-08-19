package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/htol/fb2c"
	"github.com/htol/fb2c/mobi"
)

func convertCmd(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	_ = fs.Parse(args) // ExitOnError makes the error unreachable: parse failures exit inside

	cmdArgs := fs.Args()
	if len(cmdArgs) < 2 {
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

	runConvert(cmdArgs[0], cmdArgs[1], options)
}

func runConvert(inputPath, outputPath string, options mobi.WriteOptions) {
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
