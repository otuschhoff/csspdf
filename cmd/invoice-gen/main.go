package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/otuschhoff/invoice-gen/internal/invoice"
)

const version = "0.1.0"

func main() {
	// Define command-line flags
	var (
		invoicePath = flag.String("i", "", "Path to invoice JSON file (required)")
		outputPath  = flag.String("o", "", "Output PDF path (default: auto-generated)")
		companyPath = flag.String("company", "configs/myCompany.json", "Company JSON path")
		stylePath   = flag.String("style", "configs/myStyle.json", "Style JSON path")
		locale      = flag.String("locale", "", "Locale (de/en, default: from customer)")
		showVersion = flag.Bool("version", false, "Show version")
		verbose     = flag.Bool("v", false, "Verbose output")
	)

	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("invoice-gen version %s\n", version)
		os.Exit(0)
	}

	// Validate required flags
	if *invoicePath == "" {
		fmt.Fprintf(os.Stderr, "Error: invoice path (-i) is required\n\n")
		flag.Usage()
		os.Exit(1)
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
		os.Exit(1)
	}
	log("Loaded invoice %s", inv.Invoice.ID)

	// Load company
	log("Loading company from %s", *companyPath)
	company, err := invoice.LoadCompany(*companyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading company: %v\n", err)
		os.Exit(1)
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
			os.Exit(1)
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
	generator := invoice.NewPDFGenerator(inv, company, style)
	if err := generator.Generate(*outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Invoice generated successfully: %s\n", *outputPath)
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
