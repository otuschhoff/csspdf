package csspdf

func (a Assets) Validate() error {
	if _, err := effectiveTemplateHTML(a); err != nil {
		return err
	}
	if _, err := effectiveTemplateCSS(a); err != nil {
		return err
	}
	if err := a.Flow.Validate(); err != nil {
		return err
	}
	return nil
}
