package zugferd

import (
	"encoding/xml"
	"fmt"
	"time"

	domain "github.com/otuschhoff/invoice-gen/internal/domain/invoice"
	"github.com/otuschhoff/invoice-gen/internal/format"
	invoice "github.com/otuschhoff/invoice-gen/internal/invoice"
)

// ZUGFeRDGenerator generates ZUGFeRD/XRechnung XML
type ZUGFeRDGenerator struct {
	invoice   *domain.Invoice
	company   *invoice.Company
	formatter *format.Formatter
}

// NewZUGFeRDGenerator creates a new ZUGFeRD generator
func NewZUGFeRDGenerator(inv *domain.Invoice, company *invoice.Company, formatter *format.Formatter) *ZUGFeRDGenerator {
	return &ZUGFeRDGenerator{
		invoice:   inv,
		company:   company,
		formatter: formatter,
	}
}

// Generate generates the ZUGFeRD XML
func (z *ZUGFeRDGenerator) Generate() (string, error) {
	// Parse dates
	issueDate, err := time.Parse("2006-01-02", z.invoice.Invoice.Date)
	if err != nil {
		return "", fmt.Errorf("failed to parse issue date: %w", err)
	}
	dueDate, err := time.Parse("2006-01-02", z.invoice.Invoice.DueDate)
	if err != nil {
		return "", fmt.Errorf("failed to parse due date: %w", err)
	}

	// Calculate totals and line items
	totalDays := 0.0
	for _, entry := range z.invoice.WorkEntries {
		for _, task := range entry.Tasks {
			if task.Billable {
				totalDays += task.Duration / 8.0
			}
		}
	}
	dailyRate := 0.0
	if totalDays > 0 {
		dailyRate = z.invoice.Invoice.Net / totalDays
	}

	// Build XML structure
	doc := CrossIndustryInvoice{
		XMLNsRsm: "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100",
		XMLNsRam: "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100",
		XMLNsUdt: "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100",
		XMLNsQdt: "urn:un:unece:uncefact:data:standard:QualifiedDataType:100",

		ExchangedDocumentContext: ExchangedDocumentContext{
			BusinessProcess: DocumentContextParameter{
				ID: z.invoice.Invoice.Description,
			},
			Guideline: DocumentContextParameter{
				ID: "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0",
			},
		},

		ExchangedDocument: ExchangedDocument{
			ID:       z.invoice.Invoice.ID,
			TypeCode: "380", // 380 = Invoice
			IssueDateTime: IssueDateTime{
				DateTimeString: DateTimeString{
					Format: "102",
					Value:  issueDate.Format("20060102"),
				},
			},
			IncludedNote: []Note{
				{
					Content: fmt.Sprintf("Rechnung %s - %s", z.invoice.Invoice.ID, z.invoice.Invoice.Description),
				},
			},
		},

		SupplyChainTradeTransaction: SupplyChainTradeTransaction{
			LineItems: []LineItem{
				{
					DocumentLineDocument: DocumentLineDocument{
						LineID: "1",
					},
					SpecifiedTradeProduct: SpecifiedTradeProduct{
						Name:        z.invoice.Invoice.Description,
						Description: z.invoice.Invoice.Description,
					},
					SpecifiedLineTradeAgreement: SpecifiedLineTradeAgreement{
						NetPriceProductTradePrice: NetPriceProductTradePrice{
							ChargeAmount:  fmt.Sprintf("%.2f", dailyRate),
							BasisQuantity: Quantity{UnitCode: "DAY", Value: "1"},
						},
					},
					SpecifiedLineTradeDelivery: SpecifiedLineTradeDelivery{
						BilledQuantity: Quantity{UnitCode: "DAY", Value: fmt.Sprintf("%.2f", totalDays)},
					},
					SpecifiedLineTradeSettlement: SpecifiedLineTradeSettlement{
						ApplicableTradeTax: ApplicableTradeTax{
							TypeCode:     "VAT",
							CategoryCode: "S",
							RatePercent:  fmt.Sprintf("%.0f", z.invoice.Customer.Defaults.TaxRate*100),
						},
						LineTotalAmount: fmt.Sprintf("%.2f", z.invoice.Invoice.Net),
					},
				},
			},
			ApplicableHeaderTradeAgreement: ApplicableHeaderTradeAgreement{
				BuyerReference: z.invoice.Customer.Code,
				SellerTradeParty: TradeParty{
					Name:        z.company.Name,
					Description: z.company.Suffix,
					DefinedTradeContact: TradeContact{
						PersonName: z.company.Name,
						TelephoneCommunication: Communication{
							CompleteNumber: z.company.Tel,
						},
						EmailCommunication: Communication{
							URIID: z.company.Mail,
						},
					},
					PostalTradeAddress: PostalTradeAddress{
						PostcodeCode: z.company.PLZ,
						LineOne:      z.company.Street,
						CityName:     z.company.City,
						CountryID:    z.company.Country,
					},
					SpecifiedTaxRegistration: []TaxRegistration{
						{
							ID: TaxID{
								SchemeID: "VA",
								Value:    z.company.VAT,
							},
						},
					},
				},
				BuyerTradeParty: TradeParty{
					Name: z.invoice.Customer.Name,
					PostalTradeAddress: PostalTradeAddress{
						PostcodeCode: z.invoice.Customer.Address.PostalCode,
						LineOne:      z.invoice.Customer.Address.Street,
						CityName:     z.invoice.Customer.Address.City,
						CountryID:    z.invoice.Customer.Address.Country,
					},
					SpecifiedTaxRegistration: []TaxRegistration{
						{
							ID: TaxID{
								SchemeID: "VA",
								Value:    "", // TODO: Add VatID field to CustomerDetails
							},
						},
					},
				},
			},
			ApplicableHeaderTradeDelivery: ApplicableHeaderTradeDelivery{
				ActualDeliverySupplyChainEvent: ActualDeliverySupplyChainEvent{
					OccurrenceDateTime: OccurrenceDateTime{
						DateTimeString: DateTimeString{
							Format: "102",
							Value:  issueDate.Format("20060102"),
						},
					},
				},
			},
			ApplicableHeaderTradeSettlement: ApplicableHeaderTradeSettlement{
				PaymentReference:    z.invoice.Invoice.ID,
				InvoiceCurrencyCode: z.invoice.Customer.Defaults.Currency,
				SpecifiedTradeSettlementPaymentMeans: []PaymentMeans{
					{
						TypeCode: "30", // 30 = Credit Transfer
						PayeePartyCreditorFinancialAccount: CreditorFinancialAccount{
							IBANID: z.company.Bank.IBAN,
						},
						PayeeSpecifiedCreditorFinancialInstitution: CreditorFinancialInstitution{
							BICID: z.company.Bank.BIC,
						},
					},
				},
				ApplicableTradeTax: []ApplicableTradeTax2{
					{
						CalculatedAmount: fmt.Sprintf("%.2f", z.invoice.Invoice.VAT),
						TypeCode:         "VAT",
						BasisAmount:      fmt.Sprintf("%.2f", z.invoice.Invoice.Net),
						CategoryCode:     "S",
						RatePercent:      fmt.Sprintf("%.0f", z.invoice.Customer.Defaults.TaxRate*100),
					},
				},
				SpecifiedTradePaymentTerms: []PaymentTerms{
					{
						Description: fmt.Sprintf("Zahlbar bis %s", dueDate.Format("02.01.2006")),
						DueDateTime: IssueDateTime{
							DateTimeString: DateTimeString{
								Format: "102",
								Value:  dueDate.Format("20060102"),
							},
						},
					},
				},
				SpecifiedTradeSettlementHeaderMonetarySummation: MonetarySummation{
					LineTotalAmount:     fmt.Sprintf("%.2f", z.invoice.Invoice.Net),
					TaxBasisTotalAmount: fmt.Sprintf("%.2f", z.invoice.Invoice.Net),
					TaxTotalAmount:      fmt.Sprintf("%.2f", z.invoice.Invoice.VAT),
					GrandTotalAmount:    fmt.Sprintf("%.2f", z.invoice.Invoice.Gross),
					DuePayableAmount:    fmt.Sprintf("%.2f", z.invoice.Invoice.Gross),
				},
			},
		},
	}

	// Marshal to XML
	output, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal XML: %w", err)
	}

	// Add XML declaration
	xmlStr := xml.Header + string(output)
	return xmlStr, nil
}

