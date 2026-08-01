package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/otuschhoff/go-dom2pdf/docflowpdf"
	"github.com/otuschhoff/go-dom2pdf/internal/pdfdump"
)

const version = "0.1.0"

const (
	defaultLocale         = "de"
	defaultCurrencyCode   = "EUR"
	defaultPageMarginTop  = 30.0
	defaultPageMarginLeft = 55.0
)

// RenderInvoiceOptions holds all parameters for the invoice PDF rendering use case.
type RenderInvoiceOptions struct {
	OutputPath      string
	PageFormat      string
	PageOrientation string
}

// Run executes the CLI for a specific program name with argv without the
// executable name itself.
func Run(programName string, args []string) int {
	if len(args) < 1 {
		printUsage(os.Stderr, programName)
		return 2
	}

	switch args[0] {
	case "invoice":
		return runInvoice(programName, args[1:])
	case "dump-pdf":
		return runDumpPDF(programName, args[1:])
	case "version", "-version", "--version":
		fmt.Printf("%s version %s\n", programName, version)
		return 0
	case "help", "-h", "--help", "":
		printUsage(os.Stdout, programName)
		if args[0] == "" {
			return 2
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n\n", args[0])
		printUsage(os.Stderr, programName)
		return 2
	}
}

// RenderInvoice generates the invoice PDF and writes it to opts.OutputPath.
func RenderInvoice(opts RenderInvoiceOptions) error {
	return renderInvoicePDF(opts.OutputPath, opts.PageFormat, opts.PageOrientation)
}

func renderInvoicePDF(outputPath, pageFormat, pageOrientation string) error {
	baseDir, err := resolveTemplateBaseDir()
	if err != nil {
		return err
	}

	if pageFormat == "" {
		pageFormat = docflowpdf.DefaultPageFormat
	}
	if pageOrientation == "" {
		pageOrientation = docflowpdf.PageOrientationPortrait
	}

	return docflowpdf.Render(
		outputPath,
		docflowpdf.WithAssetBaseDir(baseDir),
		docflowpdf.WithI18nTemplateMacros(true),
		docflowpdf.WithPageFormat(pageFormat),
		docflowpdf.WithPageOrientation(pageOrientation),
		docflowpdf.WithDefaultLocale(defaultLocale),
		docflowpdf.WithDefaultCurrencyCode(defaultCurrencyCode),
		docflowpdf.WithPageMarginsTopBottom(defaultPageMarginTop),
		docflowpdf.WithPageMarginsLeftRight(defaultPageMarginLeft),
		docflowpdf.WithFuncMapFactoryEx(docflowpdf.DefaultTemplateFuncMapWithContext),
	)
}

func resolveTemplateBaseDir() (string, error) {
	candidates := []string{
		filepath.Join("examples", "invoice", "templates"),
		filepath.Join("..", "examples", "invoice", "templates"),
		filepath.Join("..", "..", "examples", "invoice", "templates"),
		"templates",
	}

	for _, candidate := range candidates {
		dir := filepath.Clean(candidate)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir, nil
		}
	}

	return "", os.ErrNotExist
}

func printUsage(w io.Writer, programName string) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s <subcommand> [options]\n", programName)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  invoice     Generate intro layout plus table with service line items")
	fmt.Fprintln(w, "  dump-pdf    Display PDF structure with binary streams hidden")
	fmt.Fprintln(w, "  version     Print version and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Use '%s <subcommand> -h' for command-specific options.\n", programName)
}

func runInvoice(programName string, args []string) int {
	cmd := flag.NewFlagSet("invoice", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	var (
		outputPath      = cmd.String("o", "output/invoice.pdf", "Output PDF path")
		pageFormat      = cmd.String("page-format", docflowpdf.DefaultPageFormat, "Named page format (e.g. A4, A3, A5, letter, legal)")
		pageOrientation = cmd.String("page-orientation", docflowpdf.PageOrientationPortrait, "Page orientation: portrait or landscape")
	)

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintf(os.Stderr, "  %s invoice [options]\n", programName)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		cmd.PrintDefaults()
	}

	if err := cmd.Parse(args); err != nil {
		return 2
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}

	if err := RenderInvoice(RenderInvoiceOptions{
		OutputPath:      *outputPath,
		PageFormat:      *pageFormat,
		PageOrientation: *pageOrientation,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering invoice PDF: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Invoice PDF generated successfully: %s\n", *outputPath)
	return 0
}

func runDumpPDF(programName string, args []string) int {
	dumpCmd := flag.NewFlagSet("dump-pdf", flag.ContinueOnError)
	dumpCmd.SetOutput(os.Stderr)

	dumpCmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintf(os.Stderr, "  %s dump-pdf <file.pdf>\n", programName)
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

func main() {
	os.Exit(Run("gen-example", os.Args[1:]))
}
