package pdfrender

// CurrentFlowBox returns the current page's content box origin and width.
func (l *LayoutPDF) CurrentFlowBox() (x, y, width float64) {
	x = l.currentMargins.Left
	y = l.currentMargins.Top
	width = l.pageWidth - l.currentMargins.Left - l.currentMargins.Right
	return x, y, width
}

// CurrentFlowBottom returns the y coordinate of the bottom of the content area.
func (l *LayoutPDF) CurrentFlowBottom() float64 {
	return l.pageHeight - l.currentMargins.Bottom
}

// StartFlow initialises page tracking and persistent normal-flow state.
func (l *LayoutPDF) StartFlow() {
	l.currentPage = 1
	l.totalPages = 1
	l.flowCursorY = 0
	l.flowBottomMargin = 0
	l.flowCursorValid = false
}

// NextFlowPage advances to the next page in the flow.
func (l *LayoutPDF) NextFlowPage() {
	if l.currentPage < 1 {
		l.currentPage = 1
	}
	l.currentPage++
	if l.currentPage > l.totalPages {
		l.totalPages = l.currentPage
	}
	l.BeginPage(l.currentPage)
	if !l.deferFlowPageNum {
		l.renderPageNum(l.currentPage, l.totalPages)
	}
}