// XML structure types
type CrossIndustryInvoice struct {
	XMLName                     xml.Name                    `xml:"rsm:CrossIndustryInvoice"`
	XMLNsRsm                    string                      `xml:"xmlns:rsm,attr"`
	XMLNsRam                    string                      `xml:"xmlns:ram,attr"`
	XMLNsUdt                    string                      `xml:"xmlns:udt,attr"`
	XMLNsQdt                    string                      `xml:"xmlns:qdt,attr"`
	ExchangedDocumentContext    ExchangedDocumentContext    `xml:"rsm:ExchangedDocumentContext"`
	ExchangedDocument           ExchangedDocument           `xml:"rsm:ExchangedDocument"`
	SupplyChainTradeTransaction SupplyChainTradeTransaction `xml:"rsm:SupplyChainTradeTransaction"`
}

type ExchangedDocumentContext struct {
	BusinessProcess DocumentContextParameter `xml:"ram:BusinessProcessSpecifiedDocumentContextParameter"`
	Guideline       DocumentContextParameter `xml:"ram:GuidelineSpecifiedDocumentContextParameter"`
}

type DocumentContextParameter struct {
	ID string `xml:"ram:ID"`
}

type ExchangedDocument struct {
	ID            string        `xml:"ram:ID"`
	TypeCode      string        `xml:"ram:TypeCode"`
	IssueDateTime IssueDateTime `xml:"ram:IssueDateTime"`
	IncludedNote  []Note        `xml:"ram:IncludedNote"`
}

