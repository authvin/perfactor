package helper

import (
	"fmt"
	"os"
	"regexp"
)

// Simple helper program for the logic of extracting the duration from profiling data
// This only gets the duration, not the more complex data for profiling, for finding where to apply refactoring
func main() {
	filename := "data/sub2/1-prof.txt"
	output, err := os.ReadFile(filename)
	if output == nil {
		fmt.Println("Could not read file " + filename)
	}
	if err != nil {
		fmt.Println("Error reading file: " + err.Error())
	}
	r, _ := regexp.Compile("Duration: [^,]+,")
	duration := r.FindAll(output, -1)
	fmt.Printf("%s\n", duration)
}

// Do I manually parse the format of the profiling?
// Do I see if I can get data from pprof, or maybe from the file itself?
// Do I write a parser? Maybe complex, but it sounds like it could be fun. And given how formulaic it is, it should be fairly doable
// Writing a parser wouldn't really have an advantage though, and it is not necessarily relevant to the task at hand.
// But maybe it is relevant for future people doing something similar? A parser might be cross-language, not limited to Go.
// Though then again, it would be designed to work with the pprof format, so would need to modify it anyway
