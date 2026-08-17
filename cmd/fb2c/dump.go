package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/htol/fb2c/mobi"
)

// dumpCmd implements `fb2c dump`: decode a MOBI file (text or --json),
// extract rawml, or compare two files record by record (--diff).
func dumpCmd(args []string) {
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON instead of text")
	rawml := fs.Bool("rawml", false, "Print the extracted book text (rawml)")
	diffMode := fs.Bool("diff", false, "Compare two MOBI files record by record")
	fs.Parse(args)

	files := fs.Args()

	if *diffMode {
		if len(files) != 2 {
			fmt.Fprintln(os.Stderr, "Error: dump --diff requires exactly two files")
			printUsage()
			os.Exit(1)
		}
		dataA, err := os.ReadFile(files[0])
		exitOnError(err)
		dataB, err := os.ReadFile(files[1])
		exitOnError(err)

		report, err := mobi.Diff(dataA, dataB)
		exitOnError(err)

		if *jsonOut {
			serializeJSON(report)
		} else {
			fmt.Print(report)
		}
		if report.FirstDivergence != nil {
			os.Exit(1) // like diff(1): nonzero exit signals a difference
		}
		return
	}

	if len(files) != 1 {
		fmt.Fprintln(os.Stderr, "Error: dump requires exactly one file")
		printUsage()
		os.Exit(1)
	}
	data, err := os.ReadFile(files[0])
	exitOnError(err)

	switch {
	case *rawml:
		text, err := mobi.ExtractRawML(data)
		exitOnError(err)
		fmt.Print(text)

	case *jsonOut:
		dump, err := mobi.ReadDump(data)
		exitOnError(err)
		serializeJSON(dump)

	default:
		dump, err := mobi.ReadDump(data)
		exitOnError(err)
		fmt.Print(dump)
	}
}

func serializeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error: JSON encoding failed: %v\n", err)
		os.Exit(1)
	}
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
