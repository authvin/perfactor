package main

func main() {
	// the point of this is to use a struct that has a suboptimal ordering of fields

}

// example taken from fieldalignment.go
// this struct uses 16 pointer bytes, but could use 8 if they were swapped
// this means when using a lot of these, the GC has to scan twice as many bytes - potential slowdown
type A struct {
	Value uint32
	Name  string
}
