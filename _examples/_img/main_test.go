package imgproc

import (
	"testing"
)

var src = loadImage("testdata/test_beach.png")

func BenchmarkRunProgram(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(src)
	}
}
