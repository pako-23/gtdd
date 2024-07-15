package algorithms

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewDependencyGraph(t *testing.T) {
	t.Parallel()

	var tests = [][]string{
		{"node1", "node2", "node3"},
		{"node1"},
		{},
	}

	for _, test := range tests {
		got := NewDependencyGraph(test)
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
		first    DependencyGraph
		second   DependencyGraph
		expected bool
	}{
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: true,
		},
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
				"node4": {},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node4": {},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
			}),
			expected: false,
		},
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node3": {}},
				"node3": {},
			}),
			expected: false,
		},
		{
			first: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}, "node2": {}},
			}),
			second: DependencyGraph(map[string]map[string]struct{}{
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
		expected DependencyGraph
	}{
		{
			nodes: []string{"node1", "node2", "node3"},
			edges: []edge{},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
		{
			nodes: []string{"node1", "node2", "node3"},
			edges: []edge{{"node2", "node1"}, {"node3", "node1"}, {"node3", "node2"}},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
	}

	for _, test := range tests {
		got := NewDependencyGraph(test.nodes)

		for _, edge := range test.edges {
			got.addDependency(edge.from, edge.to)
		}

		assert.Check(t, got.Equal(test.expected))
	}
}

func TestRemoveDependency(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    DependencyGraph
		edges    []edge
		expected DependencyGraph
	}{
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}, {"node3", "node2"}, {"node3", "node1"}},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
	}

	for _, test := range tests {
		for _, edge := range test.edges {
			test.graph.removeDependency(edge.from, edge.to)
		}

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestInvertDependency(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    DependencyGraph
		edges    []edge
		expected DependencyGraph
	}{
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {"node2": {}},
				"node2": {},
				"node3": {"node2": {}, "node1": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			edges: []edge{{"node2", "node1"}, {"node3", "node2"}, {"node3", "node1"}},
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {"node2": {}, "node3": {}},
				"node2": {"node3": {}},
				"node3": {},
			}),
		},
	}

	for _, test := range tests {
		for _, edge := range test.edges {
			test.graph.invertDependency(edge.from, edge.to)
		}

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestGetDependencies(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    DependencyGraph
		expected map[string]map[string]struct{}
	}{
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
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
			graph: DependencyGraph(map[string]map[string]struct{}{
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
			dependencies := test.graph.getDependencies(node)

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
		graph    DependencyGraph
		expected DependencyGraph
	}{
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}, "node1": {}},
			}),
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node1": {}, "node2": {}},
			}),
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {"node1": {}, "node2": {}},
			}),
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}},
				"node4": {"node1": {}, "node3": {}},
				"node5": {"node1": {}, "node2": {}},
			}),
			expected: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node1": {}},
				"node4": {"node3": {}},
				"node5": {"node2": {}},
			}),
		},
	}

	for _, test := range tests {
		test.graph.transitiveReduction()

		assert.Check(t, test.graph.Equal(test.expected))
	}
}

func TestGetSchedules(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		graph    DependencyGraph
		tests    []string
		expected [][]string
	}{
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {},
				"node3": {},
			}),
			tests:    []string{"node1", "node2", "node3"},
			expected: [][]string{{"node1"}, {"node2"}, {"node3"}},
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
				"node1": {},
				"node2": {"node1": {}},
				"node3": {"node2": {}},
			}),
			tests:    []string{"node1", "node2", "node3"},
			expected: [][]string{{"node1", "node2", "node3"}},
		},
		{
			graph: DependencyGraph(map[string]map[string]struct{}{
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
