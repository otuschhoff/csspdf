package main

import (
	"fmt"
	"io"

	"github.com/otuschhoff/csspdf/internal/pdfdump"
)

func runPDFDump(program string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isHelpArgument(args[0]) {
		printPDFDumpUsage(stdout, program)
		return 0
	}
	if len(args) != 1 {
		printPDFDumpUsage(stderr, program)
		return 2
	}
	if err := pdfdump.DumpPDF(args[0]); err != nil {
		fmt.Fprintf(stderr, "Error dumping PDF: %v\n", err)
		return 1
	}
	return 0
}

func printPDFDumpUsage(w io.Writer, program string) {
	fmt.Fprintf(w, "Usage: %s <pdf-file>\n", program)
	fmt.Fprintln(w, "\nDisplays PDF object streams with bounded decompression.")
}
