package invoice

// TemplateRenderData contains the pure business data required to build
// invoice/timesheet template payloads.
type TemplateRenderData struct {
	Company   CompanyProfile
	Recipient RecipientProfile
	Invoice   InvoiceProfile
	LineItem  LineItemProfile
	Timesheet []TimesheetEntry
}

type CompanyProfile struct {
	Name            string
	Description     string
	Street          string
	CityAndCode     string
	TelContact      string
	EmailContact    string
	VATNumber       string
	BankName        string
	BankBIC         string
	BankIBAN        string
	VendorID        string
}

type RecipientProfile struct {
	Line1 string
	Line2 string
	Line3 string
	Line4 string
}

type InvoiceProfile struct {
	ID       string
	Subject  string
	Currency string
	NetTotal float64
	VATTotal float64
	GrossTotal float64
}

type LineItemProfile struct {
	ID          string
	Description string
	SubItems    []string
	ManDays     float64
	DayRate     float64
	Total       float64
}

type TimesheetEntry struct {
	Date        string
	Hours       float64
	Days        float64
	Description string
}
