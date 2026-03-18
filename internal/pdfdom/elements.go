package pdfdom

import "fmt"

// ValueFormatter formats typed domain values for rendering.
// Implemented by *invoice.Formatter; defined here to avoid an import cycle.
type ValueFormatter interface {
	FormatCurrency(value float64) string
	FormatDate(dateStr string, long bool) string
	FormatDuration(hours float64) string
	FormatFloat(value float64, decimals int) string
}

// PDFElementNode is implemented by typed HTML-like container elements.
type PDFElementNode interface {
	PDFNode
	ElementType() string
	ElementStyle() *PDFTextStyle
	ElementChildren() []PDFNode
	ElementChildLineBreaks() []bool
	ElementAttributes() []PDFNodeAttribute
	SetAttribute(name, value string) PDFElementNode
	Attribute(name string) (string, bool)
	Add(child PDFNode) PDFElementNode
	AddLine(child PDFNode) PDFElementNode
	validateChild(child PDFNode) error
}

type baseElementNode struct {
	owner           PDFElementNode
	style           *PDFTextStyle
	children        []PDFNode
	childLineBreaks []bool
	attributes      []PDFNodeAttribute
}

func (b *baseElementNode) ElementStyle() *PDFTextStyle { return b.style }
func (b *baseElementNode) ElementChildren() []PDFNode  { return b.children }
func (b *baseElementNode) ElementChildLineBreaks() []bool {
	return b.childLineBreaks
}
func (b *baseElementNode) ElementAttributes() []PDFNodeAttribute { return b.attributes }

func (b *baseElementNode) SetAttribute(name, value string) PDFElementNode {
	if b == nil || b.owner == nil || name == "" {
		return b.owner
	}
	for idx := range b.attributes {
		if b.attributes[idx].Name == name {
			b.attributes[idx].Value = value
			return b.owner
		}
	}
	b.attributes = append(b.attributes, PDFNodeAttribute{Name: name, Value: value})
	return b.owner
}

