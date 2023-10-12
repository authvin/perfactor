package util

import (
	"fmt"
	"github.com/google/pprof/profile"
	"go/token"
	"os"
	"perfactor/graph"
	"sort"
	"time"
)

func GetProfileDataFromFile(filePath string) *profile.Profile {

	// read the profile data
	rawProfile, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening profiling data: " + err.Error())
		return nil
	}
	prof, err := profile.Parse(rawProfile)
	if err != nil {
		fmt.Println("Error parsing profiling data: " + err.Error())
		return nil
	}

	// find the for loops that are the most expensive
	return prof
}

func FilterLoopsUsingProfileData(safeLoops []Loop, sorted LoopInfoArray, threshold int64) LoopInfoArray {
	output := make(LoopInfoArray, 0)
	for _, lt := range sorted {
		loop, t := lt.Loop, lt.Time
		// check if the Loop is in the list of safe loops
		if !contains(safeLoops, loop.Pos) {
			continue
		}
		if t < threshold {
			// if the Time is less than the threshold, then the Loop is not worth making concurrent
			fmt.Printf("Loop at line %d has a total Time of %s, which is less than the threshold of %s\n", loop.Line, time.Duration(t), time.Duration(threshold))
			continue
		} else {
			fmt.Printf("Loop at line %d has a total Time of %s, which is greater than the threshold of %s\n", loop.Line, time.Duration(t), time.Duration(threshold))
		}
		//println("Loop at line ", fset.Position(loop.Pos()).Line, " has a total Time of ", time)
		output = append(output, lt)
	}
	return output
}

func contains(loops []Loop, pos token.Pos) bool {
	for _, loop := range loops {
		if loop.Pos == pos {
			return true
		}
	}
	return false
}

func SortLoopsUsingProfileData(prof *profile.Profile, forLoops []Loop, fset *token.FileSet) LoopInfoArray {
	totalCumulativeTime := make(LoopInfoArray, len(forLoops))

	// find the gr nodes corresponding to the for loops
	// look through the profile data and find the for loops that are the most expensive
	gr := graph.GetGraphFromProfile(prof)

	for i, loop := range forLoops {
		totalCumulativeTime[i].Loop = loop
		// get the line numbers of the Loop
		startLine := fset.Position(loop.Pos).Line
		endLine := fset.Position(loop.End).Line
		// find the node in the gr with this line number
		nodes := gr.FindNodesByLine(startLine, endLine)
		for _, node := range nodes {
			// we have the node - now we need to get the performance data for it
			//println("Node in Loop at line ", startLine, " has a total Time of ", node.Cum, " and a self Time of ", node.Flat)
			//println("The node has the name ", node.Info.Name)
			// add the cumulative Time to the total cumulative Time for this Loop
			totalCumulativeTime[i].Time += node.Cum
		}
	}
	sort.Sort(totalCumulativeTime)

	// now we have the total cumulative Time for each Loop, sorted from least to greatest. Let's return it
	return totalCumulativeTime
}

func GetLoopInfoArray(forLoops []Loop) LoopInfoArray {
	totalCumulativeTime := make(LoopInfoArray, len(forLoops))

	// find the gr nodes corresponding to the for loops
	// look through the profile data and find the for loops that are the most expensive

	for i, loop := range forLoops {
		totalCumulativeTime[i].Loop = loop
		totalCumulativeTime[i].Time = 1
	}

	// now we have the total cumulative Time for each Loop, sorted from least to greatest. Let's return it
	return totalCumulativeTime
}
