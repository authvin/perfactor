package main

import (
	"fmt"
	"github.com/google/pprof/profile"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"perfactor/helper"
	"regexp"
)

// Overview:
// - Run the code as-is with detailed performance statistics
// - Process results, finding targets for refactoring
// - Find possible refactorings for these targets
// - Apply each refactoring sequentially in a temp folder
// - Build and run each variant with [unknown detail] performance statistics
// - Collect and analyse data from the runs
// - Apply or suggest a change (pull request, pop-up window, etc)
func main() {
	//RunCode("-mode 0", "Either", "original")
	//RunCode("-mode 1", "Either", "target")
	// we have .bench files, use them?
	//ProcessBenchData("1", "2", "3")
	findPossibleRefactorings("_streakfinder/main.go", nil)
}

const dataPath = "../_data/"

// RunCode Run external code in _tmp as a separate program - go test
func RunCode(flags string, benchname string, id string) {
	// Command should be a perf call with appropriate arguments
	//output, err := exec.Command("cmd", "/c", "dir").CombinedOutput()
	// go test %flags% -bench=%benchname% -run=NONE -benchmem -memprofile mem.pprof -cpuprofile cpu.pprof > %id%.bench
	// Potentially replace with: https://cs.opensource.google/go/go/+/refs/tags/go1.19.2:src/testing/benchmark.go;l=511
	output, err := exec.Command("powershell", "-nologo", "-noprofile", // opens powershell
		"cd", "../_tmp;", // move into the tmp folder
		"go", "test", flags, // we use go test, plus any flags that need to be passed to the executing method
		"-bench="+benchname, // the name of the benchmark method in the test file to run
		"-run=NONE",         // We don't run any normal tests. Maybe have this be a default value?
		"-memprofile "+dataPath+id+"-mem.pprof",    // record memory profile
		"-cpuprofile "+dataPath+id+"-cpu.pprof",    // record cpu profile
		"> "+dataPath+id+".bench").CombinedOutput() // put in "%id%.bench" for later use, in the _data directory
	if output != nil {
		fmt.Printf("%s", string(output))
	}
	if err != nil {
		fmt.Println(err)
	}
}

// ProcessBenchData Process the bench data
func ProcessBenchData(id ...string) {
	for i := 0; i < len(id); i++ {
		s := id[i]
		fmt.Println(s)
	}

	// Process - does this mean get speedup values?
	// 		- This allows to easily pick a faster method, but it's only useful for that
	//		- Is anything else needed?
	// 			- Would need a separate method for initial data - maybe that's fine?
	//	does this mean get a ranking of the benchmarks?
	//		- Even easier than just speedup, though with less info to give the user.
	//		- Should user-facing data be important? Gut says yes, probably
	//	does this mean 2d array with the interesting data for easier consumption?
	//		- Means we're over into proprietary data format
	//			- should be fine since we already have the original format
	//	does this mean using pprof?
	//		- Maybe? It depends what we're gonna use the data for
	// 		- Is it user-facing? Probably. Is it used internally? Solid maybe, but probably not.

	// In short - let's start by extracting speedup as a proof of concept.

	// Format: https://go.googlesource.com/proposal/+/master/design/14313-benchmark-format.md
}

