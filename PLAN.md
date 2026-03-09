# Plan: Golang PDF Invoice Generator

## Overview
Create a Golang application that generates PDF invoices matching the output of `mkdocPdfKit.js`. The app will read invoice data from per-invoice JSON files (like `RA-2026-01.json`) instead of using the Q() function to query Excel/ODS files. It will continue to use shared configuration files like `myCompany.json`, `myStyle.json`, and resources like `Unterschrift.png`.

## 1. Project Structure

```
golang-invoice/
├── cmd/
│   └── invoice-gen/
│       └── main.go              # CLI entry point
├── internal/
│   └── invoice/
│       ├── models.go            # Data structures
│       ├── pdf_generator.go    # Core PDF generation
│       ├── table_renderer.go   # Table rendering logic
│       ├── formatter.go         # Number/date formatters
│       ├── i18n.go              # Internationalization
│       └── zugferd.go           # ZUGFeRD/Factur-X XML generation
├── configs/
│   ├── myCompany.json           # Company details
│   ├── myStyle.json             # Styling configuration
│   └── customers.json           # Customer database (optional)
├── resources/
│   ├── Unterschrift.png         # Signature image
│   ├── Futura.ttc               # Custom fonts
│   └── locales/
│       ├── de.json
│       └── en.json
├── invoices/
│   └── RA-2026-01.json          # Invoice data files
├── output/
│   └── generated PDFs here
└── go.mod
```

## 2. Core Dependencies

- **PDF Generation**: `github.com/otuschhoff/gofpdf` (local at ../gofpdf) or `github.com/signintech/gopdf`
  - Alternative: `github.com/unidoc/unipdf` (commercial but more feature-rich)
- **XML Generation** (for ZUGFeRD): `encoding/xml` (stdlib)
- **JSON Parsing**: `encoding/json` (stdlib)
- **Date/Time**: `time` package (stdlib)
- **i18n**: `github.com/nicksnyder/go-i18n/v2` or custom implementation
- **CLI**: `github.com/spf13/cobra` (optional, for better CLI)

## 3. Data Models (`models.go`)

