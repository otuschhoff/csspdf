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
	case "render-logo":
		os.Exit(runRenderLogo(os.Args[2:]))
	case "letter-type":
		os.Exit(runLetterType(os.Args[2:]))
	case "letter":
		os.Exit(runLetter(os.Args[2:]))
	case "pageNum":
		os.Exit(runPageNum(os.Args[2:]))
	case "logo-and-footer":
		os.Exit(runLogoAndFooter(os.Args[2:]))
	case "footer":
		os.Exit(runRenderFooter(os.Args[2:]))
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

func runRenderLogo(args []string) int {
	logoCmd := flag.NewFlagSet("render-logo", flag.ContinueOnError)
	logoCmd.SetOutput(os.Stderr)

	var (
		outputPath = logoCmd.String("o", "output/ring-logo.pdf", "Output PDF path")
		pageWidth  = logoCmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight = logoCmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		x          = logoCmd.Float64("x", -1, "Logo center X in points (default: centered)")
		y          = logoCmd.Float64("y", -1, "Logo center Y in points (default: centered)")
		radius     = logoCmd.Float64("r", 16.5, "Logo radius in points")
		renderBg   = logoCmd.Bool("bg", false, "Render only background radial shadow")
		renderFg   = logoCmd.Bool("fg", false, "Render only foreground ring with red linear gradient")
	)

	logoCmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen render-logo [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		logoCmd.PrintDefaults()
	}

	if err := logoCmd.Parse(args); err != nil {
		return 2
	}

	if *pageWidth <= 0 || *pageHeight <= 0 || *radius <= 0 {
		fmt.Fprintln(os.Stderr, "Error: page size and radius must be greater than zero")
		return 2
	}

	if *x < 0 {
		*x = *pageWidth / 2
	}
	if *y < 0 {
		*y = *pageHeight / 2
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}

	// Default to rendering both if neither flag is specified
	showBg := *renderBg || (!*renderBg && !*renderFg)
	showFg := *renderFg || (!*renderBg && !*renderFg)

	if err := invoice.RenderLogoPDF(*outputPath, *pageWidth, *pageHeight, *x, *y, *radius, showBg, showFg); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering logo PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Logo PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runRenderFooter(args []string) int {
	footerCmd := flag.NewFlagSet("footer", flag.ContinueOnError)
	footerCmd.SetOutput(os.Stderr)

	var (
		outputPath  = footerCmd.String("o", "output/footer.pdf", "Output PDF path")
		companyPath = footerCmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = footerCmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = footerCmd.Float64("page-height", invoice.DocHeight, "Page height in points")
	)

	footerCmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen footer [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		footerCmd.PrintDefaults()
	}

	if err := footerCmd.Parse(args); err != nil {
		return 2
	}

	if *pageWidth <= 0 || *pageHeight <= 0 {
		fmt.Fprintln(os.Stderr, "Error: page size must be greater than zero")
		return 2
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}

	if err := invoice.RenderFooterPDF(*outputPath, *companyPath, *pageWidth, *pageHeight); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering footer PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Footer PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runLogoAndFooter(args []string) int {
	cmd := flag.NewFlagSet("logo-and-footer", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath  = cmd.String("o", "output/logo-and-footer.pdf", "Output PDF path")
		companyPath = cmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = cmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = cmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		pageCount   = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen logo-and-footer [options]")
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

	if err := invoice.RenderLogoAndFooterPDF(*outputPath, *companyPath, *pageWidth, *pageHeight, *pageCount); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering logo-and-footer PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Logo+Footer PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runPageNum(args []string) int {
	cmd := flag.NewFlagSet("pageNum", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath  = cmd.String("o", "output/pageNum.pdf", "Output PDF path")
		companyPath = cmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = cmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = cmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		pageCount   = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen pageNum [options]")
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

	if err := invoice.RenderPageNumPDF(*outputPath, *companyPath, *pageWidth, *pageHeight, *pageCount); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering pageNum PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ PageNum PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runLetterType(args []string) int {
	cmd := flag.NewFlagSet("letter-type", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath  = cmd.String("o", "output/letter-type.pdf", "Output PDF path")
		companyPath = cmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = cmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = cmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		pageCount   = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen letter-type [options]")
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

	if err := invoice.RenderLetterTypePDF(*outputPath, *companyPath, *pageWidth, *pageHeight, *pageCount); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering letter-type PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Letter-type PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runLetter(args []string) int {
	cmd := flag.NewFlagSet("letter", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath  = cmd.String("o", "output/letter.pdf", "Output PDF path")
		companyPath = cmd.String("company", "configs/myCompany.json", "Company JSON path")
		pageWidth   = cmd.Float64("page-width", invoice.DocWidth, "Page width in points")
		pageHeight  = cmd.Float64("page-height", invoice.DocHeight, "Page height in points")
		pageCount   = cmd.Int("pages", 2, "Number of pages to generate")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  invoice-gen letter [options]")
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

	if err := invoice.RenderLetterPDF(*outputPath, *companyPath, *pageWidth, *pageHeight, *pageCount); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering letter PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Letter PDF generated successfully: %s\n", *outputPath)
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