type IssueDateTime struct {
	DateTimeString DateTimeString `xml:"udt:DateTimeString"`
}

type DateTimeString struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type Note struct {
	Content string `xml:"ram:Content"`
}

type SupplyChainTradeTransaction struct {
	LineItems                       []LineItem                      `xml:"ram:IncludedSupplyChainTradeLineItem"`
	ApplicableHeaderTradeAgreement  ApplicableHeaderTradeAgreement  `xml:"ram:ApplicableHeaderTradeAgreement"`
	ApplicableHeaderTradeDelivery   ApplicableHeaderTradeDelivery   `xml:"ram:ApplicableHeaderTradeDelivery"`
	ApplicableHeaderTradeSettlement ApplicableHeaderTradeSettlement `xml:"ram:ApplicableHeaderTradeSettlement"`
}

type LineItem struct {
	DocumentLineDocument         DocumentLineDocument         `xml:"ram:AssociatedDocumentLineDocument"`
	SpecifiedTradeProduct        SpecifiedTradeProduct        `xml:"ram:SpecifiedTradeProduct"`
	SpecifiedLineTradeAgreement  SpecifiedLineTradeAgreement  `xml:"ram:SpecifiedLineTradeAgreement"`
	SpecifiedLineTradeDelivery   SpecifiedLineTradeDelivery   `xml:"ram:SpecifiedLineTradeDelivery"`
	SpecifiedLineTradeSettlement SpecifiedLineTradeSettlement `xml:"ram:SpecifiedLineTradeSettlement"`
}

type DocumentLineDocument struct {
	LineID string `xml:"ram:LineID"`
}

type SpecifiedTradeProduct struct {
	Name        string `xml:"ram:Name"`
	Description string `xml:"ram:Description"`
}

type SpecifiedLineTradeAgreement struct {
	NetPriceProductTradePrice NetPriceProductTradePrice `xml:"ram:NetPriceProductTradePrice"`
}

type NetPriceProductTradePrice struct {
	ChargeAmount  string   `xml:"ram:ChargeAmount"`
	BasisQuantity Quantity `xml:"ram:BasisQuantity"`
}

type Quantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type SpecifiedLineTradeDelivery struct {
	BilledQuantity Quantity `xml:"ram:BilledQuantity"`
}

type SpecifiedLineTradeSettlement struct {
	ApplicableTradeTax ApplicableTradeTax `xml:"ram:ApplicableTradeTax"`
	LineTotalAmount    string             `xml:"ram:SpecifiedTradeSettlementLineMonetarySummation>ram:LineTotalAmount"`
}

type ApplicableTradeTax struct {
	TypeCode     string `xml:"ram:TypeCode"`
	CategoryCode string `xml:"ram:CategoryCode"`
	RatePercent  string `xml:"ram:RateApplicablePercent"`
}

