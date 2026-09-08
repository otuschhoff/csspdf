package main

import (
	"github.com/otuschhoff/csspdf/internal/errorwrapcheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(errorwrapcheck.Analyzer)
}