```go
type Invoice struct {
    Invoice      InvoiceDetails      `json:"invoice"`
    Customer     CustomerDetails     `json:"customer"`
    Orders       map[string]Order    `json:"orders"`
    Quotes       map[string]Quote    `json:"quotes"`
    WorkEntries  []WorkEntry         `json:"workEntries"`
    ExportedAt   time.Time           `json:"exportedAt"`
}

type InvoiceDetails struct {
    ID           string              `json:"id"`
    Date         string              `json:"date"`
    Customer     string              `json:"customer"`
    Description  string              `json:"description"`
    Gross        float64             `json:"gross"`
    Net          float64             `json:"net"`
    VAT          float64             `json:"vat"`
    DueDate      string              `json:"dueDate"`
    Documents    []Document          `json:"documents"`
    Notes        string              `json:"notes"`
}

type CustomerDetails struct {
    Code         string              `json:"code"`
    Name         string              `json:"name"`
    Address      Address             `json:"address"`
    Contact      Contact             `json:"contact"`
    Invoicing    Invoicing           `json:"invoicing"`
    Defaults     Defaults            `json:"defaults"`
    VendorID     string              `json:"vendorId"`
    Active       bool                `json:"active"`
}

type Address struct {
    Street       string              `json:"street"`
    City         string              `json:"city"`
    PostalCode   string              `json:"postalCode"`
    Country      string              `json:"country"`
    State        string              `json:"state"`
}

type Order struct {
    ID           string              `json:"id"`
    Date         string              `json:"date"`
    Customer     string              `json:"customer"`
    Description  string              `json:"description"`
    Gross        float64             `json:"gross"`
    Net          float64             `json:"net"`
    VAT          float64             `json:"vat"`
    QuoteID      string              `json:"quoteId"`
    Documents    []Document          `json:"documents"`
}

type Quote struct {
    ID           string              `json:"id"`
    Date         string              `json:"date"`
    Customer     string              `json:"customer"`
    Title        string              `json:"title"`
    Description  string              `json:"description"`
    Gross        float64             `json:"gross"`
    Net          float64             `json:"net"`
    VAT          float64             `json:"vat"`
    ValidUntil   string              `json:"validUntil"`
    Documents    []Document          `json:"documents"`
}

type WorkEntry struct {
    Date         string              `json:"date"`
    Tasks        []Task              `json:"tasks"`
}

type Task struct {
    Customer     string              `json:"customer"`
    OrderID      string              `json:"orderId"`
    Duration     float64             `json:"duration"`
    Topics       []string            `json:"topics"`
    Billable     bool                `json:"billable"`
}

type Document struct {
    Path         string              `json:"path"`
    SHA256       string              `json:"sha256"`
    Size         int                 `json:"size"`
    MimeType     string              `json:"mimeType,omitempty"`
}

type Company struct {
    Name         string              `json:"name"`
    Suffix       string              `json:"suffix"`
    FullName     string              `json:"fullName"`
    Street       string              `json:"street"`
    PLZ          string              `json:"plz"`
    City         string              `json:"city"`
    Country      string              `json:"country"`
    Tel          string              `json:"tel"`
    Mail         string              `json:"mail"`
    VAT          string              `json:"vat"`
    Bank         BankDetails         `json:"bank"`
    FiscalID     string              `json:"fiscalId"`
    FiscalNo     string              `json:"fiscalNo"`
}

type BankDetails struct {
    Name         string              `json:"name"`
    IBAN         string              `json:"iban"`
    BIC          string              `json:"bic"`
}

type Style struct {
    FontFace         string           `json:"fontFace"`
    FontColor        string           `json:"fontColor"`
    FontColorSub     string           `json:"fontColorSub"`
    FontSize         int              `json:"fontSize"`
    FontSizeSmall    int              `json:"fontSizeSmall"`
    FontSizeTitle    int              `json:"fontSizeTitle"`
    TableCellYOffset float64          `json:"tableCellYOffset"`
    Normal           StyleVariant     `json:"normal"`
    Small            StyleVariant     `json:"small"`
    SmallGreyed      StyleVariant     `json:"smallGreyed"`
    Sub              StyleVariant     `json:"sub"`
    PageNum          StyleVariant     `json:"pageNum"`
    PageTot          StyleVariant     `json:"pageTot"`
    Footer           StyleVariant     `json:"footer"`
    Title            StyleVariant     `json:"title"`
    CompanyName      StyleVariant     `json:"companyName"`
    CompanyNameSmall StyleVariant     `json:"companyNameSmall"`
    CompanySuffix    StyleVariant     `json:"companySuffix"`
    CompanySuffixSmall StyleVariant   `json:"companySuffixSmall"`
    DocType          StyleVariant     `json:"docType"`
}

type StyleVariant struct {
    FontFace         string           `json:"fontFace"`
    FontColor        string           `json:"fontColor"`
    FontSize         int              `json:"fontSize"`
}
```

## 4. Main Components

### 4.1 PDF Generator (`pdf_generator.go`)

Core responsibilities:
- Initialize PDF document (A4 portrait: 595.28 x 841.89 pt)
- Set up fonts (Helvetica standard, Futura custom font)
- Document margins (55pt)
- Page management:
  - First page: Invoice details, items table, summary
  - Second page: Timesheet/work log
  - Third page: PO history (optional)
- Render components:
  - Logo and company header
  - Customer address block
  - Invoice metadata (ID, date, due date)
  - Invoice positions table
  - Net/VAT/Gross summary
  - Footer with bank details and page numbers
  - Signature image

Key functions:
```go
func NewPDFGenerator(invoice *Invoice, company *Company, style *Style) *PDFGenerator
func (g *PDFGenerator) Generate(outputPath string) error
func (g *PDFGenerator) renderFirstPage()
func (g *PDFGenerator) renderTimesheetPage()
func (g *PDFGenerator) renderFooter(pageNum, pageTotal int)
func (g *PDFGenerator) renderLogo(x, y, radius float64)
```

### 4.2 Table Renderer (`table_renderer.go`)

Generic table rendering engine matching the JS `renderTable()` function.

Features:
- Dynamic column widths (fixed or proportional)
- Row/column metadata (height, background color, borders)
- Cell content with styling
- Cell padding and alignment
- Multi-element cells (text with different styles)
- Header rows with different styling
- Right-aligned numbers and currency
- Currency and date formatting