type ApplicableHeaderTradeAgreement struct {
	BuyerReference   string     `xml:"ram:BuyerReference"`
	SellerTradeParty TradeParty `xml:"ram:SellerTradeParty"`
	BuyerTradeParty  TradeParty `xml:"ram:BuyerTradeParty"`
}

type TradeParty struct {
	Name                     string             `xml:"ram:Name"`
	Description              string             `xml:"ram:Description,omitempty"`
	DefinedTradeContact      TradeContact       `xml:"ram:DefinedTradeContact,omitempty"`
	PostalTradeAddress       PostalTradeAddress `xml:"ram:PostalTradeAddress"`
	SpecifiedTaxRegistration []TaxRegistration  `xml:"ram:SpecifiedTaxRegistration"`
}

type TradeContact struct {
	PersonName             string        `xml:"ram:PersonName"`
	TelephoneCommunication Communication `xml:"ram:TelephoneUniversalCommunication"`
	EmailCommunication     Communication `xml:"ram:EmailURIUniversalCommunication"`
}

type Communication struct {
	CompleteNumber string `xml:"ram:CompleteNumber,omitempty"`
	URIID          string `xml:"ram:URIID,omitempty"`
}

type PostalTradeAddress struct {
	PostcodeCode string `xml:"ram:PostcodeCode"`
	LineOne      string `xml:"ram:LineOne"`
	CityName     string `xml:"ram:CityName"`
	CountryID    string `xml:"ram:CountryID"`
}

type TaxRegistration struct {
	ID TaxID `xml:"ram:ID"`
}

type TaxID struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

type ApplicableHeaderTradeDelivery struct {
	ActualDeliverySupplyChainEvent ActualDeliverySupplyChainEvent `xml:"ram:ActualDeliverySupplyChainEvent"`
}

type ActualDeliverySupplyChainEvent struct {
	OccurrenceDateTime OccurrenceDateTime `xml:"ram:OccurrenceDateTime"`
}

type OccurrenceDateTime struct {
	DateTimeString DateTimeString `xml:"udt:DateTimeString"`
}

type ApplicableHeaderTradeSettlement struct {
	PaymentReference                                string                `xml:"ram:PaymentReference"`
	InvoiceCurrencyCode                             string                `xml:"ram:InvoiceCurrencyCode"`
	SpecifiedTradeSettlementPaymentMeans            []PaymentMeans        `xml:"ram:SpecifiedTradeSettlementPaymentMeans"`
	ApplicableTradeTax                              []ApplicableTradeTax2 `xml:"ram:ApplicableTradeTax"`
	SpecifiedTradePaymentTerms                      []PaymentTerms        `xml:"ram:SpecifiedTradePaymentTerms"`
	SpecifiedTradeSettlementHeaderMonetarySummation MonetarySummation     `xml:"ram:SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type PaymentMeans struct {
	TypeCode                                   string                       `xml:"ram:TypeCode"`
	PayeePartyCreditorFinancialAccount         CreditorFinancialAccount     `xml:"ram:PayeePartyCreditorFinancialAccount"`
	PayeeSpecifiedCreditorFinancialInstitution CreditorFinancialInstitution `xml:"ram:PayeeSpecifiedCreditorFinancialInstitution"`
}

type CreditorFinancialAccount struct {
	IBANID string `xml:"ram:IBANID"`
}

type CreditorFinancialInstitution struct {
	BICID string `xml:"ram:BICID"`
}

type ApplicableTradeTax2 struct {
	CalculatedAmount string `xml:"ram:CalculatedAmount"`
	TypeCode         string `xml:"ram:TypeCode"`
	BasisAmount      string `xml:"ram:BasisAmount"`
	CategoryCode     string `xml:"ram:CategoryCode"`
	RatePercent      string `xml:"ram:RateApplicablePercent"`
}

type PaymentTerms struct {
	Description string        `xml:"ram:Description"`
	DueDateTime IssueDateTime `xml:"ram:DueDateDateTime"`
}

type MonetarySummation struct {
	LineTotalAmount     string `xml:"ram:LineTotalAmount"`
	TaxBasisTotalAmount string `xml:"ram:TaxBasisTotalAmount"`
	TaxTotalAmount      string `xml:"ram:TaxTotalAmount"`
	GrandTotalAmount    string `xml:"ram:GrandTotalAmount"`
	DuePayableAmount    string `xml:"ram:DuePayableAmount"`
}
