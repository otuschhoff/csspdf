# Invoice Generator

A Golang application that generates ZUGFeRD/XRechnung-compliant PDF/A-3 invoices from JSON data files. This is a Go port of the JavaScript invoice generator (`mkdocPdfKit.js`), designed for clean, tested, and maintainable code.

## Features

- ✅ **PDF Generation**: Creates professional invoices with company branding
- ✅ **Two-Page Layout**: Invoice summary on page 1, detailed timesheet on page 2
- ✅ **Invoice Positions Table**: Displays line items with ID, description, man-days, daily rate, and subtotals
- ✅ **Timesheet/Worklog**: Shows detailed work entries with date, work week, hours, and descriptions
- ✅ **Signature Embedding**: Includes digital signature image on invoices
- ✅ **Internationalization**: Supports de/en locales with translated labels
- ✅ **Locale-Aware Formatting**: Currency (21.375,50 €), dates (02.03.2026), and numbers
- ✅ **ZUGFeRD/XRechnung XML**: Generates EN16931-compliant XML invoices
- ✅ **PDF/A-3 Compliance**: Embeds ZUGFeRD XML as attachment per ISO 19005-3
- ✅ **Clean Architecture**: Modular design with separated concerns (models, loaders, generators, renderers)

## Project Structure

```
biz-invoice/
├── cmd/
│   └── invoice-gen/        # CLI entry point
│       └── main.go
├── internal/
│   └── invoice/            # Core invoice logic
│       ├── models.go       # Data structures
│       ├── loaders.go      # JSON file loaders
│       ├── pdf_generator.go # PDF rendering
│       ├── i18n.go         # Translation support
│       ├── formatter.go    # Number/date formatting
│       ├── table_renderer.go # Table rendering engine
│       └── zugferd.go      # ZUGFeRD XML generator
├── configs/                # Configuration files
│   ├── myCompany.json      # Company details
│   └── myStyle.json        # PDF styling
├── data/
│   └── i18n.json           # Unified translations (en/de)
├── resources/              # Static resources
│   └── Unterschrift.png    # Signature image
├── invoices/               # Input invoice JSON files
│   └── RA-2026-01.json     # Example invoice
├── output/                 # Generated PDFs
└── bin/                    # Compiled binary
    └── invoice-gen
```

## Installation

### Prerequisites

- Go 1.21 or later
- Local pdfa3-go library at `../pdfa3-go` (for PDF/A-3 conversion)
- Local gofpdf library at `../gofpdf` (for PDF generation)

### Build

```bash
go build -o bin/invoice-gen ./cmd/invoice-gen
```

### Dependencies

- `github.com/otuschhoff/gofpdf` - PDF generation (local at ../gofpdf)
- `github.com/example/pdfa3-go` - PDF/A-3 conversion (local)

## Usage

### Basic Usage

```bash
./bin/invoice-gen full -i invoices/RA-2026-01.json
```

### Command-Line Options

Top-level:

```
invoice-gen full [options]
invoice-gen render-logo [options]
invoice-gen version
```

`full` options:

```
-i string
    Input invoice JSON file (required)
    
-o string
    Output PDF file path (default: auto-generated in output/)
    Auto-generated format: "YYYY-MM-DD - Invoice ID (Description).pdf"
    
-company string
    Company JSON file path (default: configs/myCompany.json)
    
-style string
    Style JSON file path (default: configs/myStyle.json)
    
-locale string
    Locale for translations (default: "de")
    Available: de, en
    
-v
    Verbose output
    
-version
    Print version and exit
```

### Examples

```bash
# Generate invoice with default settings
./bin/invoice-gen full -i invoices/RA-2026-01.json

# Specify output path
./bin/invoice-gen full -i invoices/RA-2026-01.json -o output/custom-name.pdf

# Use custom company configuration
./bin/invoice-gen full -i invoices/RA-2026-01.json -company configs/company-branch.json

# Generate in English
./bin/invoice-gen full -i invoices/RA-2026-01.json -locale en

# Verbose mode for debugging
./bin/invoice-gen full -i invoices/RA-2026-01.json -v

# Render only the ring logo to a standalone PDF
./bin/invoice-gen render-logo -o output/ring-logo.pdf

# Dump PDF structure with binary streams hidden
./bin/invoice-gen dump-pdf -i output/invoice.pdf
```

