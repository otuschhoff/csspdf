package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(runCLI(os.Args[0], os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(program string, args []string, stdout, stderr io.Writer) int {
	program = filepath.Base(program)
	if len(args) == 0 {
		printCLIUsage(stderr, program)
		return 2
	}

	switch args[0] {
	case "dom-parse":
		return runDOMParse(program+" dom-parse", args[1:], stdout, stderr)
	case "gen-example":
		return runGenExample(program+" gen-example", args[1:], stdout, stderr)
	case "pdfdump":
		return runPDFDump(program+" pdfdump", args[1:], stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintf(stdout, "%s version %s\n", program, version)
		return 0
	case "help", "-h", "--help":
		printCLIUsage(stdout, program)
		return 0
	default:
		fmt.Fprintf(stderr, "Error: unknown command %q\n\n", args[0])
		printCLIUsage(stderr, program)
		return 2
	}
}

func printCLIUsage(w io.Writer, program string) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s <command> [options]\n", program)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  dom-parse    Render an HTML template and print its DOM")
	fmt.Fprintln(w, "  gen-example  Generate or verify example documents")
	fmt.Fprintln(w, "  pdfdump      Display PDF object streams")
	fmt.Fprintln(w, "  version      Print version and exit")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Use '%s <command> -h' for command-specific options.\n", program)
}

func isHelpArgument(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}
