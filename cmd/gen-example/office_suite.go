package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/otuschhoff/csspdf"
)

type officeSuiteCatalog struct {
	Cases []officeSuiteCase `json:"cases"`
}

type officeSuiteCase struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Template      string `json:"template"`
	Data          string `json:"data"`
	Complexity    string `json:"complexity"`
	Orientation   string `json:"orientation"`
	ExpectedError string `json:"expectedError"`
}

func runOfficeSuite(programName string, args []string) int {
	cmd := flag.NewFlagSet("office-suite", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)
	outputDir := cmd.String("o", ".build/output/office-suite", "Output directory")
	caseID := cmd.String("case", "", "Render one case by ID")
	list := cmd.Bool("list", false, "List cases without rendering")
	cmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s office-suite [options]\n\n", programName)
		cmd.PrintDefaults()
	}
	if err := cmd.Parse(args); err != nil {
		return 2
	}

	baseDir, err := resolveOfficeSuiteBaseDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error locating office suite: %v\n", err)
		return 1
	}
	catalog, err := loadOfficeSuiteCatalog(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading office suite: %v\n", err)
		return 1
	}
	cases := selectOfficeSuiteCases(catalog.Cases, *caseID)
	if len(cases) == 0 {
		fmt.Fprintf(os.Stderr, "Error: office-suite case %q not found\n", *caseID)
		return 2
	}
	if *list {
		listOfficeSuiteCases(os.Stdout, cases)
		return 0
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}
	logoPath := filepath.Join(*outputDir, "images", "northstar-mark.png")
	if err := writeOfficeSuiteLogo(logoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating example image: %v\n", err)
		return 1
	}

	if runOfficeSuiteCases(baseDir, *outputDir, logoPath, cases) {
		return 1
	}
	return 0
}

func runOfficeSuiteCases(baseDir, outputDir, logoPath string, cases []officeSuiteCase) bool {
	failed := false
	for _, example := range cases {
		if reportOfficeSuiteResult(outputDir, example, renderOfficeSuiteCase(baseDir, outputDir, logoPath, example)) {
			failed = true
		}
	}
	return failed
}

func reportOfficeSuiteResult(outputDir string, example officeSuiteCase, err error) bool {
	if example.ExpectedError != "" {
		if err == nil {
			fmt.Fprintf(os.Stderr, "FAIL %-24s expected error containing %q\n", example.ID, example.ExpectedError)
			return true
		}
		if !strings.Contains(err.Error(), example.ExpectedError) {
			fmt.Fprintf(os.Stderr, "FAIL %-24s wrong error: %s\n", example.ID, formatCommandError(err))
			return true
		}
		fmt.Printf("PASS %-24s caught expected error: %s\n", example.ID, example.ExpectedError)
		return false
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL %-24s %s\n", example.ID, formatCommandError(err))
		return true
	}
	fmt.Printf("PASS %-24s %s\n", example.ID, filepath.Join(outputDir, example.ID+".pdf"))
	return false
}

func listOfficeSuiteCases(out io.Writer, cases []officeSuiteCase) {
	for _, example := range cases {
		kind := "renders"
		if example.ExpectedError != "" {
			kind = "expected error"
		}
		fmt.Fprintf(out, "%-24s %-8s %-14s %s\n", example.ID, example.Complexity, kind, example.Title)
	}
}

