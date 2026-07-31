package templating

import (
	"fmt"
	"regexp"
	"strings"

	css "github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
)

type PageMargins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

type PageSettings struct {
	Width   float64
	Height  float64
	Margins PageMargins
}

type pageSettingsOverride struct {
	sizeSet   bool
	width     float64
	height    float64
	topSet    bool
	rightSet  bool
	bottomSet bool
	leftSet   bool
	top       float64
	right     float64
	bottom    float64
	left      float64
}

const (
	A4Width  = 595.28
	A4Height = 841.89
)

var namedPageSizes = map[string][2]float64{
	"a0":        {2383.94, 3370.39},
	"a1":        {1683.78, 2383.94},
	"a2":        {1190.55, 1683.78},
	"a3":        {841.89, 1190.55},
	"a4":        {A4Width, A4Height},
	"a5":        {419.53, 595.28},
	"a6":        {297.64, 419.53},
	"b4":        {729.13, 1031.81},
	"b5":        {515.91, 729.13},
	"jis-b4":    {728.50, 1031.81},
	"jis-b5":    {515.91, 728.50},
	"letter":    {612.00, 792.00},
	"legal":     {612.00, 1008.00},
	"tabloid":   {792.00, 1224.00},
	"ledger":    {1224.00, 792.00},
	"executive": {521.86, 756.00},
	"statement": {396.00, 612.00},
	"folio":     {612.00, 936.00},
	"quarto":    {610.00, 780.00},
}

func ParseCSSPageSettings(cssText string, defaults PageSettings, parseLength func(string) (float64, bool)) (PageSettings, PageSettings, error) {
	if strings.TrimSpace(cssText) == "" {
		return defaults, defaults, nil
	}

	sheet, err := parser.Parse(cssText)
	if err != nil {
		return PageSettings{}, PageSettings{}, fmt.Errorf("css parse: %w", err)
	}

	var defaultOverride pageSettingsOverride
	var firstOverride pageSettingsOverride

	for _, rule := range sheet.Rules {
		if rule == nil || rule.Kind != css.AtRule || !strings.EqualFold(strings.TrimSpace(rule.Name), "@page") {
			continue
		}

		override, err := parsePageRuleDeclarations(rule.Declarations, defaults, parseLength)
		if err != nil {
			return PageSettings{}, PageSettings{}, err
		}

		switch strings.ToLower(strings.TrimSpace(rule.Prelude)) {
		case "", "all":
			defaultOverride = defaultOverride.merge(override)
		case ":first":
			firstOverride = firstOverride.merge(override)
		}
	}

	defaultSettings := defaultOverride.apply(defaults)
	firstSettings := firstOverride.apply(defaultSettings)
	return defaultSettings, firstSettings, nil
}

func (o pageSettingsOverride) merge(next pageSettingsOverride) pageSettingsOverride {
	if next.sizeSet {
		o.sizeSet = true
		o.width = next.width
		o.height = next.height
	}
	if next.topSet {
		o.topSet = true
		o.top = next.top
	}
	if next.rightSet {
		o.rightSet = true
		o.right = next.right
	}
	if next.bottomSet {
		o.bottomSet = true
		o.bottom = next.bottom
	}
	if next.leftSet {
		o.leftSet = true
		o.left = next.left
	}
	return o
}

func (o pageSettingsOverride) apply(base PageSettings) PageSettings {
	if o.sizeSet {
		base.Width = o.width
		base.Height = o.height
	}
	if o.topSet {
		base.Margins.Top = o.top
	}
	if o.rightSet {
		base.Margins.Right = o.right
	}
	if o.bottomSet {
		base.Margins.Bottom = o.bottom
	}
	if o.leftSet {
		base.Margins.Left = o.left
	}
	return base
}

