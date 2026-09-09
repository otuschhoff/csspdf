package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

const (
	colorReset   = "\033[0m"
	colorDim     = "\033[2m"
	colorType    = "\033[36m" // cyan
	colorTag     = "\033[32m" // green
	colorAttr    = "\033[35m" // magenta
	colorAttrVal = "\033[33m" // yellow
	colorText    = "\033[37m" // white
	colorComment = "\033[90m" // bright black
)

func runDOMParse(program string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isHelpArgument(args[0]) {
		printDOMParseUsage(stdout, program)
		return 0
	}
	if len(args) != 1 {
		printDOMParseUsage(stderr, program)
		return 2
	}

	input := args[0]
	tmpl, err := template.ParseFiles(input)
	if err != nil {
		fmt.Fprintf(stderr, "Error parsing template %q: %v\n", input, err)
		return 1
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, nil); err != nil {
		fmt.Fprintf(stderr, "Error executing template %q: %v\n", input, err)
		return 1
	}

	doc, err := html.Parse(&rendered)
	if err != nil {
		fmt.Fprintf(stderr, "Error parsing rendered DOM %q: %v\n", input, err)
		return 1
	}

	fmt.Fprintf(stdout, "%sDOM%s %s(%s)%s\n", colorType, colorReset, colorDim, input, colorReset)
	printNode(stdout, doc, 0)
	return 0
}

func printDOMParseUsage(w io.Writer, program string) {
	fmt.Fprintf(w, "Usage: %s <input-html-file>\n", filepath.Base(program))
}

func printNode(writer io.Writer, n *html.Node, depth int) {
	if n == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	kind := nodeKind(n)

	switch n.Type {
	case html.DocumentNode:
		fmt.Fprintf(writer, "%s%s%s%s\n", indent, colorType, kind, colorReset)
	case html.DoctypeNode:
		fmt.Fprintf(writer, "%s%s%s%s %s%s%s\n", indent, colorType, kind, colorReset, colorTag, n.Data, colorReset)
	case html.ElementNode:
		fmt.Fprintf(writer, "%s%s%s%s %s<%s%s%s%s", indent, colorType, kind, colorReset, colorDim, colorTag, n.Data, colorReset, colorDim)
		for _, attr := range n.Attr {
			fmt.Fprintf(writer, " %s%s%s=%s\"%s\"%s", colorAttr, attr.Key, colorDim, colorAttrVal, attr.Val, colorDim)
		}
		fmt.Fprintf(writer, ">%s\n", colorReset)
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text == "" {
			break
		}
		fmt.Fprintf(writer, "%s%s%s%s \"%s%s%s\"\n", indent, colorType, kind, colorReset, colorText, text, colorReset)
	case html.CommentNode:
		fmt.Fprintf(writer, "%s%s%s%s %s<!-- %s -->%s\n", indent, colorType, kind, colorReset, colorComment, n.Data, colorReset)
	default:
		fmt.Fprintf(writer, "%s%s%s%s\n", indent, colorType, kind, colorReset)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		printNode(writer, child, depth+1)
	}
}

func nodeKind(n *html.Node) string {
	switch n.Type {
	case html.ErrorNode:
		return "ErrorNode"
	case html.TextNode:
		return "TextNode"
	case html.DocumentNode:
		return "DocumentNode"
	case html.ElementNode:
		return "ElementNode"
	case html.CommentNode:
		return "CommentNode"
	case html.DoctypeNode:
		return "DoctypeNode"
	default:
		return "UnknownNode"
	}
}
