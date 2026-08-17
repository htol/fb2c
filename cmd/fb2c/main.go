// fb2c converts FB2 (FictionBook 2.0) files to MOBI or EPUB format.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

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
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)

	switch cmd {
	case "convert":
		convertCmd(flag.Args()[1:])

	case "metadata":
		metadataCmd(flag.Args()[1:])

	case "dump":
		dumpCmd(flag.Args()[1:])

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`fb2c - FB2 to MOBI/EPUB Converter

Usage:
  fb2c [--debug|-d] convert <input.fb2> <output.(mobi|epub)>   Convert FB2 to MOBI or EPUB
  fb2c [--debug|-d] metadata <input.fb2>                        Extract and display metadata
  fb2c dump [--json] <file.mobi>                               Decode PalmDB/MOBI/EXTH/INDX headers and records
  fb2c dump --rawml <file.mobi>                                Extract the book text (rawml)
  fb2c dump --diff <a.mobi> <b.mobi>                           Compare two MOBI files record by record
  fb2c help                                                   Show this help

Global Options:
  --debug, -d    Enable debug output for troubleshooting (can be placed before or after command)

Output Format:
  The format is automatically detected from output file extension:
  - .mobi - MOBI format (joint MOBI 6 + KF8 by default)
  - .epub - EPUB format

`)
}
