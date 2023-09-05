package algorithms

type edge struct {
	from string
	to   string
}

type DependencyGraph map[string]map[string]struct{}

func NewDependencyGraph(nodes []string) DependencyGraph {
	graph := DependencyGraph{}

	for _, node := range nodes {
		graph[node] = map[string]struct{}{}
	}

	return graph
}

func (d DependencyGraph) AddDependency(from, to string) {
	d[from][to] = struct{}{}
}
