package graph

import (
	"fmt"
	"github.com/google/pprof/profile"
	"path/filepath"
	"sort"
)

// Most or all of the code in this file is copied from the pprof tool.
// The original code is available at https://github.com/google/pprof
// The reason it is copied and modified here is because the graph package is not exported by the pprof tool.
// There might be cleaner solutions, but this works. It should also be easily substituted if a better
// method is found, since the code is mostly copied directly.

func GetGraphFromProfile(prof *profile.Profile) *Graph {
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
			nodes[ln] = nm.FindOrInsertLine(l, lines[ln])
		}
		locations[l.ID] = nodes
	}
	nodes := nm.Nodes()
	// Make seen-maps
	seenNode := make(map[*Node]bool)
	seenEdge := make(map[NodePair]bool)

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
				if _, ok := seenEdge[NodePair{Src: n, Dest: parent}]; !ok &&
					parent != nil && n != parent {
					seenEdge[NodePair{Src: n, Dest: parent}] = true
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
	return SelectNodesForGraph(nodes, true)
}

type NodePair struct {
	Src, Dest *Node
}

type Nodes []*Node

func (nm NodeMap) Nodes() Nodes {
	nodes := make(Nodes, 0, len(nm))
	for _, n := range nm {
		nodes = append(nodes, n)
	}
	return nodes
}

type Graph struct {
	Nodes Nodes
}

func (g Graph) FindNodesByLine(line int, end int) []*Node {
	// returns all nodes in the graph with the given line number
	nodes := make([]*Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.Info.Lineno >= line && n.Info.Lineno <= end {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		println("No nodes found for line", line)
	}
	return nodes
}

func SelectNodesForGraph(nodes Nodes, dropNegative bool) *Graph {
	// Collect Nodes into a graph.
	gNodes := make(Nodes, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Cum == 0 && n.Flat == 0 {
			continue
		}
		if dropNegative && IsNegative(n) {
			continue
		}
		gNodes = append(gNodes, n)
	}
	sortNodes(gNodes)
	return &Graph{gNodes}
}

func IsNegative(n *Node) bool {
	switch {
	case n.Flat < 0:
		return true
	case n.Flat == 0 && n.Cum < 0:
		return true
	default:
		return false
	}
}

func (nm NodeMap) FindOrInsertLine(loc *profile.Location, line profile.Line) *Node {
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

type nodeSorter struct {
	rs   Nodes
	less func(l, r *Node) bool
}

func (s nodeSorter) Len() int           { return len(s.rs) }
func (s nodeSorter) Swap(i, j int)      { s.rs[i], s.rs[j] = s.rs[j], s.rs[i] }
func (s nodeSorter) Less(i, j int) bool { return s.less(s.rs[i], s.rs[j]) }

func sortNodes(ns Nodes) {
	var score map[*Node]int64
	scoreOrder := func(l, r *Node) bool {
		if iv, jv := abs64(score[l]), abs64(score[r]); iv != jv {
			return iv > jv
		}
		if iv, jv := l.Info.Name, r.Info.Name; iv != jv {
			return iv < jv
		}
		if iv, jv := abs64(l.Flat), abs64(r.Flat); iv != jv {
			return iv > jv
		}
		return compareNodes(l, r)
	}

	score = make(map[*Node]int64, len(ns))
	for _, n := range ns {
		score[n] = n.Cum
	}

	sort.Sort(nodeSorter{ns, scoreOrder})
}
func compareNodes(l, r *Node) bool {
	return fmt.Sprint(l.Info) < fmt.Sprint(r.Info)
}

func abs64(i int64) int64 {
	if i < 0 {
		return -i
	}
	return i
}