```go
type Table struct {
    Width        float64
    Padding      float64
    RowHeightMin float64
    Title        string
    ColMeta      []ColumnMeta
    RowMeta      []RowMeta
    Content      [][]Cell
}

type ColumnMeta struct {
    Width        float64
}

type RowMeta struct {
    Fill         string
    Height       float64
    Stroke       string
}

type Cell struct {
    Type         string  // text, currency, date, time, float, workWeek
    Value        interface{}
    Elements     []CellElement
}

type CellElement struct {
    Text         string
    Face         string
    Size         int
    Color        string
    Align        string
    Hide         bool
    Continued    bool
    LineBreak    bool
    OffsetX      float64
}

func RenderTable(pdf *PDF, table *Table, startY float64) error
```

### 4.3 Formatters (`formatter.go`)

Locale-aware formatting for numbers, currency, and dates:

```go
type Formatter struct {
    Locale       string
    Currency     string
    DecSeparator string
    ThouSeparator string
}

func (f *Formatter) FormatCurrency(value float64) string
// German: "21.375,00 €"
// English: "€21,375.00"

func (f *Formatter) FormatFloat(value float64, decimals int) string
// German: "13,5"
// English: "13.5"

func (f *Formatter) FormatDate(date time.Time, format string) string
// ISO: "2026-03-02"
// Full: "2. März 2026" / "March 2, 2026"

func (f *Formatter) FormatDuration(hours float64) string
// "13:30" for 13.5 hours

func (f *Formatter) FormatWorkWeek(date time.Time) string
// "9.1" (week 9, day 1)
```

### 4.4 Internationalization (`i18n.go`)

Load and manage translations from JSON files:

```go
type I18n struct {
    Translations map[string]map[string]string
    Locale       string
}

func NewI18n(locale string) (*I18n, error)
func (i *I18n) LoadLocale(path string) error
func (i *I18n) T(key string) string

// Translation keys (matching JS implementation):
// - docType (quote, invoice, response)
// - descServices, manHours, manDays, dailyRate
// - date, month, workWeekShort, timesheet
// - netDue, vat, totalDue
// - greeting, invoiceIntro, invoiceOutro, closing
// - bankAccount
// - poHistTitle, poHistStatus, poHistInvoiced, poHistPayed, poHistRemaining
```

### 4.5 ZUGFeRD/Factur-X (`zugferd.go`)

Generate XRechnung-compliant XML and embed into PDF:

```go
type ZUGFeRDGenerator struct {
    Invoice      *Invoice
    Company      *Company
    Customer     *CustomerDetails
}

func (z *ZUGFeRDGenerator) GenerateXML() ([]byte, error)
func (z *ZUGFeRDGenerator) EmbedIntoPDF(pdfPath, xmlPath, outputPath string) error

// XML Structure:
// - CrossIndustryInvoice (root)
// - ExchangedDocumentContext (specification)
// - ExchangedDocument (invoice metadata)
// - SupplyChainTradeTransaction
//   - IncludedSupplyChainTradeLineItem (line items)
//   - ApplicableHeaderTradeAgreement (buyer/seller)
//   - ApplicableHeaderTradeDelivery (delivery date)
//   - ApplicableHeaderTradeSettlement (payment terms, totals)
```

## 5. Implementation Steps

### Phase 1: Foundation (Days 1-2)
1. Set up Go project structure and initialize go.mod
2. Define all data models in `models.go`
3. Implement JSON file readers:
   - `LoadInvoice(path string) (*Invoice, error)`
   - `LoadCompany(path string) (*Company, error)`
   - `LoadStyle(path string) (*Style, error)`
4. Create basic PDF output with company header
5. Test font loading (Helvetica + Futura)

### Phase 2: Core PDF Generation (Days 3-5)
1. Implement generic table renderer with:
   - Column width calculation
   - Cell rendering with padding
   - Row styling (background, borders)
   - Text alignment
2. Create formatters for currency, dates, numbers
3. Build first page layout:
   - Header with logo (renderLogo function)
   - Customer address block
   - Invoice details section (ID, date, description, due date)
   - Invoice positions table
   - Summary section (net, VAT, gross)
4. Implement custom font loading (Futura.ttc)
5. Add color support for text and fills

### Phase 3: Additional Pages (Days 6-7)
1. Footer rendering:
   - Company info line
   - Bank details line
   - Page numbers (current/total)
2. Second page: Timesheet table with:
   - Date column
   - Work week column
   - Man-hours column
   - Man-days column
   - Description column
   - Total row
3. Third page: PO history (optional)
4. Signature image embedding at bottom of first page

