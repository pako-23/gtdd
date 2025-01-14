package algorithms_test

import (
	"fmt"
	"testing"

	"github.com/pako-23/gtdd/internal/algorithms"
	"github.com/pako-23/gtdd/internal/runner"
	"gotest.tools/v3/assert"
)

func TestPraDetNoDependencies(t *testing.T) {
	testNoDependencies(t, algorithms.PraDet)
}

func TestPraDetExistingDependencies(t *testing.T) {
	testExistingDependencies(t, algorithms.PraDet)
}

func TestPraDetOrDependencies(t *testing.T) {
	testOrDependencies(t, algorithms.PraDet)
}

func TestPraDetErdosRenyiGenerated(t *testing.T) {
	testErdosRenyiGenerated(t, algorithms.PraDet)
}

func TestPraDetOrDependenciesMultipleLen(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		testsuite    []string
		dependencies map[string][][]string
		expected     []algorithms.DependencyGraph
	}{
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test2": {{"test1"}},
				"test3": {{"test1", "test2"}},
				"test5": {
					{"test1", "test2", "test3"},
					{"test4"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test4": {}},
				}),
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test3": {}},
				}),
			},
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5", "test6"},
			dependencies: map[string][][]string{
				"test2": {{"test1"}},
				"test3": {{"test1", "test2"}},
				"test5": {
					{"test1", "test2", "test3"},
					{"test4"},
				},
				"test6": {
					{"test1", "test2", "test3", "test5"},
					{"test4", "test5"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test4": {}},
					"test6": {"test5": {}},
				}),
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test3": {}},
					"test6": {"test5": {}},
				}),
			},
		},
	}

	for _, test := range tests {
		runner, _ := runner.NewRunnerSet[*mockRunner](5,
			newMockRunnerBuilder,
			withDependencyMap(test.dependencies))
		graph, err := algorithms.PraDet(test.testsuite, runner)

		assert.NilError(t, err)
		found := false

		for _, expectedDeps := range test.expected {
			if expectedDeps.Equal(graph) {
				found = true
				break
			}
		}

		assert.Check(t, found,
			fmt.Sprintf("expected graph %v, but got %v", graph, test.expected))
	}
}
