# Office and Business Document Suite

This gallery exercises the renderer with 20 synthetic, non-production documents. It contains 17 successful examples and three intentional failures. The cases progress from simple correspondence to long legal documents, landscape catalogs, and a high-density pagination workload.

## Run the suite

From the repository root:

```bash
go run ./cmd/csspdf gen-example office-suite
```

Successful PDFs are written to `.build/output/office-suite`. A deterministic
PNG brand mark is generated under `.build/output/office-suite/images` so image
loading is tested without committing an opaque or externally licensed binary.

Useful variants:

```bash
# Discover all cases and complexity levels.
go run ./cmd/csspdf gen-example office-suite -list

# Render one successful case.
go run ./cmd/csspdf gen-example office-suite -case 12-board-pack -o .build/output/board-demo

# Verify one intentional failure.
go run ./cmd/csspdf gen-example office-suite -case 19-invalid-colspan
```

The command exits successfully for an intentional failure only when the renderer returns the expected error. A missing error or a different error makes the suite fail.

## Case inventory

| ID | Document | Focus |
|---|---|---|
| `01-internal-memo` | Internal memo | Compact metadata and callout |
| `02-business-letter` | Business letter | Formal typography and spacing |
| `03-meeting-agenda` | Executive agenda | Colored cover band and structured schedule |
| `04-receipt` | Retail receipt | Narrow layout, raster image, currency |
| `05-sales-quote` | Sales quote | Commercial line items and totals |
| `06-purchase-order` | Purchase order | Dense six-column procurement table |
| `07-tax-invoice` | Tax invoice | Wrapped cells, tax summary, payment box |
| `08-delivery-note` | Delivery note | Warehouse identifiers and signatures |
| `09-expense-report` | Expense report | Categories, receipts, certification |
| `10-timesheet` | Timesheet | Duration formatting and approval area |
| `11-project-status` | Status dashboard | KPI blocks, milestones, risk cards |
| `12-board-pack` | Board pack | Cover, forced pages, decisions |
| `13-service-contract` | Services contract | Long-form legal typography and signatures |
| `14-eula` | EULA | Dense clauses, bullets, acceptance notice |
| `15-annual-report` | Annual report | Image, cover, KPI and history tables |
| `16-product-catalog` | Product catalog | Landscape page, fixed columns, color swatches |
| `17-engine-stress` | Capacity exercise | 180 rows, repeated headers, 12 narrative pages |
| `18-invalid-font-size` | Fault injection | Rejects non-finite span sizing |
| `19-invalid-colspan` | Fault injection | Rejects impossible table geometry |
| `20-missing-image` | Fault injection | Rejects unavailable resources |

## Structure

- `catalog.json` is the executable inventory and expected-outcome contract.
- `templates.html` contains one named template per case plus shared header, footer, and page-number fragments.
- `styles.css` demonstrates the supported CSS subset with Helvetica, Times, and Courier families, page controls, alignment, borders, fills, and color systems.
- `data/*.json` keeps business content independent from presentation.
- The stress rows are generated deterministically by the CLI to avoid storing repetitive fixture data.

All names, addresses, account references, and transactions are fictional. Domains use `.invalid` where contact details appear.