### Phase 4: Internationalization (Day 8)
1. Implement i18n system:
   - Load translation files (de.json, en.json)
   - Translation lookup function
   - Format locale-specific numbers/dates
2. Create translation files with all required keys
3. Apply translations throughout PDF generation
4. Test German and English outputs

### Phase 5: ZUGFeRD/Factur-X (Days 9-10)
1. XML structure generation:
   - Document context
   - Seller/buyer trade parties
   - Line items with tax
   - Payment terms
   - Monetary summations
2. Ghostscript integration:
   - Call gs command to embed XML
   - PDF/A-3 compliance
   - Color profile handling
3. Validate XML against XRechnung schema
4. Test XML attachment to PDF

### Phase 6: Polish & Testing (Days 11-12)
1. CLI implementation:
   - Argument parsing (`-i`, `-o`, `-company`, `-style`, `-locale`)
   - Help text
   - Version info
2. Error handling:
   - File not found
   - Invalid JSON
   - Missing required fields
   - PDF generation errors
3. Input validation:
   - Required fields present
   - Valid dates
   - Positive amounts
4. Output file naming:
   - `YYYY-MM-DD - Invoice ID (Description).pdf`
5. Test suite:
   - Unit tests for formatters
   - Integration tests with sample invoices
   - Visual comparison with JS output
6. Documentation (README.md, usage examples)

## 6. Key Differences from JS Version

| Aspect | JavaScript (mkdocPdfKit.js) | Golang Implementation |
|--------|----------------------------|----------------------|
| Data Source | Excel/ODS files via Q() function | Per-invoice JSON files |
| Data Access | Dynamic queries with where clauses | Direct struct field access |
| Type System | Dynamic typing | Strong static typing |
| Dependencies | pdfkit, xlsx, lodash, moment | gofpdf, stdlib only |
| Excel Query | Complex Q() function with column mapping | Not needed - data pre-structured |
| Config | Hardcoded + queried from Excel | JSON configuration files |
| Execution | Node.js script | Compiled Go binary |

## 7. CLI Interface

```bash
# Basic usage (uses default config locations)
invoice-gen -i invoices/RA-2026-01.json

# Full options
invoice-gen \
  -i invoices/RA-2026-01.json \
  -o output/invoice.pdf \
  -company configs/myCompany.json \
  -style configs/myStyle.json \
  -locale de \
  -signature resources/Unterschrift.png

# Short flags
invoice-gen -i RA-2026-01.json -l en -o custom-output.pdf

# Help
invoice-gen --help

# Version
invoice-gen --version

# Build
go build -o invoice-gen cmd/invoice-gen/main.go
```

### Command-line Flags

```go
-i, --invoice string      Path to invoice JSON file (required)
-o, --output string       Output PDF path (default: auto-generated)
-c, --company string      Company JSON path (default: configs/myCompany.json)
-s, --style string        Style JSON path (default: configs/myStyle.json)
-l, --locale string       Locale (de/en) (default: from customer or "de")
    --signature string    Signature image path (default: resources/Unterschrift.png)
    --no-zugferd          Skip ZUGFeRD XML generation
-v, --verbose             Verbose output
-h, --help                Help
    --version             Version info
```

## 8. Configuration Loading Strategy

Priority order for configuration:
1. Command-line arguments (highest priority)
2. Environment variables
3. Config files in default locations
4. Embedded defaults (fallback)

```go
// Environment variables
INVOICE_COMPANY_CONFIG=/path/to/myCompany.json
INVOICE_STYLE_CONFIG=/path/to/myStyle.json
INVOICE_RESOURCES=/path/to/resources
INVOICE_LOCALE=de

// Default paths (relative to working directory)
./configs/myCompany.json
./configs/myStyle.json
./resources/Unterschrift.png
./resources/locales/{locale}.json
```

## 9. Testing Strategy

### Unit Tests
- Formatters (currency, date, float)
- I18n translation lookup
- Table width calculations
- Cell alignment logic
- Date parsing

### Integration Tests
- Complete invoice generation
- Multiple locales
- Different invoice amounts
- Various work entry counts
- Missing optional fields

### Golden File Tests
- XML output comparison
- Known-good XML files
- XRechnung validation

### Visual Tests
- Generate PDFs from test data
- Compare with JS-generated PDFs
- Manual inspection of:
  - Layout accuracy
  - Font rendering
  - Color correctness
  - Table alignment

