package algorithms_test

import (
	"fmt"
	"testing"

	"github.com/pako-23/gtdd/internal/algorithms"
	"gotest.tools/v3/assert"
)

type edge struct {
	from, to string
}

func TestNewDependencyGraph(t *testing.T) {
	t.Parallel()

	var tests = [][]string{
		{"node1", "node2", "node3"},
		{"node1"},
		{},
	}

	for _, test := range tests {
		got := algorithms.NewDependencyGraph(test)
		assert.Equal(t, len(got), len(test))

		for _, node := range test {
			edges, ok := got[node]

			assert.Check(t, ok)
			assert.Equal(t, len(edges), 0)
		}
	}
}

func TestEqualDependencyGraph(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		first    algorithms.DependencyGraph
		second   algorithms.DependencyGraph
		expected bool
	}{
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: true,
		},
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
				"node4": {},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node4": {},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
			}),
			expected: false,
		},
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node3": {}},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}, "node2": {}},
			}),
			second: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}, "node2": {}},
			}),
			expected: true,
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.first.Equal(test.second), test.expected)
	}

}

func TestAddDependency(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		nodes    []string
		edges    []edge
		expected algorithms.DependencyGraph
	}{
		{
			nodes: []string{"node1", "node2", "node3"},
			edges: []edge{},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
		{
			nodes: []string{"node1", "node2", "node3"},
			edges: []edge{{"node2", "node1"}, {"node3", "node1"}, {"node3", "node2"}},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
	}

	for _, test := range tests {
		got := algorithms.NewDependencyGraph(test.nodes)

		for _, edge := range test.edges {
			got.AddDependency(edge.from, edge.to)
		}

		assert.Check(t, got.Equal(test.expected))
	}
}

func TestRemoveDependency(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    algorithms.DependencyGraph
		edges    []edge
		expected algorithms.DependencyGraph
	}{
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}, {"node3", "node2"}, {"node3", "node1"}},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
	}

	for _, test := range tests {
		for _, edge := range test.edges {
			test.graph.RemoveDependency(edge.from, edge.to)
		}

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestInvertDependency(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    algorithms.DependencyGraph
		edges    []edge
		expected algorithms.DependencyGraph
	}{
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {"node2": {}},
				"node2": {},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}, {"node3", "node2"}, {"node3", "node1"}},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {"node2": {}, "node3": {}},
				"node2": {"node3": {}},
				"node3": {},
			}),
		},
	}

	for _, test := range tests {
		for _, edge := range test.edges {
			test.graph.InvertDependency(edge.from, edge.to)
		}

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestGetDependencies(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    algorithms.DependencyGraph
		expected map[string]map[string]struct{}
	}{
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			expected: map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			},
		},

		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {"node3": {}},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			expected: map[string]map[string]struct{}{
				"node1": {"node2": {}, "node1": {}, "node3": {}},
				"node2": {"node2": {}, "node1": {}, "node3": {}},
				"node3": {"node2": {}, "node1": {}, "node3": {}},
			},
		},
	}

	for _, test := range tests {
		for node, expected := range test.expected {
			dependencies := test.graph.GetDependencies(node)

			assert.Equal(t, len(dependencies), len(expected))

			for dependency := range expected {
				_, ok := dependencies[dependency]

				assert.Check(t, ok)
			}
		}
	}
}

func TestTransitiveReduction(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    algorithms.DependencyGraph
		expected algorithms.DependencyGraph
	}{
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node1": {}, "node2": {}},
			}),
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node1": {}, "node2": {}},
			}),
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}},
				"node4": {"node1": {}, "node3": {}},
				"node5": {"node1": {}, "node2": {}},
			}),
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}},
				"node4": {"node3": {}},
				"node5": {"node2": {}},
			}),
		},
	}

	for _, test := range tests {
		test.graph.TransitiveReduction()

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestGetSchedules(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    algorithms.DependencyGraph
		tests    []string
		expected [][]string
	}{
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			tests:    []string{"node1", "node2", "node3"},
			expected: [][]string{{"node1"}, {"node2"}, {"node3"}},
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			tests:    []string{"node1", "node2", "node3"},
			expected: [][]string{{"node1", "node2", "node3"}},
		},
		{
			graph: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}},
				"node4": {"node1": {}, "node3": {}},
				"node5": {"node1": {}, "node2": {}},
			}),
			tests: []string{"node1", "node2", "node3", "node4", "node5"},
			expected: [][]string{
				{"node1", "node2", "node5"},
				{"node1", "node3", "node4"},
			},
		},
	}

	for _, test := range tests {
		schedules := test.graph.GetSchedules(test.tests)

		assert.Equal(t, len(schedules), len(test.expected))

		for _, schedule := range test.expected {
			scheduleFound := false

			for _, computedSchedule := range schedules {
				if len(computedSchedule) != len(schedule) {
					continue
				}

				same := true

				for i := range schedule {
					if schedule[i] != computedSchedule[i] {
						same = false
						break
					}
				}

				if same {
					scheduleFound = true
					break
				}
			}

			assert.Check(t, scheduleFound, fmt.Sprintf("The schedule %v is missing", schedule))
		}

	}
}