// Find possible refactorings for a given target
func findPossibleRefactorings(path string, src any) {
	// keep all code versions in memory
	// use _tmp for writing and building the refactored code
	// any permanent data goes in _data
	fileset := token.NewFileSet()
	astFile, err := parser.ParseFile(fileset, path, src, parser.AllErrors)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	astFile.Pos()

	// PLAN FOR TODAY:
	// 1. Get the AST - We have the AST, need visitor
	// 2. Find out how to find the node that corresponds to a line - fset.Position(fn.Pos()).Line
	// 3. Find a way to extract the lines of code we want to look at from the profiling data - riiiight.
	fmt.Println("Start")
	// We need to use a visitor to do what we want to. The visitor can insert, so let's start by inserting a comment
	// For example, we can insert a comment where the largest problem is
	// Still need to find line numbers somewhere. Lexer?
	fmt.Println("Done")

	// Looks like we need to find our own line numbers. If so, we'll need a map of lines to position of the newlines
	// We first need to go through the source file and find every newline (but only newline characters, not newlines in strings!)
	// This we put into an array, where index = line number. index 0 can be whatever, not sure
	// Once we have this, we can do an easy lookup method where we give a position and it returns the index, which will be line number

	// LMAO nevermind we have fset.Position(fn.Pos()).Line

	// Data from profiling. It has a set structure, that part is fine. The bigger question is:
	// - how do I know which line in the profiler to look at?
	// We definitelt want to look at % cumulative, and only for lines of code in our source files
	// But how do we find out which of the lines of code mentioned are ours, and which are in other areas?
	// Also - can we use call graphs somehow? Like, a tree data structure? Because then we'd be able to try at different
	// points in the hierarchy. Imagine function A calls function B 5 times. But function B calls function C 50 times each time
	// Yes, we could parallelize A, but we might see more of an improvement by parallelizing B.
	//	And in some cases, maybe both is the right answer
	// TLDR: We really want a data structure here that gives us call hierarchy
	rawProfile, err := os.Open("1-cpu.pprof")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	prof, err := profile.Parse(rawProfile)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	//prof.FilterSamplesByName(regexp.MustCompile("perfactor"), regexp.MustCompile("testing"), nil, nil)
	r := prof.ShowFrom(regexp.MustCompile("runProgram"))
	if !r {
		fmt.Println("Regex found nothing")
	}

	// We want to filter to only the code we're looking for
	// How do we do this effectively? How do we make sure that the data we get is accumulative, but flat?

	i := 0
	for _, counter := range helper.GetLinesByTime(prof) {
		//if !strings.Contains(counter.File, "perfactor") {
		//	continue
		//}
		i++
		fmt.Printf("%v - %s : %s took %v\n", i, counter.File, counter.Func, counter.Time)
	}

	// This works, I guess? It's a data representation of the cpu profile
	// Might need to sort it manually. How does pprof find its numbers? Time to investigativite

	// The internals often use a directed graph. Would that work?
	// The internals use an open apache license, and I might just be able to import the internals too...
	//graph.New(prof, nil)
	// Fun fact: There's nothing stopping me from using internals except the *compiler* doesn't allow it...
	// Might need a change of plans for this. Technically I can just... copy the code, keep its license, and I'm good to go
	// I could even modify the code
	// But is that really necessary? I'll probably be able to build a graph or tree structure myself with the data
	// This might take time though, and it likely won't be as robust as the pprof stuff

	// So, to summarise:
	// - we have an AST, and a way to get line numbers from a node
	// - We have a profile parser, and we can make graphs from profiles
	// - The goal is to find hotspots in the graph,
	//		and find the corresponding AST node that should be looked at for refactoring
	// - TODO:
	// 		- Find out how to make options for the graph
	// 		- Find out how to interact with the graph to find hotspots
	//g := multi.NewDirectedGraph()
	// We have a graph!
	// All it really demands is that the nodes have an ID(). So the struct I am thinking of looks something ilke: (see struct below method)

	// First try. Not sure if this works right, because it doesn't add any edges. How do we know where to add edges?
	//for i := 1; i <= len(prof.Sample); i++ {
	//	sample := Node{
	//		UID:    int64(i),
	//		Sample: *prof.Sample[i-1],
	//	}
	//	//fmt.Println(prof.Sample[i-1].Location)
	//	g.AddNode(sample)
	//}
	// We definitely need to study how the profile data is added to the graph in pprof...
	// We might not need all of it, but it will at least be helpful to see how it connects it
	// (if it actually uses the graph for organising the data. It might just be used for printing?)

	// Okay, let's have a look at implementing the graph similarly to how pprof does it in graph.go::newGraph

	// Create nodes
	locations := make(map[uint64][]*Node, len(prof.Location))
	nm := make(NodeMap, len(prof.Location))
	for _, l := range prof.Location {
		lines := l.Line
		if len(lines) == 0 {
			lines = []profile.Line{{}}
		}
		nodes := make([]*Node, len(lines))
		for ln := range lines {
			nodes[ln] = nm.findOrInsertLine(l, lines[ln])
		}
		locations[l.ID] = nodes
	}
	nodes := nm.nodes()
	// Make seen-maps
	seenNode := make(map[*Node]bool)
	seenEdge := make(map[nodePair]bool)

	// Iterate through samples
	for _, sample := range prof.Sample {
		var w, dw int64
		w = sample.Value[1]
		dw = sample.Value[0]
		if dw == 0 && w == 0 {
			continue
		}
		for k := range seenNode {
			delete(seenNode, k)
		}
		for k := range seenEdge {
			delete(seenEdge, k)
		}
		var parent *Node
		residual := false

		for i := len(sample.Location) - 1; i >= 0; i-- {
			l := sample.Location[i]
			locNodes := locations[l.ID]
			for ni := len(locNodes) - 1; ni >= 0; ni-- {
				n := locNodes[ni]
				if n == nil {
					residual = true
					continue
				}
				if _, ok := seenNode[n]; !ok {
					seenNode[n] = true
					n.CumDiv += dw
					n.Cum += w
				}
				if _, ok := seenEdge[nodePair{n, parent}]; !ok &&
					parent != nil && n != parent {
					seenEdge[nodePair{n, parent}] = true
					if e := parent.Out[n]; e != nil {
						e.WeightDiv += dw
						e.Weight += w
						if residual {
							e.Residual = true
						}
						if ni == len(locNodes)-1 {
							e.Inline = false
						}
					}
					info := &Edge{Src: parent, Dest: n, WeightDiv: dw, Weight: w, Residual: residual, Inline: ni == len(locNodes)-1}
					parent.Out[n] = info
					n.In[parent] = info
				}
				parent = n
				residual = false
			}
		}
		if parent != nil && !residual {
			parent.Flat += w
			parent.FlatDiv += dw
		}
	}
	_ = selectNodesForGraph(nodes, true)
	fmt.Println()
}

