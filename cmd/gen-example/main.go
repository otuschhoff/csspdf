package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/otuschhoff/csspdf/docflowpdf"
	"github.com/otuschhoff/csspdf/internal/pdfdump"
)

const version = "0.1.0"

const defaultCurrencyCode = "EUR"

// Run executes the CLI for a specific program name with argv without the
// executable name itself.
func Run(programName string, args []string) int {
	if len(args) < 1 {
		printUsage(os.Stderr, programName)
		return 2
	}

	switch args[0] {
	case "layered":
		return runLayered(programName, args[1:])
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

func renderLayeredPDF(outputPath string) error {
	baseDir, err := resolveLayeredTemplateBaseDir()
	if err != nil {
		return err
	}

	assetInput := docflowpdf.AssetInput{
		HTML:       docflowpdf.TextSource{FilePath: filepath.Join(baseDir, "doc.html")},
		Flow:       docflowpdf.JSONSource{FilePath: filepath.Join(baseDir, "flow.json")},
		SourceData: docflowpdf.JSONSource{FilePath: filepath.Join(baseDir, "source.json")},
		CSSLayers: []docflowpdf.CSSLayerInput{
			{Name: "corporate-base", Source: docflowpdf.TextSource{FilePath: filepath.Join(baseDir, "styles", "corporate", "base.css")}},
			{Name: "document", Source: docflowpdf.TextSource{FilePath: filepath.Join(baseDir, "styles", "document", "doc.css")}},
			{Name: "customer-override", Source: docflowpdf.TextSource{FilePath: filepath.Join(baseDir, "styles", "overrides", "customer.css")}, Optional: true},
		},
	}

	return docflowpdf.Render(
		outputPath,
		docflowpdf.WithAssetInput(assetInput),
		docflowpdf.WithDefaultLocale("en"),
		docflowpdf.WithDefaultCurrencyCode(defaultCurrencyCode),
		docflowpdf.WithFuncMapFactoryEx(docflowpdf.DefaultTemplateFuncMapWithContext),
	)
}

func resolveLayeredTemplateBaseDir() (string, error) {
	candidates := []string{
		filepath.Join("examples", "layered"),
		filepath.Join("..", "examples", "layered"),
		filepath.Join("..", "..", "examples", "layered"),
	}

	for _, candidate := range candidates {
		dir := filepath.Clean(candidate)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return filepath.Abs(dir)
		}
	}

	return "", os.ErrNotExist
}
func printUsage(w io.Writer, programName string) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s <subcommand> [options]\n", programName)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  layered     Generate layered-css concept example")
	fmt.Fprintln(w, "  dump-pdf    Display PDF structure with binary streams hidden")
	fmt.Fprintln(w, "  version     Print version and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Use '%s <subcommand> -h' for command-specific options.\n", programName)
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

func runLayered(programName string, args []string) int {
	cmd := flag.NewFlagSet("layered", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	outputPath := cmd.String("o", "output/layered.pdf", "Output PDF path")

	cmd.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintf(os.Stderr, "  %s layered [options]\n", programName)
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

	if err := renderLayeredPDF(*outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering layered example PDF: %s\n", formatCommandError(err))
		return 1
	}

	fmt.Printf("✓ Layered example PDF generated successfully: %s\n", *outputPath)
	return 0
}

func main() {
	os.Exit(Run("gen-example", os.Args[1:]))
}
