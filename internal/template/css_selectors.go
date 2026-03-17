package template

import "strings"

// SelectorList represents a comma-separated list of selectors.
type SelectorList []ComplexSelector

// MaxSpecificity returns the maximum specificity across all selectors in the list.
// This is useful for pseudo-classes like :is(), :not(), and :has().
func (list SelectorList) MaxSpecificity() Specificity {
	max := Specificity{}
	for _, selector := range list {
		max = maxSpecificity(max, selector.Specificity())
	}
	return max
}

// ComplexSelector is a selector with combinator-linked compounds, e.g. "div > .x + #id".
type ComplexSelector struct {
	Head  CompoundSelector
	Tails []SelectorTail
}

// Specificity calculates specificity for the complete complex selector.
func (selector ComplexSelector) Specificity() Specificity {
	s := selector.Head.Specificity()
	for _, tail := range selector.Tails {
		s = s.Add(tail.Right.Specificity())
	}
	return s
}

// SelectorTail links a combinator and right-hand compound selector.
type SelectorTail struct {
	Combinator Combinator
	Right      CompoundSelector
}

// Combinator defines CSS combinators between compounds.
type Combinator string

const (
	CombinatorDescendant    Combinator = " "
	CombinatorChild         Combinator = ">"
	CombinatorNextSibling   Combinator = "+"
	CombinatorSubsequent    Combinator = "~"
	CombinatorColumn        Combinator = "||"
	CombinatorShadowDeep    Combinator = ">>>" // non-standard/legacy
	CombinatorShadowSlotted Combinator = "/deep/"
)

// CompoundSelector is a simple sequence without combinators, e.g. "div.foo#id[attr]".
type CompoundSelector struct {
	TypeName   *QualifiedName
	Universal  bool
	Subclasses []SubclassSelector
	PseudoElem *PseudoElementSelector // if present, should be last in authoring order
}

// Specificity calculates specificity for the compound selector.
func (selector CompoundSelector) Specificity() Specificity {
	s := Specificity{}

	if selector.TypeName != nil {
		s.C++
	}

	for _, sub := range selector.Subclasses {
		s = s.Add(sub.Specificity())
	}

	if selector.PseudoElem != nil {
		s = s.Add(selector.PseudoElem.Specificity())
	}

	return s
}

// QualifiedName supports optional namespace prefixes.
// Namespace semantics:
//   - nil: no explicit namespace prefix (e.g. "div")
//   - "": empty namespace prefix (e.g. "|div")
//   - "*": any namespace (e.g. "*|div")
type QualifiedName struct {
	Namespace *string
	Name      string
}

// SubclassSelector represents selectors inside a compound after type/universal.
type SubclassSelector interface {
	isSubclassSelector()
	Specificity() Specificity
}

// IDSelector models "#id".
type IDSelector struct {
	Value string
}

func (IDSelector) isSubclassSelector() {}

func (IDSelector) Specificity() Specificity {
	return Specificity{A: 1}
}

// ClassSelector models ".class".
type ClassSelector struct {
	Value string
}

func (ClassSelector) isSubclassSelector() {}

func (ClassSelector) Specificity() Specificity {
	return Specificity{B: 1}
}

// AttributeSelector models selectors like [attr], [attr="x" i], [ns|attr^="foo"].
type AttributeSelector struct {
	Name     QualifiedName
	Matcher  AttributeMatcher
	Value    *string
	Modifier *AttributeModifier
}

func (AttributeSelector) isSubclassSelector() {}

func (AttributeSelector) Specificity() Specificity {
	return Specificity{B: 1}
}

// AttributeMatcher defines operators used in attribute selectors.
type AttributeMatcher string

const (
	AttributeMatchExists    AttributeMatcher = ""   // [attr]
	AttributeMatchExact     AttributeMatcher = "="  // [attr=value]
	AttributeMatchIncludes  AttributeMatcher = "~=" // [attr~=token]
	AttributeMatchDash      AttributeMatcher = "|=" // [attr|=en]
	AttributeMatchPrefix    AttributeMatcher = "^=" // [attr^=x]
	AttributeMatchSuffix    AttributeMatcher = "$=" // [attr$=x]
	AttributeMatchSubstring AttributeMatcher = "*=" // [attr*=x]
)

