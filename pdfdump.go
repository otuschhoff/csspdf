package main

import (
	"fmt"
	"io"
	"os"

	"github.com/otuschhoff/csspdf/internal/pdfdump"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: pdfdump <pdf-file>")
		fmt.Fprintln(stderr, "\nDisplays PDF object streams with bounded decompression.")
		return 2
	}
	if err := pdfdump.DumpPDF(args[0]); err != nil {
		fmt.Fprintf(stderr, "Error dumping PDF: %v\n", err)
		return 1
	}
	return 0
}
