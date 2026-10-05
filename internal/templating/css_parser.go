package templating

import (
	"fmt"

	css "github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
	"github.com/gorilla/css/scanner"
)

func parseCSSStylesheet(text string) (*css.Stylesheet, error) {
	// Douceur does not advance past a semicolon in a qualified-rule prelude.
	// Validate tokens first so malformed selectors cannot trap its parser.
	if err := validateCSSRulePreludes(text); err != nil {
		return nil, err
	}
	return parser.Parse(text)
}

func validateCSSRulePreludes(text string) error {
	scan := scanner.New(text)
	var prelude cssRulePrelude
	for {
		token := scan.Next()
		switch token.Type {
		case scanner.TokenEOF:
			return nil
		case scanner.TokenError:
			return fmt.Errorf("tokenizer error: %s", token.String())
		case scanner.TokenBOM, scanner.TokenS, scanner.TokenComment, scanner.TokenCDO, scanner.TokenCDC:
			continue
		}
		if err := prelude.consume(token); err != nil {
			return err
		}
	}
}

type cssRulePrelude struct {
	rule           *css.Rule
	inDeclarations bool
}

func (p *cssRulePrelude) consume(token *scanner.Token) error {
	if p.inDeclarations {
		if token.Type == scanner.TokenChar && token.Value == "}" {
			p.inDeclarations = false
		}
		return nil
	}
	if token.Type == scanner.TokenChar {
		if handled, err := p.consumeDelimiter(token); handled {
			return err
		}
	}
	if p.rule == nil {
		p.rule = css.NewRule(css.QualifiedRule)
		if token.Type == scanner.TokenAtKeyword {
			p.rule.Kind = css.AtRule
			p.rule.Name = token.Value
		}
	}
	return nil
}

func (p *cssRulePrelude) consumeDelimiter(token *scanner.Token) (bool, error) {
	switch token.Value {
	case ";":
		if p.rule == nil || p.rule.Kind != css.AtRule {
			return true, fmt.Errorf("unexpected semicolon in CSS rule prelude: %s", token.String())
		}
		p.rule = nil
	case "{":
		if p.rule != nil {
			p.inDeclarations = !p.rule.EmbedsRules()
			p.rule = nil
		}
	case "}":
		// Inside a prelude, Douceur treats closing braces as ordinary tokens.
	default:
		return false, nil
	}
	return true, nil
}