func parsePageRuleDeclarations(decls []*css.Declaration, defaults PageSettings, parseLength func(string) (float64, bool)) (pageSettingsOverride, error) {
	var override pageSettingsOverride
	for _, decl := range decls {
		if decl == nil {
			continue
		}
		property := strings.ToLower(strings.TrimSpace(decl.Property))
		value := strings.TrimSpace(decl.Value)
		switch property {
		case "size":
			width, height, err := parseCSSPageSize(value, defaults, parseLength)
			if err != nil {
				return override, err
			}
			override.sizeSet = true
			override.width = width
			override.height = height
		case "margin":
			top, right, bottom, left, err := parseCSSMarginShorthand(value, parseLength)
			if err != nil {
				return override, err
			}
			override.topSet = true
			override.rightSet = true
			override.bottomSet = true
			override.leftSet = true
			override.top = top
			override.right = right
			override.bottom = bottom
			override.left = left
		case "margin-top":
			v, err := parseCSSPageLength(value, property, parseLength)
			if err != nil {
				return override, err
			}
			override.topSet = true
			override.top = v
		case "margin-right":
			v, err := parseCSSPageLength(value, property, parseLength)
			if err != nil {
				return override, err
			}
			override.rightSet = true
			override.right = v
		case "margin-bottom":
			v, err := parseCSSPageLength(value, property, parseLength)
			if err != nil {
				return override, err
			}
			override.bottomSet = true
			override.bottom = v
		case "margin-left":
			v, err := parseCSSPageLength(value, property, parseLength)
			if err != nil {
				return override, err
			}
			override.leftSet = true
			override.left = v
		}
	}
	return override, nil
}

func parseCSSPageSize(raw string, defaults PageSettings, parseLength func(string) (float64, bool)) (float64, float64, error) {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(raw)))
	if len(parts) == 0 {
		return 0, 0, fmt.Errorf("@page size cannot be empty")
	}

	orientation := "portrait"
	width := 0.0
	height := 0.0
	lengths := make([]float64, 0, 2)

	for _, part := range parts {
		switch part {
		case "portrait", "landscape":
			orientation = part
		case "auto":
			width = defaults.Width
			height = defaults.Height
		default:
			if named, ok := namedPageSizes[part]; ok {
				width = named[0]
				height = named[1]
				continue
			}
			value, ok := parseLength(part)
			if !ok {
				return 0, 0, fmt.Errorf("unsupported @page size value %q", raw)
			}
			lengths = append(lengths, value)
		}
	}

	if width == 0 || height == 0 {
		if len(lengths) != 2 {
			return 0, 0, fmt.Errorf("@page size %q must be a known page name or two lengths", raw)
		}
		width = lengths[0]
		height = lengths[1]
	}

	if orientation == "landscape" {
		if height > width {
			width, height = height, width
		}
	} else if width > height && len(lengths) == 0 {
		width, height = height, width
	}

	return width, height, nil
}

func parseCSSMarginShorthand(raw string, parseLength func(string) (float64, bool)) (top, right, bottom, left float64, err error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 || len(parts) > 4 {
		return 0, 0, 0, 0, fmt.Errorf("invalid @page margin value %q", raw)
	}

	values := make([]float64, 0, len(parts))
	for _, part := range parts {
		value, err := parseCSSPageLength(part, "margin", parseLength)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		values = append(values, value)
	}

	switch len(values) {
	case 1:
		return values[0], values[0], values[0], values[0], nil
	case 2:
		return values[0], values[1], values[0], values[1], nil
	case 3:
		return values[0], values[1], values[2], values[1], nil
	default:
		return values[0], values[1], values[2], values[3], nil
	}
}

func parseCSSPageLength(raw, property string, parseLength func(string) (float64, bool)) (float64, error) {
	value, ok := parseLength(raw)
	if !ok {
		return 0, fmt.Errorf("invalid %s value %q", property, raw)
	}
	return value, nil
}

var (
	runningFooterRe = regexp.MustCompile(`(?is)footer\s*\{[^}]*position\s*:\s*running\(\s*([a-z0-9_-]+)\s*\)\s*;?[^}]*\}`)
	pageElementRe   = regexp.MustCompile(`(?is)@page(?:\s+[^\{]+)?\s*\{[\s\S]*?@bottom-center\s*\{[^}]*content\s*:\s*element\(\s*([a-z0-9_-]+)\s*\)\s*;?[^}]*\}`)
)

// ParseRunningFooterName returns the running footer name when both rules exist:
//   - footer { position: running(name); }
//   - @page { @bottom-center { content: element(name); } }
//
// and both names match.
func ParseRunningFooterName(cssText string) (string, bool) {
	if strings.TrimSpace(cssText) == "" {
		return "", false
	}
	runningMatch := runningFooterRe.FindStringSubmatch(strings.ToLower(cssText))
	if len(runningMatch) < 2 {
		return "", false
	}
	elementMatch := pageElementRe.FindStringSubmatch(strings.ToLower(cssText))
	if len(elementMatch) < 2 {
		return "", false
	}
	if runningMatch[1] != elementMatch[1] {
		return "", false
	}
	return runningMatch[1], true
}
