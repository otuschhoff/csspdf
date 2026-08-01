package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	invoiceexample "github.com/otuschhoff/go-dom2pdf/examples/invoice"
	"github.com/otuschhoff/go-dom2pdf/internal/pdfdump"
	templateload "github.com/otuschhoff/go-dom2pdf/internal/templating"
)

const version = "0.1.0"

// Page dimension defaults matching the underlying layout engine.
const (
	DocWidth  = templateload.A4Width
	DocHeight = templateload.A4Height
)

// RenderTotalsOptions holds all parameters for the totals PDF rendering use case.
type RenderTotalsOptions struct {
	OutputPath string
	PageWidth  float64
	PageHeight float64
	PageCount  int
}

// RenderTotals generates the totals PDF and writes it to opts.OutputPath.
func RenderTotals(opts RenderTotalsOptions) error {
	return invoiceexample.RenderTotalsPDF(
		opts.OutputPath,
		opts.PageWidth,
		opts.PageHeight,
		opts.PageCount,
	)
}

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "totals":
		os.Exit(runTotals(os.Args[2:]))
	case "dump-pdf":
		os.Exit(runDumpPDF(os.Args[2:]))
	case "version", "-version", "--version":
		fmt.Printf("go-dom2pdf version %s\n", version)
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	case "":
		printUsage(os.Stderr)
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  go-dom2pdf <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  totals      Generate intro layout plus table with service line items")
	fmt.Fprintln(w, "  dump-pdf    Display PDF structure with binary streams hidden")
	fmt.Fprintln(w, "  version     Print version and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Use 'go-dom2pdf <subcommand> -h' for command-specific options.")
}

func runTotals(args []string) int {
	cmd := flag.NewFlagSet("totals", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath = cmd.String("o", "output/totals.pdf", "Output PDF path")
		pageWidth  = cmd.Float64("page-width", DocWidth, "Page width in points")
		pageHeight = cmd.Float64("page-height", DocHeight, "Page height in points")
		pageCount  = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  go-dom2pdf totals [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		cmd.PrintDefaults()
	}

	if err := cmd.Parse(args); err != nil {
		return 2
	}

	if *pageWidth <= 0 || *pageHeight <= 0 || *pageCount < 1 {
		fmt.Fprintln(os.Stderr, "Error: page size must be greater than zero and pages must be at least 1")
		return 2
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}

	if err := RenderTotals(RenderTotalsOptions{
		OutputPath: *outputPath,
		PageWidth:  *pageWidth,
		PageHeight: *pageHeight,
		PageCount:  *pageCount,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering totals PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Totals PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runDumpPDF(args []string) int {
	dumpCmd := flag.NewFlagSet("dump-pdf", flag.ContinueOnError)
	dumpCmd.SetOutput(os.Stderr)

	dumpCmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  go-dom2pdf dump-pdf <file.pdf>")
	}

	if err := dumpCmd.Parse(args); err != nil {
		return 2
	}

	if len(dumpCmd.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: PDF file path is required\n\n")
		dumpCmd.Usage()
		return 2
	}

	pdfPath := dumpCmd.Args()[0]

	if err := pdfdump.DumpPDF(pdfPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error dumping PDF: %v\n", err)
		return 1
	}

	return 0
}
