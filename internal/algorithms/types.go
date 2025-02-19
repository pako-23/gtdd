package algorithms

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
)

var ErrDependencyDetectorNotExisting = errors.New("the dependency detection strategy does not exist")

// edge represents a directed edge into the DependencyGraph.
type edge struct {
	from string
	to   string
}

// DependencyGraph represents the graph encoding the dependencies between the
// tests of a test suite. In the graph, each node is a test of the test suite
// and each edge represents the dependency relationship between the tests.
type DependencyGraph map[string]map[string]struct{}

type DependencyDetector func([]string, *runner.RunnerSet) (DependencyGraph, error)

// NewDependencyGraph returns a DependencyGraph without any edges from a
// list of tests.
func NewDependencyGraph(nodes []string) DependencyGraph {
	graph := DependencyGraph{}

	for _, node := range nodes {
		graph[node] = map[string]struct{}{}
	}

	return graph
}

// DependencyGraphFromJson returns a DependencyGraph form a JSON file. If
// there is any error it is returned.
func DependencyGraphFromJson(fileName string) (DependencyGraph, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	graph := map[string][]string{}
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to decode graph JSON data: %w", err)
	}

	tests := make([]string, 0, len(graph))
	for test := range graph {
		tests = append(tests, test)
	}

	g := NewDependencyGraph(tests)
	for test, dependencies := range graph {
		for _, dependency := range dependencies {
			g.AddDependency(test, dependency)
		}
	}

	return g, nil
}

// AddDependency adds a dependency relationship between two tests of a
// test suite.
func (d DependencyGraph) AddDependency(from, to string) {
	d[from][to] = struct{}{}
}

// RemoveDependency removes a dependency relationship between two tests of a
// test suite.
func (d DependencyGraph) RemoveDependency(from, to string) {
	delete(d[from], to)
}

// InvertDependency inverts a dependency relationship between two tests of a
// test suite.
func (d DependencyGraph) InvertDependency(from, to string) {
	d.RemoveDependency(from, to)
	d.AddDependency(to, from)
}

func (d DependencyGraph) Equal(other DependencyGraph) bool {
	if len(d) != len(other) {
		return false
	}

	for node, otherEdges := range other {
		edges, ok := d[node]
		if !ok || len(edges) != len(otherEdges) {
			return false
		}

		for to := range otherEdges {
			if _, ok := edges[to]; !ok {
				return false
			}
		}
	}

	return true

}

// GetDependencies returns all the dependencies of a given test.
func (d DependencyGraph) GetDependencies(test string) map[string]struct{} {
	var (
		dependencies = map[string]struct{}{}
		stack        = []string{}
		visited      = map[string]struct{}{}
	)

	stack = append(stack, test)

	for len(stack) != 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for u := range d[v] {
			if _, seen := visited[u]; !seen {
				dependencies[u] = struct{}{}
				stack = append(stack, u)
			} else if u == test {
				dependencies[u] = struct{}{}
			}
		}
		visited[v] = struct{}{}
	}

	return dependencies
}

// TransitiveReduction computes the transitive reduction of a dependency graph.
func (d DependencyGraph) TransitiveReduction() {
	for node, edges := range d {
		minEdges := make(map[string]struct{})

		for edge := range edges {
			minEdges[edge] = struct{}{}
		}

		for v := range edges {
			dependencies := d.GetDependencies(v)

			for u := range edges {
				_, isDependency := dependencies[u]
				_, isMinimalEdge := minEdges[u]

				if isDependency && isMinimalEdge {
					delete(minEdges, u)
				}
			}
		}

		d[node] = minEdges
	}
}

// ToJSON returns a JSON representation of the dependencies relationship
// between tests of a test suite.
func (d DependencyGraph) ToJSON(w io.Writer) {
	d.TransitiveReduction()
	graph := map[string][]string{}

	for test, dependencies := range d {
		tests := []string{}
		for dependency := range dependencies {
			tests = append(tests, dependency)
		}

		graph[test] = tests
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		log.Errorf("failed to create json from data: %v", err)
	}

	w.Write(data)
}

// ToDOT returns a DOT representation of the dependencies relationship
// between tests of a test suite.
func (d DependencyGraph) ToDOT(w io.Writer) {
	w.Write([]byte("digraph {\n"))
	w.Write([]byte("    compound = \"true\"\n"))
	w.Write([]byte("    newrank = \"true\"\n"))
	w.Write([]byte("    subgraph \"root\" {\n"))

	tests := make([]string, 0, len(d))
	for test := range d {
		tests = append(tests, test)
	}
	sort.Strings(tests)

	for _, test := range tests {
		fmt.Fprintf(w, "        \"%s\"\n", test)
	}

	for _, test := range tests {
		dependencies := []string{}

		for dependency := range d[test] {
			dependencies = append(dependencies, dependency)
		}
		sort.Strings(dependencies)

		for _, dependency := range dependencies {
			fmt.Fprintf(w, "        \"%s\" -> \"%s\"\n", test, dependency)
		}
	}

	w.Write([]byte("    }\n"))
	w.Write([]byte("}\n"))
}

// GetSchedules returns the schedules needed to cover all the provided tests
// based on the dependencies into the dependency graph.
func (d DependencyGraph) GetSchedules(tests []string) [][]string {
	var (
		schedules = [][]string{}
		visited   = map[string]struct{}{}
	)

	for i := len(tests) - 1; i >= 0; i-- {
		if _, ok := visited[tests[i]]; ok {
			continue
		}

		deps := d.GetDependencies(tests[i])
		schedule := []string{}

		for _, item := range tests {
			if _, ok := deps[item]; ok {
				visited[item] = struct{}{}
				schedule = append(schedule, item)
			}
		}
		schedule = append(schedule, tests[i])
		schedules = append(schedules, schedule)
	}

	return schedules
}
