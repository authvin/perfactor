package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"
	"perfactor/concurrentcheck"
)

func main() {
	singlechecker.Main(concurrentcheck.Analyzer)
}
