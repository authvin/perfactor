package imgproc

import (
	"testing"
)

var src = loadImage("testdata/Beach_3.jpg")

func BenchmarkRunProgram(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(src)
	}
}
