package csspdf

import (
	htmltmpl "html/template"
	"strings"
)

type currencyMeta struct {
	Code   string
	Symbol string
	Name   string
}

var currencyMetaByCode = map[string]currencyMeta{
	"AUD": {Code: "AUD", Symbol: "$", Name: "Australian Dollar"},
	"CAD": {Code: "CAD", Symbol: "$", Name: "Canadian Dollar"},
	"CHF": {Code: "CHF", Symbol: "CHF", Name: "Swiss Franc"},
	"CNY": {Code: "CNY", Symbol: "\u00a5", Name: "Chinese Yuan"},
	"EUR": {Code: "EUR", Symbol: "\u20ac", Name: "Euro"},
	"GBP": {Code: "GBP", Symbol: "\u00a3", Name: "British Pound"},
	"JPY": {Code: "JPY", Symbol: "\u00a5", Name: "Japanese Yen"},
	"NOK": {Code: "NOK", Symbol: "kr", Name: "Norwegian Krone"},
	"SEK": {Code: "SEK", Symbol: "kr", Name: "Swedish Krona"},
	"USD": {Code: "USD", Symbol: "$", Name: "US Dollar"},
}

func currencyTemplateFuncs(currency currencyMeta) htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		"currency": func(attribute ...string) string {
			if len(attribute) == 0 {
				return currency.Code
			}
			switch strings.ToLower(strings.TrimSpace(attribute[0])) {
			case "", "code", "iso", "iso4217", "currency", "id":
				return currency.Code
			case "symbol", "sign", "glyph":
				return currency.Symbol
			case "name", "label", "display", "pretty":
				return currency.Name
			default:
				return currency.Code
			}
		},
		"currencyCode":   func() string { return currency.Code },
		"currencySymbol": func() string { return currency.Symbol },
		"currencyName":   func() string { return currency.Name },
	}
}

func resolveCurrencyMeta(code string) currencyMeta {
	resolvedCode := strings.ToUpper(strings.TrimSpace(code))
	if resolvedCode == "" {
		resolvedCode = DefaultCurrencyCode
	}
	if meta, ok := currencyMetaByCode[resolvedCode]; ok {
		return meta
	}
	return currencyMeta{Code: resolvedCode, Symbol: resolvedCode, Name: resolvedCode}
}
