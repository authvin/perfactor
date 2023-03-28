package util

import (
	"fmt"
	"github.com/google/pprof/profile"
	"go/ast"
	"go/token"
	"os"
	"perfactor/cmd"
	graph2 "perfactor/graph"
	"sort"
)

func GetDataFromProfile(forLoops []*ast.ForStmt, fset *token.FileSet) LoopTimeArray {
	// look through the profile data and find the for loops that are the most expensive
	if cmd.ProfileSource == "" {
		return nil
	}
	// read the profile data
	rawProfile, err := os.Open("1-cpu.pprof")
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	prof, err := profile.Parse(rawProfile)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	graph := graph2.GetGraphFromProfile(prof)
	// find the for loops that are the most expensive
	return sortLoopsUsingProfileData(graph, forLoops, fset)
}

func FilterLoopsUsingProfileData(safeLoops []token.Pos, sorted LoopTimeArray, fset *token.FileSet) []token.Pos {
	output := make([]token.Pos, 0)
	for _, lt := range sorted {
		loop, time := lt.loop, lt.time
		// check if the loop is in the list of safe loops
		if !contains(safeLoops, loop.Pos()) {
			continue
		}
		if time == 0 {
			// if the time is 0, then the loop is not worth making concurrent
			continue
		}
		println("Loop at line ", fset.Position(loop.Pos()).Line, " has a total time of ", time)
		output = append(output, loop.Pos())
	}
	return output
}

func contains(loops []token.Pos, pos token.Pos) bool {
	for _, loop := range loops {
		if loop == pos {
			return true
		}
	}
	return false
}

func sortLoopsUsingProfileData(graph *graph2.Graph, forLoops []*ast.ForStmt, fset *token.FileSet) LoopTimeArray {
	// find the graph nodes corresponding to the for loops
	totalCumulativeTime := make(LoopTimeArray, len(forLoops))
	for i, loop := range forLoops {
		totalCumulativeTime[i].loop = loop
		// get the line numbers of the loop
		startLine := fset.Position(loop.Pos()).Line
		endLine := fset.Position(loop.End()).Line
		// find the node in the graph with this line number
		nodes := graph.FindNodesByLine(startLine, endLine)
		for _, node := range nodes {
			// we have the node - now we need to get the performance data for it
			//println("Node in loop at line ", startLine, " has a total time of ", node.Cum, " and a self time of ", node.Flat)
			//println("The node has the name ", node.Info.Name)
			// add the cumulative time to the total cumulative time for this loop
			totalCumulativeTime[i].time += node.Cum
		}
	}
	sort.Sort(totalCumulativeTime)

	// now we have the total cumulative time for each loop, sorted from least to greatest. Let's return it
	return totalCumulativeTime
}