func (b *baseElementNode) Attribute(name string) (string, bool) {
	if b == nil || name == "" {
		return "", false
	}
	for _, attr := range b.attributes {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

func (b *baseElementNode) Add(child PDFNode) PDFElementNode {
	if b == nil || b.owner == nil || child == nil {
		return b.owner
	}
	if err := b.owner.validateChild(child); err != nil {
		panic(err)
	}
	b.children = append(b.children, child)
	b.childLineBreaks = append(b.childLineBreaks, false)
	return b.owner
}

func (b *baseElementNode) AddLine(child PDFNode) PDFElementNode {
	if b == nil || b.owner == nil || child == nil {
		return b.owner
	}
	if err := b.owner.validateChild(child); err != nil {
		panic(err)
	}
	b.children = append(b.children, child)
	b.childLineBreaks = append(b.childLineBreaks, true)
	return b.owner
}

// --- Element type declarations ---

type ElemDiv struct{ baseElementNode }
type ElemSpan struct{ baseElementNode }
type ElemBr struct{ baseElementNode }
type ElemH1 struct{ baseElementNode }
type ElemH2 struct{ baseElementNode }
type ElemH3 struct{ baseElementNode }
type ElemUl struct{ baseElementNode }
type ElemOl struct{ baseElementNode }
type ElemLi struct{ baseElementNode }
type ElemImg struct{ baseElementNode }
type ElemUseTemplate struct{ baseElementNode }
type ElemCreateTemplate struct{ baseElementNode }

// ElemCurrencyValue formats a float64 as a locale-aware currency string.
type ElemCurrencyValue struct {
	baseElementNode
	Value float64
}

// ElemDateValue formats an ISO-8601 date string ("2006-01-02").
// Attribute "long" = "true" selects the long date format.
type ElemDateValue struct {
	baseElementNode
	Value string
}

// ElemDurationValue formats hours as "H:MM" duration text.
type ElemDurationValue struct {
	baseElementNode
	Value float64
}

// ElemManDaysValue formats a man-days quantity with locale-aware decimal
// formatting and an optional " PT" (Personentage) unit suffix.
// Attribute "unit" = "true" appends the unit suffix.
type ElemManDaysValue struct {
	baseElementNode
	Value float64
}

type ElemTable struct{ baseElementNode }
type ElemColgroup struct{ baseElementNode }
type ElemCol struct{ baseElementNode }
type ElemThead struct{ baseElementNode }
type ElemTbody struct{ baseElementNode }
type ElemTr struct{ baseElementNode }
type ElemTd struct{ baseElementNode }
type ElemTh struct{ baseElementNode }

// --- isPDFNode implementations ---

func (*ElemDiv) isPDFNode()            {}
func (*ElemSpan) isPDFNode()           {}
func (*ElemBr) isPDFNode()             {}
func (*ElemH1) isPDFNode()             {}
func (*ElemH2) isPDFNode()             {}
func (*ElemH3) isPDFNode()             {}
func (*ElemUl) isPDFNode()             {}
func (*ElemOl) isPDFNode()             {}
func (*ElemLi) isPDFNode()             {}
func (*ElemImg) isPDFNode()            {}
func (*ElemUseTemplate) isPDFNode()    {}
func (*ElemCreateTemplate) isPDFNode() {}
func (*ElemCurrencyValue) isPDFNode()  {}
func (*ElemDateValue) isPDFNode()      {}
func (*ElemDurationValue) isPDFNode()  {}
func (*ElemManDaysValue) isPDFNode()   {}
func (*ElemTable) isPDFNode()          {}
func (*ElemColgroup) isPDFNode()       {}
func (*ElemCol) isPDFNode()            {}
func (*ElemThead) isPDFNode()          {}
func (*ElemTbody) isPDFNode()          {}
func (*ElemTr) isPDFNode()             {}
func (*ElemTd) isPDFNode()             {}
func (*ElemTh) isPDFNode()             {}

// --- ElementType implementations ---

func (e *ElemDiv) ElementType() string            { return "div" }
func (e *ElemSpan) ElementType() string           { return "span" }
func (e *ElemBr) ElementType() string             { return "br" }
func (e *ElemH1) ElementType() string             { return "h1" }
func (e *ElemH2) ElementType() string             { return "h2" }
func (e *ElemH3) ElementType() string             { return "h3" }
func (e *ElemUl) ElementType() string             { return "ul" }
func (e *ElemOl) ElementType() string             { return "ol" }
func (e *ElemLi) ElementType() string             { return "li" }
func (e *ElemImg) ElementType() string            { return "img" }
func (e *ElemUseTemplate) ElementType() string    { return "use-template" }
func (e *ElemCreateTemplate) ElementType() string { return "create-template" }
func (e *ElemCurrencyValue) ElementType() string  { return "currency-value" }
func (e *ElemDateValue) ElementType() string      { return "date-value" }
func (e *ElemDurationValue) ElementType() string  { return "duration-value" }
func (e *ElemManDaysValue) ElementType() string   { return "man-days-value" }
func (e *ElemTable) ElementType() string          { return "table" }
func (e *ElemColgroup) ElementType() string       { return "colgroup" }
func (e *ElemCol) ElementType() string            { return "col" }
func (e *ElemThead) ElementType() string          { return "thead" }
func (e *ElemTbody) ElementType() string          { return "tbody" }
func (e *ElemTr) ElementType() string             { return "tr" }
func (e *ElemTd) ElementType() string             { return "td" }
func (e *ElemTh) ElementType() string             { return "th" }

// --- Constructors ---

func NewElemDiv() *ElemDiv {
	n := &ElemDiv{}
	n.owner = n
	return n
}

func NewElemTable() *ElemTable {
	n := &ElemTable{}
	n.owner = n
	return n
}

func NewElemColgroup() *ElemColgroup {
	n := &ElemColgroup{}
	n.owner = n
	return n
}

func NewElemCol() *ElemCol {
	n := &ElemCol{}
	n.owner = n
	return n
}

func NewElemThead() *ElemThead {
	n := &ElemThead{}
	n.owner = n
	return n
}

func NewElemTbody() *ElemTbody {
	n := &ElemTbody{}
	n.owner = n
	return n
}

func NewElemTr() *ElemTr {
	n := &ElemTr{}
	n.owner = n
	return n
}

func NewElemTd() *ElemTd {
	n := &ElemTd{}
	n.owner = n
	return n
}

func NewElemTh() *ElemTh {
	n := &ElemTh{}
	n.owner = n
	return n
}

func NewElemSpan() *ElemSpan {
	n := &ElemSpan{}
	n.owner = n
	return n
}

func NewElemBr() *ElemBr {
	n := &ElemBr{}
	n.owner = n
	return n
}

func NewElemH1() *ElemH1 {
	n := &ElemH1{}
	n.owner = n
	n.style = &PDFTextStyle{FontSize: 20, FontStyle: "B", FontStyleSet: true, LineHeight: 1.2}
	return n
}

func NewElemH2() *ElemH2 {
	n := &ElemH2{}
	n.owner = n
	n.style = &PDFTextStyle{FontSize: 16, FontStyle: "B", FontStyleSet: true, LineHeight: 1.2}
	return n
}

func NewElemH3() *ElemH3 {
	n := &ElemH3{}
	n.owner = n
	n.style = &PDFTextStyle{FontSize: 12, FontStyle: "B", FontStyleSet: true, LineHeight: 1.2}
	return n
}

func NewElemUl() *ElemUl {
	n := &ElemUl{}
	n.owner = n
	return n
}

func NewElemOl() *ElemOl {
	n := &ElemOl{}
	n.owner = n
	return n
}

func NewElemLi() *ElemLi {
	n := &ElemLi{}
	n.owner = n
	return n
}

func NewElemImg() *ElemImg {
	n := &ElemImg{}
	n.owner = n
	return n
}

func NewElemUseTemplate() *ElemUseTemplate {
	n := &ElemUseTemplate{}
	n.owner = n
	return n
}

func NewElemCreateTemplate() *ElemCreateTemplate {
	n := &ElemCreateTemplate{}
	n.owner = n
	return n
}

func NewElemCurrencyValue(value float64) *ElemCurrencyValue {
	n := &ElemCurrencyValue{Value: value}
	n.owner = n
	return n
}

func NewElemDateValue(dateStr string) *ElemDateValue {
	n := &ElemDateValue{Value: dateStr}
	n.owner = n
	return n
}

func NewElemDurationValue(hours float64) *ElemDurationValue {
	n := &ElemDurationValue{Value: hours}
	n.owner = n
	return n
}

func NewElemManDaysValue(days float64) *ElemManDaysValue {
	n := &ElemManDaysValue{Value: days}
	n.owner = n
	return n
}

// --- Format methods for value elements ---

// Format returns the locale-formatted currency string.
func (e *ElemCurrencyValue) Format(f ValueFormatter) string {
	if f == nil {
		return fmt.Sprintf("%.2f", e.Value)
	}
	return f.FormatCurrency(e.Value)
}

// Format returns the formatted date, using long format when the "long"
// attribute is set to "true".
func (e *ElemDateValue) Format(f ValueFormatter) string {
	if f == nil {
		return e.Value
	}
	long := false
	if v, ok := e.Attribute("long"); ok && v == "true" {
		long = true
	}
	return f.FormatDate(e.Value, long)
}

// Format returns the duration as "H:MM".
func (e *ElemDurationValue) Format(f ValueFormatter) string {
	if f == nil {
		return fmt.Sprintf("%g", e.Value)
	}
	return f.FormatDuration(e.Value)
}

// Format returns the man-days value formatted with locale decimal separators.
// When the "unit" attribute is "true" a " PT" suffix is appended.
func (e *ElemManDaysValue) Format(f ValueFormatter) string {
	var formatted string
	if f == nil {
		formatted = fmt.Sprintf("%g", e.Value)
	} else {
		formatted = f.FormatFloat(e.Value, 2)
	}
	if v, ok := e.Attribute("unit"); ok && v == "true" {
		formatted += " PT"
	}
	return formatted
}

// --- validateChild implementations ---

func (e *ElemDiv) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("div child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemSpan) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("span child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemH1) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("h1 child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemH2) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("h2 child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemH3) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("h3 child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemUl) validateChild(child PDFNode) error {
	if _, ok := child.(*ElemLi); ok {
		return nil
	}
	return fmt.Errorf("ul child must be ElemLi")
}

func (e *ElemOl) validateChild(child PDFNode) error {
	if _, ok := child.(*ElemLi); ok {
		return nil
	}
	return fmt.Errorf("ol child must be ElemLi")
}

func (e *ElemLi) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("li child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemBr) validateChild(_ PDFNode) error {
	return fmt.Errorf("br may not have children")
}

func (e *ElemImg) validateChild(_ PDFNode) error {
	return fmt.Errorf("img may not have children")
}

func (e *ElemUseTemplate) validateChild(_ PDFNode) error {
	return fmt.Errorf("use-template may not have children")
}

func (e *ElemCreateTemplate) validateChild(child PDFNode) error {
	if _, ok := child.(PDFElementNode); !ok {
		return fmt.Errorf("create-template children must be element nodes")
	}
	return nil
}

func (e *ElemCurrencyValue) validateChild(_ PDFNode) error {
	return fmt.Errorf("currency-value may not have children")
}

func (e *ElemDateValue) validateChild(_ PDFNode) error {
	return fmt.Errorf("date-value may not have children")
}

func (e *ElemDurationValue) validateChild(_ PDFNode) error {
	return fmt.Errorf("duration-value may not have children")
}

func (e *ElemManDaysValue) validateChild(_ PDFNode) error {
	return fmt.Errorf("man-days-value may not have children")
}

func (e *ElemTable) validateChild(child PDFNode) error {
	switch child.(type) {
	case *ElemColgroup, *ElemThead, *ElemTbody, *ElemTr:
		return nil
	default:
		return fmt.Errorf("table child must be ElemColgroup, ElemThead, ElemTbody, or ElemTr")
	}
}

func (e *ElemColgroup) validateChild(child PDFNode) error {
	if _, ok := child.(*ElemCol); ok {
		return nil
	}
	return fmt.Errorf("colgroup child must be ElemCol")
}

func (e *ElemCol) validateChild(child PDFNode) error {
	return fmt.Errorf("col may not have children")
}

func (e *ElemThead) validateChild(child PDFNode) error {
	if _, ok := child.(*ElemTr); ok {
		return nil
	}
	return fmt.Errorf("thead child must be ElemTr")
}

func (e *ElemTbody) validateChild(child PDFNode) error {
	if _, ok := child.(*ElemTr); ok {
		return nil
	}
	return fmt.Errorf("tbody child must be ElemTr")
}

func (e *ElemTr) validateChild(child PDFNode) error {
	switch child.(type) {
	case *ElemTd, *ElemTh:
		return nil
	}
	return fmt.Errorf("tr child must be ElemTd or ElemTh")
}

func (e *ElemTd) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("td child must be PDFElementNode or PDFTextNode")
	}
}

func (e *ElemTh) validateChild(child PDFNode) error {
	switch child.(type) {
	case PDFElementNode, *PDFTextNode:
		return nil
	default:
		return fmt.Errorf("th child must be PDFElementNode or PDFTextNode")
	}
}
