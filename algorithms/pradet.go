package algorithms

import "github.com/pako-23/gtdd/runners"

func Pradet(tests []string, oracle *runners.RunnerSet) (DependencyGraph, error) {
	g := NewDependencyGraph(tests)
	// edges := make([]edge, len(tests))
	// TODO: implement pradet
	// Question to ask: concurrent or not? concurrent is a bit different from
	// the original.
	return g, nil
}
