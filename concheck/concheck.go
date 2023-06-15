package concheck

import (
	"perfactor/concurrentcheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(concurrentcheck.Analyzer)
}
