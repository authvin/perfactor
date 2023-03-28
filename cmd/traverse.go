package cmd

import (
	"fmt"
	"github.com/google/pprof/profile"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"os"
	gr "perfactor/graph"
	"sort"
	"strconv"
	"sync"
)

// Define the Cobra command
var traverseCmd = &cobra.Command{
	Use:     "traverse",
	Aliases: []string{"trav"},
	Short:   "Traverse the graph",
	Run:     traverse,
}

// Stack from https://stackoverflow.com/a/28542256
type Stack struct {
	lock sync.Mutex
	s    []*gr.Node
}

func NewStack() *Stack {
	return &Stack{sync.Mutex{}, make([]*gr.Node, 0)}
}

func (s *Stack) Push(v *gr.Node) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.s = append(s.s, v)
}

func (s *Stack) Pop() *gr.Node {
	s.lock.Lock()
	defer s.lock.Unlock()

	l := len(s.s)
	if l == 0 {
		return nil
	}

	res := s.s[l-1]
	s.s = s.s[:l-1]
	return res
}

func (s *Stack) Peek() *gr.Node {
	s.lock.Lock()
	defer s.lock.Unlock()

	l := len(s.s)
	if l == 0 {
		return nil
	}

	return s.s[l-1]
}

func (s *Stack) HasNode() bool {
	return len(s.s) != 0
}

// Define the traverse function
func traverse(cmd *cobra.Command, args []string) {
	//// Create a prompt for the starting node
	//startingNodePrompt := promptui.Prompt{
	//	Label: "Starting node",
	//}
	//
	//// Get the starting node from the user
	//startingNode, err := startingNodePrompt.Run()
	//if err != nil {
	//	fmt.Println("Error getting starting node:", err)
	//	return
	//}
	//
	//// TODO: Find the node with the given value in the graph
	//
	//// Traverse the graph
	//currentNode := &gr.Node{value: startingNode}
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
	graph := gr.GetGraphFromProfile(prof)
	currentNode := graph.Nodes[0]
	parents := NewStack()
loop:
	for {
		// Display information about this node
		fmt.Println("Currently looking at: " + currentNode.Info.Name + " : " + strconv.FormatFloat(float64(currentNode.Cum)/1_000_000_000, 'f', -1, 64) + " s")
		// Move to next
		items := []string{
			"To Child",
		}
		if parents.HasNode() {
			items = append(items, "To Parent")
			items = append(items, "To Sibling")
		}
		items = append(items, "Quit")
		movePrompt := promptui.Select{
			Label: "Where do you want to move?",
			Items: items,
		}
		_, selectedMove, err := movePrompt.Run()
		if err != nil {
			fmt.Println("Error getting move:", err)
			return
		}
		switch selectedMove {
		case "Quit":
			break loop
		case "To Parent":
			// set the next select to be from current parent
			if parents.HasNode() {
				currentNode = parents.Pop()
			}
		case "To Child":
			currentNode = fetchChild(currentNode, parents)
			// Find the selected child node
			//for _, edge := range currentNode.Out {
			//	if edge.Dest.Info.Name == selectedChild {
			//		currentNode = edge.Dest
			//		break
			//	}
			//}
		case "To Sibling":
			par := parents.Peek()
			if par != nil {
				currentNode = fetchChild(par, parents)
			}
			// Find the selected child node
			//for _, edge := range currentNode.Out {
			//	if edge.Dest.Info.Name == selectedChild {
			//		currentNode = edge.Dest
			//		break
			//	}
			//}
		}
	}
}

func fetchChild(currentNode *gr.Node, parents *Stack) *gr.Node {
	edges, items := getChildrenValues(currentNode)

	// Create a prompt for the current node's children
	childPrompt := promptui.Select{
		Label: "Choose a node",
		Items: append(items, "Cancel"),
	}

	// Get the user's selection
	i, selectedChild, err := childPrompt.Run()
	if err != nil {
		fmt.Println("Error getting child:", err)
		return currentNode
	}
	if selectedChild == "Cancel" {
		return currentNode
	}
	if parents.Peek() != currentNode {
		parents.Push(currentNode)
	}
	return edges[i].Dest
}

// Helper function to get the values of a node's children
func getChildrenValues(node *gr.Node) (Edges, []string) {
	var values []string
	edges := make(Edges, 0)
	for _, Edge := range node.Out {
		edges = append(edges, Edge)
	}
	sort.Sort(edges)
	for _, Edge := range edges {
		str := "" + Edge.Dest.Info.Name + ": " + formatAsSeconds(Edge.Dest.Cum) + " s"
		values = append(values, str)
	}
	return edges, values
}

type Edges []*gr.Edge

func (e Edges) Len() int {
	return len(e)
}

func (e Edges) Less(i int, j int) bool {
	// Use > instead of < because we want the largest first
	return e[i].Dest.Cum > e[j].Dest.Cum
}

func (e Edges) Swap(i int, j int) {
	temp := e[j]
	e[j] = e[i]
	e[i] = temp
}

func init() {
	rootCmd.AddCommand(traverseCmd)
}

func formatAsSeconds(i int64) string {
	return strconv.FormatFloat(float64(i)/1_000_000_000, 'f', -1, 64)
}

// Sort the graph?
// Concept:
// We sort the graph results using a simple algorithm, and then traverse it depth-first looking for refactorable loops.
// When we find the first one, we refactor it and then re-run the program to see improvement
// - here, maybe try several refactoring options?
// We re-do the above until there is no noticeable improvement
