# Refactoring Plan (Phased, LLM-Executable)

## Objective
Refactor `internal/` into clear concern-based packages with small, testable files and functions, while preserving behavior and removing legacy pathways at the end.

This plan is intentionally scoped into small phases that a coding LLM can execute safely in one pass.

## Design Principles
- Single responsibility per package.
- One canonical rendering pipeline (no parallel legacy path once migration is complete).
- No package cycles; dependency direction is enforced.
- Small files and functions to keep complexity manageable.

## Size and Scope Budgets
- **Per LLM phase**: 4–8 files, ~200–600 changed LOC, max 1 behavior change.
- **Function length target**: 15–40 LOC (hard cap ~60 except parser state machines).
- **File length target**: 150–350 LOC (hard cap ~400; split above this).

## Target Package Layout

```text
internal/
  app/
    invoice/            # use-case orchestration only
  domain/
    invoice/            # pure business models/rules
  i18n/                 # locale lookup + translation
  format/               # currency/date/duration formatting
  template/
    load.go             # template execution
    docflow_parser.go   # HTML/CSS parser primitives
    css_selectors.go
    page_css.go
  pdfcore/              # PDF element tree + text engine + style merging
  pdflayout/            # page flow, table rendering, layout composition
  pdfdump/              # standalone PDF dump/inspection utility
  invoice/              # temporary compatibility façade during migration
```

## Dependency Direction (Must Hold)
- `domain` -> no internal dependencies.
- `i18n`, `format` -> may depend on `domain`, never on pdf/layout.
- `template` -> parser/template logic only, no invoice business data.
- `pdfcore` -> rendering primitives only, no app/domain orchestration.
- `pdflayout` -> may depend on `pdfcore` and `template` adapter contracts.
- `app/invoice` -> orchestrates all lower layers.
- `cmd/*` -> depends on `app/invoice` and `pdfdump` only.

## Baseline Hotspots to Split
- `internal/invoice/pdf_text.go`
- `internal/invoice/pdf_generator.go`
- `internal/invoice/pdf_dumper.go`
- `internal/invoice/html_flow_parser.go`
- `internal/invoice/table_renderer.go`

---

## Phase 0: Baseline and Safety Rails

### Goal
Lock behavior before structural refactors.

### Tasks
1. Add/refresh golden validation for `totals` output path (PDF dump/text snapshot).
2. Add a repeatable smoke command/script for:
   - `go build ./internal/... ./cmd/...`
   - `bin/invoice-gen totals`
3. Document dependency direction and phase budgets in this plan.

### Acceptance Criteria
- Build passes.
- Smoke generation passes.
- Golden artifacts unchanged from baseline.

---

## Phase 1: Extract PDF Dump (Low Risk)

### Goal
Move dumper logic first to establish migration pattern.

### Tasks
1. Move `internal/invoice/pdf_dumper.go` -> `internal/pdfdump/*`.
2. Keep temporary `invoice.DumpPDF` wrapper delegating to new package.
3. Update command wiring incrementally.

### Acceptance Criteria
- `invoice-gen dump-pdf` unchanged.
- Focused dumper tests pass (or add basic regression tests if missing).
- Full build passes.

### Legacy Note
Delete wrapper in next phase once callers are switched.

---

## Phase 2: Introduce Application Service Boundary

### Goal
Create one orchestrator entrypoint used by CLI.

### Tasks
1. Add `internal/app/invoice` service (e.g. `RenderTotals(...)`).
2. Move orchestration from `internal/invoice/pdf_generator.go` into service.
3. `cmd/invoice-gen/main.go` calls service only.

### Acceptance Criteria
- CLI behavior unchanged.
- No direct deep rendering orchestration in `cmd/`.
- Build and smoke pass.

### Legacy Note
Mark old top-level orchestration funcs as temporary compatibility layer.

---

## Phase 3: Extract PDF Core Primitives

### Goal
Split `pdf_text.go` into cohesive modules under `internal/pdfcore`.

### Tasks
1. Move node/type model (`PDFNode`, `PDFElementNode`, element structs).
2. Move style and merge logic (`PDFTextStyle`, defaults/merge).
3. Move text engine and box measurement.
4. Keep temporary aliases/adapters in `internal/invoice` for one phase.

### Suggested file split
- `internal/pdfcore/nodes.go`
- `internal/pdfcore/elements.go`
- `internal/pdfcore/styles.go`
- `internal/pdfcore/text_engine.go`
- `internal/pdfcore/measure.go`

