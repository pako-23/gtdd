package algorithms

import (
	"fmt"

	"github.com/pako-23/gtdd/internal/runner"
	log "github.com/sirupsen/logrus"
)

func edgeSelectPraDet(g DependencyGraph, edges []edge) (int, map[string]struct{}) {
	triedEdges := 1
	it := 0
	g.invertDependency(edges[it].from, edges[it].to)

	deps := g.getDependencies(edges[it].to)

	_, cycle := deps[edges[it].to]

	for cycle {
		g.invertDependency(edges[it].to, edges[it].from)
		if triedEdges == len(edges) {
			return -1, nil
		}

		it += 1
		triedEdges += 1

		g.invertDependency(edges[it].from, edges[it].to)
		deps = g.getDependencies(edges[it].to)
		_, cycle = deps[edges[it].to]
	}

	return it, deps
}

func PraDet(tests []string, oracle *runner.RunnerSet) (DependencyGraph, error) {
	g := NewDependencyGraph(tests)
	edges := []edge{}

	if len(g) <= 1 {
		return g, nil
	}

	for i := 1; i < len(tests); i++ {
		for j := i; j < len(tests); j++ {
			edges = append(edges, edge{from: tests[j], to: tests[j-i]})
			g.addDependency(tests[j], tests[j-i])
		}
	}
	log.Debug("starting dependency detection algorithm")

	it, deps := edgeSelectPraDet(g, edges)
	for len(edges) > 0 && it >= 0 {
		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, edges[it].to)
		results, err := oracle.RunSchedule(schedule)
		if err != nil {
			return nil, fmt.Errorf("pradet could not run schedule: %w", err)
		}
		log.Debugf("run tests %v -> %v", schedule, results.Results)

		g.removeDependency(edges[it].to, edges[it].from)

		for i, test := range schedule {
			if test == edges[it].from {
				if !results.Results[i] {
					g.addDependency(edges[it].from, edges[it].to)
				}
				edges = append(edges[:it], edges[it+1:]...)
				break
			} else if !results.Results[i] {
				g.addDependency(edges[it].from, edges[it].to)
				break
			}
		}

		if len(edges) == 0 {
			break
		}

		it, deps = edgeSelectPraDet(g, edges)
	}

	log.Debug("finished dependency detection algorithm")
	g.transitiveReduction()

	return g, nil
}
