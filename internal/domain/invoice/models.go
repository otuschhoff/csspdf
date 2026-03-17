// Package invoice holds the pure business domain models for invoice processing.
// It has no dependencies on PDF rendering, layout, or I/O concerns.
package invoice

import "time"

type Invoice struct {
	Invoice     InvoiceDetails
	Customer    CustomerDetails
	Orders      map[string]Order
	Quotes      map[string]Quote
	WorkEntries []WorkEntry
	ExportedAt  time.Time
}

type InvoiceDetails struct {
	ID          string
	Date        string
	Customer    string
	Description string
	Gross       float64
	Net         float64
	VAT         float64
	DueDate     string
	Documents   []Document
	Notes       string
}

type CustomerDetails struct {
	Code      string
	Name      string
	Address   Address
	Contact   Contact
	Invoicing Invoicing
	Defaults  Defaults
	VendorID  string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Address struct {
	Street     string
	City       string
	PostalCode string
	Country    string
	State      string
}

type Contact struct {
	Email         string
	Website       string
	ContactPerson string
}

type Invoicing struct {
	ContactPerson string
	Email         string
}

type Defaults struct {
	QuoteInterval        string
	InvoiceInterval      string
	NetPaymentDue        int
	QuoteBindingDuration int
	Language             string
	Currency             string
	TaxRate              float64
}

type Order struct {
	ID          string
	Date        string
	Customer    string
	Description string
	Gross       float64
	Net         float64
	VAT         float64
	QuoteID     string
	Documents   []Document
}

type Quote struct {
	ID          string
	Date        string
	Customer    string
	Title       string
	Description string
	Gross       float64
	Net         float64
	VAT         float64
	ValidUntil  string
	Documents   []Document
}

type WorkEntry struct {
	Date  string
	Tasks []Task
}

type Task struct {
	Customer string
	OrderID  string
	Duration float64
	Topics   []string
	Billable bool
}

type Document struct {
	Path     string
	SHA256   string
	Size     int
	MimeType string
}
