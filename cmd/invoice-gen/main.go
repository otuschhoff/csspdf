package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/otuschhoff/invoice-gen/internal/invoice"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "full":
		os.Exit(runFull(os.Args[2:]))
	case "totals":
		os.Exit(runTotals(os.Args[2:]))
	case "dump-pdf":
		os.Exit(runDumpPDF(os.Args[2:]))
	case "version", "-version", "--version":
		fmt.Printf("invoice-gen version %s\n", version)
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
	fmt.Fprintln(w, "  invoice-gen <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  full        Generate full invoice PDF output")
	fmt.Fprintln(w, "  render-logo Generate a PDF containing only the ring logo")
	fmt.Fprintln(w, "  intro       Generate letter-type layout plus intro text block")
	fmt.Fprintln(w, "  totals      Generate intro layout plus table with service line items")
	fmt.Fprintln(w, "  letter-type Generate letter layout with document type header and info box")
	fmt.Fprintln(w, "  letter      Generate pageNum layout plus letter address block")
	fmt.Fprintln(w, "  pageNum     Generate logo+footer PDF with page numbers on pages > 1")
	fmt.Fprintln(w, "  logo-and-footer Generate a PDF with title logo and footer template")
	fmt.Fprintln(w, "  footer      Generate a PDF containing only the footer template")
	fmt.Fprintln(w, "  dump-pdf    Display PDF structure with binary streams hidden")
	fmt.Fprintln(w, "  version     Print version and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Use 'invoice-gen <subcommand> -h' for command-specific options.")
}

func runTotals(args []string) int {
	cmd := flag.NewFlagSet("totals", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath  = cmd.String("o", "output/totals.pdf", "Output PDF path")
		companyPath = cmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = cmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = cmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		pageCount   = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen totals [options]")
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

	if err := invoice.RenderTotalsPDF(*outputPath, *companyPath, *pageWidth, *pageHeight, *pageCount); err != nil {
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
		fmt.Fprintln(os.Stderr, "  invoice-gen dump-pdf <file.pdf>")
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

	if err := invoice.DumpPDF(pdfPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error dumping PDF: %v\n", err)
		return 1
	}

	return 0
}

func runFull(args []string) int {
	fullCmd := flag.NewFlagSet("full", flag.ContinueOnError)
	fullCmd.SetOutput(os.Stderr)

	// Define full subcommand flags
	var (
		invoicePath = fullCmd.String("i", "", "Path to invoice JSON file (required)")
		outputPath  = fullCmd.String("o", "", "Output PDF path (default: auto-generated)")
		companyPath = fullCmd.String("company", "configs/myCompany.json", "Company JSON path")
		stylePath   = fullCmd.String("style", "configs/myStyle.json", "Style JSON path")
		locale      = fullCmd.String("locale", "", "Locale (de/en, default: from customer)")
		showVersion = fullCmd.Bool("version", false, "Show version")
		verbose     = fullCmd.Bool("v", false, "Verbose output")
	)

	fullCmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen full -i <invoice.json> [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fullCmd.PrintDefaults()
	}

	if err := fullCmd.Parse(args); err != nil {
		return 2
	}

	// Show version
	if *showVersion {
		fmt.Printf("invoice-gen version %s\n", version)
		return 0
	}

	// Validate required flags
	if *invoicePath == "" {
		fmt.Fprintf(os.Stderr, "Error: invoice path (-i) is required\n\n")
		fullCmd.Usage()
		return 2
	}

	// Log if verbose
	log := func(format string, args ...interface{}) {
		if *verbose {
			fmt.Printf(format+"\n", args...)
		}
	}

	// Load invoice
	log("Loading invoice from %s", *invoicePath)
	inv, err := invoice.LoadInvoice(*invoicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading invoice: %v\n", err)
		return 1
	}
	log("Loaded invoice %s", inv.Invoice.ID)

	// Load company
	log("Loading company from %s", *companyPath)
	company, err := invoice.LoadCompany(*companyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading company: %v\n", err)
		return 1
	}
	log("Loaded company %s", company.Name)

	// Load style
	log("Loading style from %s", *stylePath)
	style := invoice.LoadStyleOrDefault(*stylePath)
	log("Loaded style configuration")

	// Determine locale
	if *locale == "" {
		*locale = inv.Customer.Defaults.Language
		if *locale == "" {
			*locale = "de"
		}
	}
	log("Using locale: %s", *locale)

	// Generate output path if not specified
	if *outputPath == "" {
		// Format: YYYY-MM-DD - Invoice ID (Description).pdf
		outputDir := "output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			return 1
		}

		filename := fmt.Sprintf("%s - Invoice %s (%s).pdf",
			inv.Invoice.Date,
			inv.Invoice.ID,
			sanitizeFilename(inv.Invoice.Description))
		*outputPath = filepath.Join(outputDir, filename)
	}

	log("Output path: %s", *outputPath)

	// Generate PDF
	log("Generating PDF...")
	generator, err := invoice.NewPDFGenerator(inv, company, style, *locale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing PDF generator: %v\n", err)
		return 1
	}
	if err := generator.Generate(*outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Invoice generated successfully: %s\n", *outputPath)
	return 0
}

// sanitizeFilename removes characters that are problematic in filenames
func sanitizeFilename(s string) string {
	// Replace problematic characters
	replacements := map[rune]rune{
		'/':  '-',
		'\\': '-',
		':':  '-',
		'*':  '-',
		'?':  '-',
		'"':  '-',
		'<':  '-',
		'>':  '-',
		'|':  '-',
	}

	result := make([]rune, 0, len(s))
	for _, r := range s {
		if replacement, ok := replacements[r]; ok {
			result = append(result, replacement)
		} else {
			result = append(result, r)
		}
	}

	// Truncate if too long
	str := string(result)
	if len(str) > 100 {
		str = str[:100]
	}

	return str
}
