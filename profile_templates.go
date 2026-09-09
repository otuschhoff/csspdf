package csspdf

import (
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	"github.com/otuschhoff/gofpdf"
)

func legacyProfileTemplateFactories() map[string]pdfrender.TemplateFactory {
	ringLogo := func(pdf *gofpdf.Fpdf) gofpdf.Template {
		return pdfrender.CreateRingLogoTemplate(pdf, pdfrender.LogoBaseRadius, pdfrender.LogoTplCenter, pdfrender.LogoTplCenter)
	}
	return map[string]pdfrender.TemplateFactory{
		"ringlogo":  ringLogo,
		"ring-logo": ringLogo,
		"ring_logo": ringLogo,
	}
}