### Test Data
```
test/fixtures/
├── invoices/
│   ├── simple.json
│   ├── multi-day.json
│   ├── english.json
│   └── german.json
├── expected/
│   ├── simple.pdf (for visual comparison)
│   └── simple.xml (for XML validation)
└── configs/
    ├── test-company.json
    └── test-style.json
```

## 10. Error Handling

Key error scenarios to handle gracefully:

```go
// File errors
- Invoice file not found
- Config file not found
- Resource (image/font) not found
- Cannot write output file

// Data errors
- Invalid JSON syntax
- Missing required fields (ID, date, customer, amounts)
- Invalid date format
- Negative or zero amounts
- Unknown customer code

// PDF errors
- Font loading failure
- Image loading failure
- Insufficient space for content
- Invalid color codes

// ZUGFeRD errors
- XML generation failure
- Ghostscript not found
- PDF embedding failure
```

Each error should include:
- Clear error message
- Context (which file, which field)
- Suggested fix
- Exit code (for scripting)

## 11. Future Enhancements

### Phase 2 Features
- **Web API**: REST API for invoice generation
- **Database Backend**: Store invoices in PostgreSQL/SQLite
- **Template System**: Multiple invoice templates
- **Batch Processing**: Generate multiple invoices at once
- **Watch Mode**: Auto-regenerate on file changes

### Phase 3 Features
- **Interactive CLI**: Prompt for missing fields
- **Invoice Preview**: View without saving
- **Email Delivery**: Send via SMTP
- **Archive Management**: Organize past invoices
- **Reporting**: Summary statistics

### Phase 4 Features
- **Web UI**: Browser-based invoice creation
- **Customer Portal**: Customers view their invoices
- **Payment Integration**: Stripe/PayPal links
- **Recurring Invoices**: Automatic generation
- **Multi-currency**: Support USD, GBP, etc.

## 12. Performance Considerations

- **PDF Generation**: Target < 1 second per invoice
- **Memory Usage**: Keep < 50MB for typical invoice
- **Concurrent Processing**: Support batch generation with goroutines
- **Caching**: Cache fonts and company data
- **Lazy Loading**: Load translations only for active locale

## 13. Compatibility Notes

The Golang implementation should produce PDFs that are:
- **Visually identical** to JS version (within 1-2pt)
- **ZUGFeRD-compliant** (pass validation tools)
- **Readable** with same fonts and colors
- **Processable** by accounting software

Areas where differences are acceptable:
- Minor font rendering variations (different PDF library)
- Slight spacing differences (< 2pt)
- Internal PDF structure (as long as output looks same)

## 14. Documentation Requirements

### README.md
- Project overview
- Installation instructions
- Quick start guide
- CLI usage examples
- Configuration guide
- Troubleshooting

### Code Documentation
- Godoc comments for all public functions
- Examples for key functions
- Architecture overview
- Package descriptions

### User Guide
- Invoice JSON structure
- Configuration options
- Styling guide
- Locale customization
- ZUGFeRD/XRechnung compliance

## 15. Development Environment

### Required Tools
- Go 1.21 or later
- Ghostscript (for ZUGFeRD)
- Git
- Make (optional, for build automation)

### IDE Setup
- VSCode with Go extension
- golangci-lint for linting
- gopls for language server

### Build System
```makefile
.PHONY: build test clean install

build:
	go build -o bin/invoice-gen cmd/invoice-gen/main.go

test:
	go test ./...

clean:
	rm -rf bin/ output/

install:
	go install ./cmd/invoice-gen

lint:
	golangci-lint run

run:
	go run cmd/invoice-gen/main.go -i invoices/RA-2026-01.json
```

## 16. Deployment

### Binary Distribution
- Single executable (no dependencies except Ghostscript)
- Multi-platform builds (Linux, macOS, Windows)
- Dockerfile for containerized deployment

### Package Formats
- `.deb` for Debian/Ubuntu
- `.rpm` for RedHat/CentOS
- Homebrew formula for macOS
- Chocolatey package for Windows

## Summary

This plan provides a complete roadmap for creating a production-ready Golang PDF invoice generator that closely matches the functionality and output of the existing JavaScript implementation. The key advantage is eliminating the Excel/ODS dependency and using structured JSON files for invoice data, while maintaining compatibility with existing configuration files and resources.

Total estimated development time: **10-12 days** for a complete, tested implementation.
