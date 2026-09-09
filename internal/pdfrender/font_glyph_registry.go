package pdfrender

import "sort"

// FontGlyphRegistry tracks used glyphs per font for later subsetting.
type FontGlyphRegistry struct {
	used map[string]map[rune]struct{}
}

func NewFontGlyphRegistry() *FontGlyphRegistry {
	return &FontGlyphRegistry{used: make(map[string]map[rune]struct{})}
}

func (g *FontGlyphRegistry) Record(fontFace, text string) {
	if g == nil || fontFace == "" || text == "" {
		return
	}
	if _, ok := g.used[fontFace]; !ok {
		g.used[fontFace] = make(map[rune]struct{})
	}
	for _, character := range text {
		g.used[fontFace][character] = struct{}{}
	}
}

func (g *FontGlyphRegistry) UsedGlyphs(fontFace string) []rune {
	if g == nil || g.used[fontFace] == nil {
		return nil
	}
	result := make([]rune, 0, len(g.used[fontFace]))
	for character := range g.used[fontFace] {
		result = append(result, character)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

func (g *FontGlyphRegistry) Snapshot() map[string][]rune {
	result := make(map[string][]rune)
	if g == nil {
		return result
	}
	for font := range g.used {
		result[font] = g.UsedGlyphs(font)
	}
	return result
}
