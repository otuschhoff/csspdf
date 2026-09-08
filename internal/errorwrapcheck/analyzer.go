package errorwrapcheck

import (
	"go/ast"
	"go/constant"
	"go/types"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errorwrapcheck",
	Doc:  "reports fmt.Errorf calls that format an error without wrapping it",
	Run:  run,
}

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isFmtErrorf(pass, call) || len(call.Args) < 2 {
				return true
			}
			format, known := staticString(pass, call.Args[0])
			if !known || hasWrappingVerb(format) {
				return true
			}
			for _, argument := range call.Args[1:] {
				argumentType := pass.TypesInfo.TypeOf(argument)
				if argumentType != nil && types.Implements(argumentType, errorType) {
					pass.Reportf(call.Pos(), "fmt.Errorf formats an error without %%w")
					break
				}
			}
			return true
		})
	}
	return nil, nil
}

func isFmtErrorf(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	function, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	return ok && function.Pkg() != nil && function.Pkg().Path() == "fmt" && function.Name() == "Errorf"
}

func staticString(pass *analysis.Pass, expression ast.Expr) (string, bool) {
	if value := pass.TypesInfo.Types[expression].Value; value != nil && value.Kind() == constant.String {
		return constant.StringVal(value), true
	}
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok {
		return "", false
	}
	left, leftKnown := staticString(pass, binary.X)
	right, rightKnown := staticString(pass, binary.Y)
	if !leftKnown && !rightKnown {
		return "", false
	}
	return left + right, true
}

func hasWrappingVerb(format string) bool {
	for index := 0; index < len(format); index++ {
		if format[index] != '%' {
			continue
		}
		index++
		if index < len(format) && format[index] == '%' {
			continue
		}
		for ; index < len(format); index++ {
			if unicode.IsLetter(rune(format[index])) {
				if format[index] == 'w' {
					return true
				}
				break
			}
		}
	}
	return false
}