func resolveOfficeSuiteBaseDir() (string, error) {
	for _, candidate := range []string{
		filepath.Join("examples", "office-suite"),
		filepath.Join("..", "examples", "office-suite"),
		filepath.Join("..", "..", "examples", "office-suite"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", os.ErrNotExist
}

func loadOfficeSuiteCatalog(baseDir string) (officeSuiteCatalog, error) {
	data, err := os.ReadFile(filepath.Join(baseDir, "catalog.json"))
	if err != nil {
		return officeSuiteCatalog{}, err
	}
	var catalog officeSuiteCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return officeSuiteCatalog{}, fmt.Errorf("parse catalog: %w", err)
	}
	if len(catalog.Cases) != 20 {
		return officeSuiteCatalog{}, fmt.Errorf("catalog contains %d cases, want 20", len(catalog.Cases))
	}
	seen := make(map[string]struct{}, len(catalog.Cases))
	for _, example := range catalog.Cases {
		if example.ID == "" || example.Template == "" || example.Data == "" {
			return officeSuiteCatalog{}, fmt.Errorf("case has incomplete identity: %+v", example)
		}
		if _, exists := seen[example.ID]; exists {
			return officeSuiteCatalog{}, fmt.Errorf("duplicate case ID %q", example.ID)
		}
		seen[example.ID] = struct{}{}
	}
	return catalog, nil
}

func selectOfficeSuiteCases(cases []officeSuiteCase, caseID string) []officeSuiteCase {
	if caseID == "" {
		return cases
	}
	for _, example := range cases {
		if example.ID == caseID {
			return []officeSuiteCase{example}
		}
	}
	return nil
}

func renderOfficeSuiteCase(baseDir, outputDir, logoPath string, example officeSuiteCase) error {
	dataPath := filepath.Join(baseDir, "data", example.Data)
	dataBytes, err := os.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("read source data: %w", err)
	}
	var source map[string]any
	if err := json.Unmarshal(dataBytes, &source); err != nil {
		return fmt.Errorf("parse source data: %w", err)
	}
	populateOfficeSuiteStressData(example.ID, source)
	source["AssetLogo"] = filepath.Base(logoPath)

	flow := csspdf.Flow{
		MainFlow: []csspdf.Section{{
			Template:    example.Template,
			Transformer: "generic",
			Payload:     csspdf.PayloadConfig{IncludeSource: true},
		}},
		PageNumber: csspdf.Section{
			Template:    "page-number",
			Transformer: "generic",
			Payload: csspdf.PayloadConfig{Runtime: map[string]string{
				"Page":  "page.number",
				"Total": "page.total",
			}},
		},
	}
	orientation := example.Orientation
	if orientation == "" {
		orientation = "portrait"
	}
	return csspdf.Render(filepath.Join(outputDir, example.ID+".pdf"),
		csspdf.WithAssetBaseDir(outputDir),
		csspdf.WithAssetInput(csspdf.AssetInput{
			HTML:       csspdf.TextSource{FilePath: filepath.Join(baseDir, "templates.html")},
			CSS:        csspdf.TextSource{FilePath: filepath.Join(baseDir, "styles.css")},
			Flow:       csspdf.JSONSource{Object: flow},
			SourceData: csspdf.JSONSource{Object: source},
		}),
		csspdf.WithPageOrientation(orientation),
		csspdf.WithDefaultLocale("en"),
		csspdf.WithDefaultCurrencyCode("USD"),
		csspdf.WithNow(func() time.Time { return time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC) }),
		csspdf.WithFuncMapFactoryEx(csspdf.DefaultTemplateFuncMapWithContext),
	)
}

func populateOfficeSuiteStressData(caseID string, source map[string]any) {
	if caseID != "17-engine-stress" {
		return
	}
	rows := make([]map[string]any, 0, 180)
	for index := 1; index <= 180; index++ {
		units := 1 + index%7
		rate := 37.5 + float64(index%11)*8.25
		rows = append(rows, map[string]any{
			"ID":          index,
			"Reference":   fmt.Sprintf("CAP-%04d-%c", index, 'A'+rune(index%6)),
			"Title":       fmt.Sprintf("Operational work package %03d", index),
			"Description": "Multi-line validation text exercises wrapping, row measurement, repeated headers, borders, alignment, and pagination under sustained load.",
			"Units":       units,
			"Rate":        rate,
			"Amount":      float64(units) * rate,
		})
	}
	sections := make([]map[string]any, 0, 12)
	for index := 1; index <= 12; index++ {
		sections = append(sections, map[string]any{
			"Heading": fmt.Sprintf("Capacity observation %02d", index),
			"Body":    "This deliberately long narrative repeats across forced pages to exercise page lifecycle behavior, running footers, page counters, text measurement, decoration, and deterministic output. The content is synthetic and designed only as a renderer workload.",
			"Note":    fmt.Sprintf("Checkpoint %02d: verify that this callout remains intact and that the following section starts on a fresh page.", index),
		})
	}
	source["Rows"] = rows
	source["Sections"] = sections
}

func writeOfficeSuiteLogo(filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 360, 120))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 18, G: 40, B: 55, A: 255}}, image.Point{}, draw.Src)
	colors := []color.RGBA{
		{R: 19, G: 180, B: 167, A: 255},
		{R: 244, G: 183, B: 64, A: 255},
		{R: 239, G: 108, B: 86, A: 255},
	}
	for index, fill := range colors {
		x := 28 + index*54
		draw.Draw(canvas, image.Rect(x, 24+index*8, x+38, 96-index*8), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	}
	for index := 0; index < 7; index++ {
		x := 208 + index*18
		height := 18 + index*8
		draw.Draw(canvas, image.Rect(x, 96-height, x+9, 96), &image.Uniform{C: colors[index%len(colors)]}, image.Point{}, draw.Src)
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, canvas)
}

func sortedOfficeSuiteIDs(cases []officeSuiteCase) []string {
	ids := make([]string, 0, len(cases))
	for _, example := range cases {
		ids = append(ids, example.ID)
	}
	sort.Strings(ids)
	return ids
}