// AttributeModifier models case modifiers in attribute selectors.
type AttributeModifier string

const (
	AttributeModifierInsensitive AttributeModifier = "i"
	AttributeModifierSensitive   AttributeModifier = "s"
)

// PseudoClassSelector models pseudo-classes like :hover, :is(...), :nth-child(...).
// Unknown pseudo-classes can be represented via Name + Args.
type PseudoClassSelector struct {
	Name string
	Args PseudoArgs // nil for non-functional pseudo-classes
}

func (PseudoClassSelector) isSubclassSelector() {}

func (selector PseudoClassSelector) Specificity() Specificity {
	name := strings.ToLower(selector.Name)

	switch name {
	case "where":
		return Specificity{}
	case "is", "not", "has":
		if args, ok := selector.Args.(SelectorListPseudoArgs); ok {
			return args.Selectors.MaxSpecificity()
		}
		if args, ok := selector.Args.(*SelectorListPseudoArgs); ok && args != nil {
			return args.Selectors.MaxSpecificity()
		}
		return Specificity{B: 1}
	case "nth-child", "nth-last-child":
		base := Specificity{B: 1}
		if args, ok := selector.Args.(NthPseudoArgs); ok {
			if args.Of != nil {
				base = base.Add(args.Of.MaxSpecificity())
			}
			return base
		}
		if args, ok := selector.Args.(*NthPseudoArgs); ok && args != nil {
			if args.Of != nil {
				base = base.Add(args.Of.MaxSpecificity())
			}
			return base
		}
		return base
	default:
		return Specificity{B: 1}
	}
}

// PseudoElementSelector models pseudo-elements like ::before or ::part(...).
type PseudoElementSelector struct {
	Name string
	Args PseudoArgs // nil for non-functional pseudo-elements
}

// Specificity for pseudo-elements is equivalent to a type selector.
func (PseudoElementSelector) Specificity() Specificity {
	return Specificity{C: 1}
}

// PseudoArgs is an extensible argument model for functional pseudo selectors.
type PseudoArgs interface {
	isPseudoArgs()
}

// SelectorListPseudoArgs holds nested selector-list arguments, e.g. :is(...), :has(...), :not(...).
type SelectorListPseudoArgs struct {
	Selectors SelectorList
}

func (SelectorListPseudoArgs) isPseudoArgs() {}

// NthPseudoArgs models :nth-* formulas and optional "of <selector-list>" clause.
type NthPseudoArgs struct {
	A  int
	B  int
	Of *SelectorList
}

func (NthPseudoArgs) isPseudoArgs() {}

// IdentPseudoArgs stores identifier-style pseudo arguments, e.g. :lang(en).
type IdentPseudoArgs struct {
	Value string
}

func (IdentPseudoArgs) isPseudoArgs() {}

// StringPseudoArgs stores string-style pseudo arguments.
type StringPseudoArgs struct {
	Value string
}

func (StringPseudoArgs) isPseudoArgs() {}

// RawTokenPseudoArgs stores parser tokens for unknown/experimental syntax.
// Keeping raw tokens allows round-tripping and forward compatibility.
type RawTokenPseudoArgs struct {
	Tokens []string
}

func (RawTokenPseudoArgs) isPseudoArgs() {}

// Specificity is the CSS specificity tuple (A, B, C).
// A: ID selectors
// B: class/attribute/pseudo-class selectors
// C: type/pseudo-element selectors
type Specificity struct {
	A int
	B int
	C int
}

// Add adds two specificity tuples component-wise.
func (s Specificity) Add(other Specificity) Specificity {
	return Specificity{
		A: s.A + other.A,
		B: s.B + other.B,
		C: s.C + other.C,
	}
}

// Compare compares two specificity tuples.
// Returns 1 if s > other, -1 if s < other, 0 if equal.
func (s Specificity) Compare(other Specificity) int {
	if s.A != other.A {
		if s.A > other.A {
			return 1
		}
		return -1
	}
	if s.B != other.B {
		if s.B > other.B {
			return 1
		}
		return -1
	}
	if s.C != other.C {
		if s.C > other.C {
			return 1
		}
		return -1
	}
	return 0
}

func maxSpecificity(left, right Specificity) Specificity {
	if left.Compare(right) >= 0 {
		return left
	}
	return right
}