type nodePair struct {
	src, dest *Node
}

type Nodes []*Node

func (nm NodeMap) nodes() Nodes {
	nodes := make(Nodes, 0, len(nm))
	for _, n := range nm {
		nodes = append(nodes, n)
	}
	return nodes
}

type Graph struct {
	Nodes Nodes
}

func selectNodesForGraph(nodes Nodes, dropNegative bool) *Graph {
	// Collect nodes into a graph.
	gNodes := make(Nodes, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Cum == 0 && n.Flat == 0 {
			continue
		}
		if dropNegative && isNegative(n) {
			continue
		}
		gNodes = append(gNodes, n)
	}
	return &Graph{gNodes}
}

func isNegative(n *Node) bool {
	switch {
	case n.Flat < 0:
		return true
	case n.Flat == 0 && n.Cum < 0:
		return true
	default:
		return false
	}
}

func (nm NodeMap) findOrInsertLine(loc *profile.Location, line profile.Line) *Node {
	var objfile string
	if m := loc.Mapping; m != nil && m.File != "" {
		objfile = m.File
	}
	if ni := nodeInfo(loc, line, objfile); ni != nil {
		return nm.FindOrInsertNode(*ni)
	}
	return nil
}

func (nm NodeMap) FindOrInsertNode(info NodeInfo) *Node {
	if n, ok := nm[info]; ok {
		return n
	}

	n := &Node{
		Info: info,
		In:   make(map[*Node]*Edge),
		Out:  make(map[*Node]*Edge),
	}
	nm[info] = n
	if info.Address == 0 && info.Lineno == 0 {
		// This node represents the whole function, so point Function
		// back to itself.
		n.Function = n
		return n
	}
	// Find a node that represents the whole function.
	info.Address = 0
	info.Lineno = 0
	n.Function = nm.FindOrInsertNode(info)
	return n
}

// Node is the proposed node for the graph. It has the ID that the graph needs, and it has the sample.
// If we make ID equal the index of the sample, we can also find it again in the profile if we really need to
type Node struct {
	UID int64
	// The following is matching the Node in the pprof graph.go
	Info                       NodeInfo
	Function                   *Node
	Flat, FlatDiv, Cum, CumDiv int64
	In, Out                    map[*Node]*Edge
	// LabelTags TagMap
	// NumericTags map[string]TagMap
}

type Edge struct {
	Src, Dest         *Node
	Weight, WeightDiv int64
	Residual          bool
	Inline            bool
}

type NodeInfo struct {
	Name, OrigName, File, Objfile string
	Address                       uint64
	StartLine, Lineno             int
}

func nodeInfo(l *profile.Location, line profile.Line, objfile string) *NodeInfo {
	if line.Function == nil {
		return &NodeInfo{Address: l.Address, Objfile: objfile}
	}
	ni := &NodeInfo{
		Address: l.Address,
		Lineno:  int(line.Line),
		Name:    line.Function.Name,
	}
	if fname := line.Function.Filename; fname != "" {
		ni.File = filepath.Clean(fname)
	}
	return ni
}

type NodeMap map[NodeInfo]*Node

func (t Node) ID() int64 {
	return t.UID
}

// Apply refactoring (temp folder, or actual code)
func applyRefactoring() {

}

// Suggest a change to be made
func suggestChange() {

}
