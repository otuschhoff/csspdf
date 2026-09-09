package csspdf

func cloneFlow(flow Flow) Flow {
	return Flow{
		MainFlow:   cloneSections(flow.MainFlow),
		PageNumber: cloneSection(flow.PageNumber),
	}
}

func cloneSections(sections []Section) []Section {
	if sections == nil {
		return nil
	}
	out := make([]Section, len(sections))
	for index, section := range sections {
		out[index] = cloneSection(section)
	}
	return out
}

func cloneSection(section Section) Section {
	section.Payload.Runtime = cloneStringMap(section.Payload.Runtime)
	section.Payload.I18nVars = cloneStringMap(section.Payload.I18nVars)
	static := section.Payload.Static
	if static != nil {
		section.Payload.Static = make(map[string]any, len(static))
		for key, value := range static {
			section.Payload.Static[key] = cloneJSONValue(value)
		}
	}
	return section
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = cloneJSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			out[index] = cloneJSONValue(child)
		}
		return out
	default:
		return value
	}
}