## dump-pdf Command

Displays the internal structure of a PDF file with all binary streams hidden for debugging and analysis.

### Options

```
-i string
    Path to PDF file to dump (required)
```

### Examples

```bash
# Dump a generated invoice
./bin/invoice-gen dump-pdf -i output/RA-2026-01.pdf

# Dump and save to file
./bin/invoice-gen dump-pdf -i output/logo.pdf > /tmp/pdf-structure.txt

# Dump and analyze specific objects
./bin/invoice-gen dump-pdf -i output/invoice.pdf | grep -A5 "obj"
```

## Configuration Files

### Invoice JSON (`invoices/*.json`)

Contains invoice-specific data including invoice details, customer information, orders, quotes, and work entries.

Key sections:
- `invoice`: ID, date, amounts, due date
- `customer`: Name, address, contact details
- `orders`: Linked order information
- `quotes`: Linked quote details  
- `workEntries`: Billable work with tasks, duration, topics

Example structure available in `invoices/RA-2026-01.json`.

### Company JSON (`configs/myCompany.json`)

Company branding and contact information:
- Name, address, contact details
- Bank details (IBAN, BIC)
- Tax IDs (VAT, Fiscal)

### Style JSON (`configs/myStyle.json`)

PDF styling configuration:
- Font faces, sizes, colors
- Layout dimensions
- Text styles for different elements

## Features Deep Dive

### PDF Generation

The invoice generator creates a two-page PDF:

**Page 1** (Invoice Summary):
- Company logo and name
- Customer address block
- Invoice ID, date, and due date
- **Invoice positions table** with:
  - Line items (ID, description, man-days, daily rate, subtotal)
  - Summary section (net, VAT, gross)
- Signature image
- Footer with company details and page numbers

**Page 2** (Timesheet):
- Work log table showing:
  - Date, work week, hours per day
  - Man-days (8h equivalent)
  - Task descriptions from topics
  - Total hours and man-days

### Internationalization (i18n)

The `i18n.go` module provides:
- Translation loading from JSON files
- Variable replacement (`{{varName}}`)
- Locale-specific separators (decimal, thousands)

Translation keys include:
- `descServices`, `manDays`, `dailyRate`, `subTotal`
- `netDue`, `vat`, `totalDue`
- `timesheet`, `date`, `workWeekShort`, `manHours`

### Locale-Aware Formatting

The `formatter.go` module handles:
- **Currency**: `21375.50` → `"21.375,50 €"` (German)
- **Date**: `"2026-03-02"` → `"02.03.2026"`  
- **Float**: `1.687` → `"1,69"` (2 decimals)
- **Duration**: `13.5` hours → `"13:30"`
- **Work Week**: Date → `"9.1"` (week.year)

### Table Rendering

The `table_renderer.go` module provides a generic table engine:
- Column definitions with widths and alignment
- Row metadata (background colors, borders)
- Cell types: text, currency, date, float, workweek, duration
- Auto-sizing with minimum heights
- Supports bold, small text, and alignment (L/C/R)

### ZUGFeRD XML Generation

The `zugferd.go` module generates EN16931-compliant XML:
- CrossIndustryInvoice structure
- Trade parties (seller/buyer)
- Line items with pricing and quantities
- Tax calculations and monetary summation
- Payment terms and means
- Conforms to XRechnung 3.0 specification

### PDF/A-3 Conversion

Integration with `pdfa3-go` library:
1. Generate temporary PDF with gofpdf
2. Generate ZUGFeRD XML
3. Embed XML and ICC profile using pdfa3 converter
4. Output ISO 19005-3 compliant PDF/A-3

Benefits:
- Long-term archival (PDF/A-3)
- Machine-readable invoice data (ZUGFeRD XML)
- Compatible with e-invoicing systems
- Verified by German tax authorities (XRechnung)

## Development

## Tasks

Run tasks with `xc <task>`. The task definitions below are xc-compatible.

### build

Build CLI binaries into `bin/`.

```sh
go build -o bin/invoice-gen ./cmd/invoice-gen
go build -o bin/dom-parse ./cmd/dom-parse
```

### smoke

Build all packages, run tests, and generate the totals PDF. Use this as a quick sanity check after code changes.

```sh
go build ./internal/... ./cmd/...
go test ./internal/...
go build -o bin/invoice-gen ./cmd/invoice-gen
bin/invoice-gen totals
```

