package docflowpdf

import "fmt"

type Section struct {
	Template    string        `json:"template"`
	Transformer string        `json:"transformer"`
	Payload     PayloadConfig `json:"payload"`
}

type PayloadConfig struct {
	IncludeSource bool              `json:"includeSource"`
	Runtime       map[string]string `json:"runtime"`
	I18nVars      map[string]string `json:"i18nVars"`
	Static        map[string]any    `json:"static"`
	LocalePath    string            `json:"localePath"`
}

type Flow struct {
	MainFlow   []Section `json:"mainFlow"`
	PageNumber Section   `json:"pageNumber"`
}

type Assets struct {
	HTML       string
	CSS        string
	Flow       Flow
	SourceData map[string]any
}

func (a Assets) Validate() error {
	if a.HTML == "" {
		return fmt.Errorf("template HTML must not be empty")
	}
	if len(a.Flow.MainFlow) == 0 {
		return fmt.Errorf("flow must include at least one mainFlow section")
	}
	if a.Flow.PageNumber.Template == "" || a.Flow.PageNumber.Transformer == "" {
		return fmt.Errorf("flow must define pageNumber template and transformer")
	}
	for _, section := range a.Flow.MainFlow {
		if section.Template == "" || section.Transformer == "" {
			return fmt.Errorf("every mainFlow section must define template and transformer")
		}
	}
	if len(a.SourceData) == 0 {
		return fmt.Errorf("source data must not be empty")
	}
	return nil
}
