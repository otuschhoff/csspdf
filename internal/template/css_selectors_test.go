package template

import (
	"fmt"
	"testing"
)

func TestSpecificitySimpleCompound(t *testing.T) {
	namespace := "*"
	selector := CompoundSelector{
		TypeName: &QualifiedName{Namespace: &namespace, Name: "div"},
		Subclasses: []SubclassSelector{
			IDSelector{Value: "main"},
			ClassSelector{Value: "hero"},
			AttributeSelector{
				Name:    QualifiedName{Name: "data-x"},
				Matcher: AttributeMatchPrefix,
				Value:   strPtr("a"),
			},
		},
		PseudoElem: &PseudoElementSelector{Name: "before"},
	}

	got := selector.Specificity()
	want := Specificity{A: 1, B: 2, C: 2}
	if got != want {
		t.Fatalf("unexpected specificity: got %+v, want %+v", got, want)
	}
}

func TestSpecificityComplexSelectorWithCombinators(t *testing.T) {
	selector := ComplexSelector{
		Head: CompoundSelector{
			TypeName: &QualifiedName{Name: "article"},
			Subclasses: []SubclassSelector{
				ClassSelector{Value: "post"},
			},
		},
		Tails: []SelectorTail{
			{
				Combinator: CombinatorChild,
				Right: CompoundSelector{
					Subclasses: []SubclassSelector{IDSelector{Value: "content"}},
				},
			},
			{
				Combinator: CombinatorDescendant,
				Right: CompoundSelector{
					TypeName: &QualifiedName{Name: "a"},
					Subclasses: []SubclassSelector{
						PseudoClassSelector{Name: "hover"},
					},
				},
			},
		},
	}

	got := selector.Specificity()
	want := Specificity{A: 1, B: 2, C: 2}
	if got != want {
		t.Fatalf("unexpected specificity: got %+v, want %+v", got, want)
	}
}

func TestPseudoClassSpecificitySelectors4(t *testing.T) {
	isSelector := PseudoClassSelector{
		Name: "is",
		Args: SelectorListPseudoArgs{Selectors: SelectorList{
			{Head: CompoundSelector{Subclasses: []SubclassSelector{ClassSelector{Value: "a"}}}},
			{Head: CompoundSelector{Subclasses: []SubclassSelector{IDSelector{Value: "x"}}}},
		}},
	}
	if got, want := isSelector.Specificity(), (Specificity{A: 1, B: 0, C: 0}); got != want {
		t.Fatalf(":is specificity mismatch: got %+v, want %+v", got, want)
	}

	notSelector := PseudoClassSelector{
		Name: "not",
		Args: &SelectorListPseudoArgs{Selectors: SelectorList{
			{Head: CompoundSelector{TypeName: &QualifiedName{Name: "section"}}},
			{Head: CompoundSelector{Subclasses: []SubclassSelector{ClassSelector{Value: "x"}}}},
		}},
	}
	if got, want := notSelector.Specificity(), (Specificity{A: 0, B: 1, C: 0}); got != want {
		t.Fatalf(":not specificity mismatch: got %+v, want %+v", got, want)
	}

	whereSelector := PseudoClassSelector{Name: "where", Args: SelectorListPseudoArgs{Selectors: SelectorList{
		{Head: CompoundSelector{Subclasses: []SubclassSelector{IDSelector{Value: "nope"}}}},
	}}}
	if got := whereSelector.Specificity(); got != (Specificity{}) {
		t.Fatalf(":where specificity must be zero, got %+v", got)
	}
}

func TestPseudoClassNthChildOfSpecificity(t *testing.T) {
	nth := PseudoClassSelector{
		Name: "nth-child",
		Args: NthPseudoArgs{
			A: 2,
			B: 1,
			Of: &SelectorList{
				{Head: CompoundSelector{Subclasses: []SubclassSelector{ClassSelector{Value: "item"}}}},
				{Head: CompoundSelector{Subclasses: []SubclassSelector{IDSelector{Value: "featured"}}}},
			},
		},
	}

	got := nth.Specificity()
	want := Specificity{A: 1, B: 1, C: 0}
	if got != want {
		t.Fatalf("unexpected specificity for nth-child(... of ...): got %+v, want %+v", got, want)
	}
}

func TestSpecificityCompare(t *testing.T) {
	cases := []struct {
		name  string
		left  Specificity
		right Specificity
		want  int
	}{
		{name: "A wins", left: Specificity{A: 1}, right: Specificity{A: 0, B: 999}, want: 1},
		{name: "B wins", left: Specificity{B: 2}, right: Specificity{B: 3}, want: -1},
		{name: "C tie-break", left: Specificity{A: 1, B: 2, C: 2}, right: Specificity{A: 1, B: 2, C: 1}, want: 1},
		{name: "equal", left: Specificity{A: 1, B: 1, C: 1}, right: Specificity{A: 1, B: 1, C: 1}, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.left.Compare(tc.right); got != tc.want {
				t.Fatalf("unexpected compare result: got %d, want %d", got, tc.want)
			}
		})
	}
}

func ExampleComplexSelector_Specificity() {
	selector := ComplexSelector{
		Head: CompoundSelector{
			TypeName: &QualifiedName{Name: "div"},
			Subclasses: []SubclassSelector{
				IDSelector{Value: "main"},
				ClassSelector{Value: "card"},
			},
		},
		Tails: []SelectorTail{
			{
				Combinator: CombinatorChild,
				Right: CompoundSelector{
					TypeName: &QualifiedName{Name: "a"},
					Subclasses: []SubclassSelector{
						PseudoClassSelector{Name: "hover"},
					},
				},
			},
		},
	}

	s := selector.Specificity()
	fmt.Printf("%d,%d,%d\n", s.A, s.B, s.C)
	// Output: 1,2,2
}

func ExamplePseudoClassSelector_Specificity() {
	pc := PseudoClassSelector{
		Name: "is",
		Args: SelectorListPseudoArgs{Selectors: SelectorList{
			{Head: CompoundSelector{Subclasses: []SubclassSelector{ClassSelector{Value: "btn"}}}},
			{Head: CompoundSelector{Subclasses: []SubclassSelector{IDSelector{Value: "submit"}}}},
		}},
	}

	s := pc.Specificity()
	fmt.Printf("%d,%d,%d\n", s.A, s.B, s.C)
	// Output: 1,0,0
}

func strPtr(value string) *string {
	return &value
}