### render-sample

Build and render the sample invoice in verbose mode.

```sh
go build -o bin/invoice-gen ./cmd/invoice-gen
./bin/invoice-gen -i invoices/RA-2026-01.json -v
```

## Code Organization

The codebase follows clean architecture principles:

**Data Layer** (`models.go`):
- Pure data structures matching JSON schema
- No business logic

**Loading Layer** (`loaders.go`):
- File I/O and validation
- Error handling

**Business Logic**:
- `formatter.go`: Locale-specific formatting
- `i18n.go`: Translation management
- `zugferd.go`: XML generation

**Rendering Layer**:
- `table_renderer.go`: Generic table engine
- `pdf_generator.go`: PDF layout and composition

**CLI Layer** (`main.go`):
- Flag parsing
- Orchestration
- User feedback

## Testing

To test the generator:

```bash
# Build
go build -o bin/invoice-gen ./cmd/invoice-gen

# Test with sample invoice
./bin/invoice-gen -i invoices/RA-2026-01.json -v

# Check output
ls -lh output/*.pdf

# Verify PDF/A-3 compliance (requires pdfinfo or similar)
pdfinfo "output/2026-03-02 - Invoice RA-2026-01 (Work performed in 2026-02).pdf"
```

Expected output size: ~57KB (with embedded ICC profile and XML)

### Adding Translation Keys

1. Add translation entry to `data/i18n.json`:
   ```json
   {
        "myNewKey": { "en": "English text", "de": "German text" },
     ...
   }
   ```

2. Use in code:
   ```go
   text := g.i18n.T("myNewKey")
   ```

3. For variables:
   ```json
   {
     "greeting": "Hello {{name}}"
   }
   ```
   ```go
   text := g.i18n.TWithVars("greeting", map[string]string{"name": "World"})
   ```

### Customizing PDF Layout

1. Modify `configs/myStyle.json` for fonts and colors
2. Adjust table definitions in `pdf_generator.go`:
   - Column widths in `TableDef.Columns`
   - Row styling in `RowDef` (Background, Border)
   - Cell formatting in `CellDef` (Bold, Small, Align)
3. Update positioning values (Y coordinates)

## Troubleshooting

### PDF/A-3 Conversion Fails

If PDF/A-3 conversion fails, the generator falls back to regular PDF. Common issues:

- Missing ICC profile: Ensure `resources/sRGB2014.icc` exists or system profile is available
- Font embedding issues: pdfa3 converter attempts to embed fonts automatically
- Large images: May cause memory issues during conversion

To debug:
```bash
./bin/invoice-gen -i invoices/RA-2026-01.json -v
```

### Signature Image Not Found

Place `Unterschrift.png` in one of these locations:
- `resources/Unterschrift.png`
- `Unterschrift.png` (project root)

If missing, signature section is skipped silently.

### Translation Not Found

Check `data/i18n.json` contains the key with both `en` and `de` values.

### Incorrect Currency Formatting

Ensure `customerDefaults.currency` in invoice JSON matches expected value (e.g., "EUR"). Update `formatter.go` if custom currency symbols are needed.

## Roadmap

Completed features (as of Phase 4):
- [x] Project structure and models
- [x] JSON loaders with validation  
- [x] Basic PDF generation
- [x] i18n with de/en translations
- [x] Locale-aware formatting
- [x] Table rendering engine
- [x] Invoice positions table
- [x] Timesheet/worklog page
- [x] Signature embedding
- [x] ZUGFeRD XML generation
- [x] PDF/A-3 conversion

Potential future enhancements:
- [ ] Unit tests for core modules
- [ ] Integration tests
- [ ] Font subsetting for smaller PDFs
- [ ] Multiple VAT rates support
- [ ] Custom font loading (Futura)
- [ ] Batch invoice generation
- [ ] Invoice validation with ZUGFeRD validator
- [ ] Docker container for deployment
- [ ] REST API wrapper
- [ ] Web UI for invoice generation

## License

This project is proprietary software for Oliver Tuschhoff - Beratung und Training.

## Author

**Oliver Tuschhoff**  
Email: mail@oliver-tuschhoff.de  
Website: https://oliver-tuschhoff.de  

---

Generated with ❤️ by the Go Invoice Generator