### Acceptance Criteria
- Existing rendering outputs match baseline.
- Build and focused tests pass.

### Legacy Note
Remove aliases in Phase 4.

---

## Phase 4: Extract Layout Engine

### Goal
Move page flow and table layout out of invoice package.

### Tasks
1. Move table renderer from `internal/invoice/table_renderer.go` to `internal/pdflayout/table/*`.
2. Move page lifecycle/flow logic from `pdf_generator.go` into `internal/pdflayout/flow/*`.
3. Keep data-building and business composition out of layout package.

### Acceptance Criteria
- One layout engine path used by totals rendering.
- No duplicated flow logic across packages.
- Build + smoke + golden checks pass.

### Legacy Note
Delete old layout helpers left in `internal/invoice` once switched.

---

## Phase 5: Finalize Template Separation

### Goal
Strictly separate template generation, parsing, and mapping.

### Tasks
1. Keep template execution and CSS parsing in `internal/template`.
2. Move HTML->PDF element mapping adapter logic out of mixed invoice parser file where practical.
3. Remove invoice-level wrappers that only forward to `internal/template`.

### Acceptance Criteria
- `internal/template` remains pure parser/template layer.
- No business/i18n logic leaks into parser package.

### Legacy Note
Delete parser forwarders and dead compatibility helpers.

---

## Phase 6: Domain/Data Layer Cleanup

### Goal
Make business/data preparation pure and independently testable.

### Tasks
1. Move invoice domain models to `internal/domain/invoice`.
2. Move/clean data builders so they depend only on domain + i18n/format interfaces.
3. Isolate i18n and formatter concerns into dedicated packages.

### Acceptance Criteria
- Data builders run in unit tests without PDF/layout dependencies.
- Clear interface boundaries for translators/formatters.

---

## Phase 7: Legacy Pathway Removal (Mandatory)

### Goal
Remove all temporary compatibility pathways.

### Tasks
1. Delete deprecated wrappers, aliases, and legacy entrypoints.
2. Remove old tests tied solely to deleted pathways.
3. Search for dead symbols and remove unreachable code.

### Acceptance Criteria
- Exactly one canonical totals pipeline remains.
- No `TODO remove after migration` markers remain.
- Build/test/smoke/golden checks pass.

---

## Phase 8: Prevent Regression

### Goal
Enforce maintainability constraints in CI.

### Tasks
1. Add lint/config checks for function length, cyclomatic complexity, and max file length warnings.
2. Add architecture rule checks (dependency direction).
3. Add contributor notes for package responsibilities.

### Acceptance Criteria
- CI fails when boundaries are violated.
- Architecture and size budgets are documented and enforced.

---

## Legacy Removal Strategy (Cross-Phase)

For every extracted module:
1. Introduce wrapper/alias for one phase only.
2. Switch all call sites in the next phase.
3. Delete wrapper/alias in the immediately following phase.

No compatibility shim should survive longer than two phases.

---

## Standard LLM Task Packet Template

Each phase execution prompt should include:
1. Objective and non-goals.
2. Exact files allowed to modify.
3. Max LOC/file-count budget.
4. Required validations.
5. Expected commit message format:
   - Why
   - What changed
   - Validation
   - Next legacy cleanup step

If a task exceeds budget, split it into two sub-phases before coding.

---

## Recommended Immediate Next Step
Proceed with **Phase 5 (Finalize Template Separation)**.

## Execution Status (Updated 2026-03-17)

### Completed Phases
- **Phase 0+2** completed (`77482dc`): smoke task + app/invoice service boundary
- **Phase 1** completed (`76c8976`): PDF dump extraction into `internal/pdfdump`
- **Phase 3** completed (`5da99ba`): PDF core extraction into `internal/pdfcore`
- **Phase 4** completed (`faafb4f`): table renderer extraction into `internal/pdflayout`

### Follow-up Commits
- `ccc2e1f`: added temporary invoice-level `pdf_dumper` compatibility shim

### Phase 5 Progress
- **Phase 5a committed** (`744362b`): removed invoice-level template forwarding helpers from `html_flow_parser`, updated parser tests, and cleaned plan status.
- **Phase 5b implemented in working tree**: removed `internal/invoice/page_settings.go` compatibility wrapper; `pdf_generator` and tests now call `template.ParseCSSPageSettings` directly.

### Next Step
Commit Phase 5b and continue with remaining template/data boundary cleanup in Phase 5/6.
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
