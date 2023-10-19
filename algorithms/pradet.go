package algorithms

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/pako-23/gtdd/runners"
)

type PraDet struct{}

func edgeSelectPraDet(g DependencyGraph, edges []edge, begin int) (int, map[string]struct{}) {
	triedEdges := 1
	g.InvertDependency(edges[begin].from, edges[begin].to)
	deps := g.GetDependencies(edges[begin].to)

	_, cycle := deps[edges[begin].to]

	for cycle {
		g.InvertDependency(edges[begin].to, edges[begin].from)
		if triedEdges == len(edges) {
			return -1, nil
		}

		begin += 1
		if begin == len(edges) {
			begin = 0
		}
		triedEdges += 1

		g.InvertDependency(edges[begin].from, edges[begin].to)
		deps = g.GetDependencies(edges[begin].to)
		_, cycle = deps[edges[begin].to]
	}

	return begin, deps
}

func (_ *PraDet) FindDependencies(tests []string, oracle *runners.RunnerSet) (DependencyGraph, error) {
	g := NewDependencyGraph(tests)
	edges := []edge{}

	for i := 1; i < len(tests); i++ {
		for j := 0; j < i; j++ {
			edges = append(edges, edge{from: tests[i], to: tests[j]})
			g.AddDependency(tests[i], tests[j])
		}
	}

	log.Debug("starting dependency detection algorithm")
	it := 0
	for len(edges) > 0 {
		runnerID, err := oracle.Reserve()
		if err != nil {
			return nil, fmt.Errorf("pradet could not reserve runner: %w", err)
		}

		it, deps := edgeSelectPraDet(g, edges, it)
		if it == -1 {
			break
		}

		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, edges[it].to)

		results, err := oracle.Get(runnerID).Run(schedule)
		oracle.Release(runnerID)
		if err != nil {
			return nil, fmt.Errorf("pradet could not run schedule: %w", err)
		}
		log.Debugf("run tests %v -> %v", schedule, results)

		g.RemoveDependency(edges[it].to, edges[it].from)

		if FindFailed(results) != -1 {
			g.AddDependency(edges[it].from, edges[it].to)
		}

		edges = append(edges[:it], edges[it+1:]...)
		it += 1
		if it == len(edges) {
			it = 0
		}
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
