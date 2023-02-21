# PPROF
This document is an attempt at diving into and explaining the pprof format in an understandable way. Specifically, rather than dig solely into the binary file, this is a look at the Profile data structure that the pprof tool parses this data into. The mentioned tool is a tool for visualisation of go profile data, though it can also take more generic perf data. It is intended for manual use, and the tool produces reports in various different formats.

For our purposes, we want to be able to analyse the profile data directly, rather than look at the reports designed for humans. Unfortunately, large parts of pprof are internal, and not everything we might need is exposed. Thus, this will focus on the pprof profile format with the perspective of generating something similar to pprof's reports, but aimed at being machine-readable and usable for analysing profiling results programmatically.

This is a simplified version of said profile:

```go
type Profile struct {
    [...]
    Sample            []*Sample
    Mapping           []*Mapping
    Location          []*Location
    Function          []*Function
    [...]
}
```

There are four major parts to the struct we are immediately interested in; Sample, Mapping, Location and Function, though mainly we want Sample.

A Sample looks like this:

```go
type Sample struct {
	Location []*Location
	Value    []int64
	Label    map[string][]string
	NumLabel map[string][]int64
	NumUnit  map[string][]string

	locationIDX []uint64
	labelX      []label
}
```
The location says which part of the code was executing at the time the sample was taken, the value contains the time spent, and the NumUnit contains the unit of time used. Value is an array containing two values; the amount of time actually recorded, and the extrapolated, estimated time. 

Location contains the following information:

```go
type Location struct {
	ID       uint64
	Mapping  *Mapping
	Address  uint64
	Line     []Line
	IsFolded bool

	mappingIDX uint64
}
```

In other words, a profile contains a series of samples, and each sample contains how long it spent there, as well as the stack trace during the sample. This raw data needs rearranging in order to see how everything is connected, and see how it hangs together. One way is to place it into a graph, connecting nodes that call other nodes. What would the edge weights be?


The edge weights in the pprof graph add their values as weights. It also makes nodes of each location. Should we just replicate this? It seems like the easiest way to go about it, since we can't use their internals. 

The method newGraph in internal/graph/graph.go is where the profile is converted into a graph structure. It does this by looping over the samples, then for each sample filling out a map of seen nodes and a map of seen edges as it goes through the sample.

It then loops through all the locations in the sample in order, and inside this it loops through all the nodes. It gets this from CreateNodes, which creates nodes corresponding to all locations present in a profile. It returns the set of all nodes, plus a mapping from location ID to the set of nodes for that location.

The loop through all nodes in a location is where the logic happens. First it checks if the node is nil. If so, we have a residual. This variable is used when adding later edges, and signifies a node that "connect[s] nodes that were connected through a separate node, which has been removed from the report" - from documentation

So assuming the node is not nil, it then checks if the node has been seen. If not, it marks it as seen and adds the sample to the node. Then it checks if the current edge is seen (that being, parent to edge), and if not then it adds the edge to the parent.

This relatively simple structure sets up the whole graph. It might later be modified, but this is the core structure that can then be used in analysis.

_More time should be spent looking at all the arguments given with the addSample and AddEdgeToDiv method calls, as they might be important and I do not yet feel I have a full grasp on them._


Each sample contains a list of locations present in this sample. This list is the hierarchy of method calls in the stack, ending with the currently executing code. When processing a sample, we add the value of the sample to the flat value of a node only once. For all other nodes, representing the rest of the locations, we instead add to the cumulative time. We also add to the cumulative time of the currently executing node.

This means that in the finished graph, each node has a flat time of only when it has been directly running, and a cumulative time of when it has been awaiting a returning sub-function. For some methods, they will have equal flat and cumulative because they have little to no sub-functions. For other methods, they simply start and coordinate sub-processes, and spends all its time waiting for them to finish running.


