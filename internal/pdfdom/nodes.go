package pdfdom

// Translator resolves i18n keys to their translated strings.
// Implemented by *invoice.I18n; defined here to avoid an import cycle.
type Translator interface {
	T(key string) string
	TWithVars(key string, vars map[string]string) string
}

// PDFNode is implemented by all declarative document/text tree nodes.
type PDFNode interface {
	isPDFNode()
}

// PDFNodeAttribute defines an element attribute (name/value).
type PDFNodeAttribute struct {
	Name  string
	Value string
}

// PDFTextNode is a declarative text node that can contain child text nodes.
type PDFTextNode struct {
	Text     string
	I18nKey  string
	I18nVars map[string]string
	Style    *PDFTextStyle
	Children []PDFNode
	// ChildLineBreaks stores whether a line break should be inserted before a
	// child at the same index in Children.
	ChildLineBreaks []bool
}

func (*PDFTextNode) isPDFNode() {}

// PDFText is kept as a compatibility alias for existing call sites.
type PDFText = PDFTextNode

// PDFDocumentNode is the root node and may contain element and text nodes.
type PDFDocumentNode struct {
	Style           *PDFTextStyle
	Children        []PDFNode
	ChildLineBreaks []bool
}

func (*PDFDocumentNode) isPDFNode() {}

// Add appends a child node inline (no implicit newline).
func (t *PDFTextNode) Add(child PDFNode) *PDFTextNode {
	if child == nil {
		return t
	}
	t.Children = append(t.Children, child)
	t.ChildLineBreaks = append(t.ChildLineBreaks, false)
	return t
}

// AddLine appends a child node and inserts an implicit newline before it.
func (t *PDFTextNode) AddLine(child PDFNode) *PDFTextNode {
	if child == nil {
		return t
	}
	t.Children = append(t.Children, child)
	t.ChildLineBreaks = append(t.ChildLineBreaks, true)
	return t
}

// Add appends a child node inline (no implicit newline).
func (d *PDFDocumentNode) Add(child PDFNode) *PDFDocumentNode {
	if child == nil {
		return d
	}
	d.Children = append(d.Children, child)
	d.ChildLineBreaks = append(d.ChildLineBreaks, false)
	return d
}

// AddLine appends a child node and inserts an implicit newline before it.
func (d *PDFDocumentNode) AddLine(child PDFNode) *PDFDocumentNode {
	if child == nil {
		return d
	}
	d.Children = append(d.Children, child)
	d.ChildLineBreaks = append(d.ChildLineBreaks, true)
	return d
}
