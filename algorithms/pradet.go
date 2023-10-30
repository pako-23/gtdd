package algorithms

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/pako-23/gtdd/runners"
)

type PraDet struct{}

func edgeSelectPraDet(g DependencyGraph, edges []edge, it int) (int, map[string]struct{}) {
	triedEdges := 1
	g.InvertDependency(edges[it].from, edges[it].to)
	// log.Infof("invert edge %d/%d: %s -> %s", *it+1, len(edges), edges[*it].from, edges[*it].to)

	deps := g.GetDependencies(edges[it].to)

	_, cycle := deps[edges[it].to]

	for cycle {
		// log.Infof("cycle in edge %d/%d: %s -> %s", *it+1, len(edges), edges[*it].from, edges[*it].to)
		g.InvertDependency(edges[it].to, edges[it].from)
		if triedEdges == len(edges) {
			return -1, nil
		}

		it += 1
		if it == len(edges) {
			it = 0
		}
		triedEdges += 1

		// log.Infof("invert edge %d/%d: %s -> %s", *it+1, len(edges), edges[*it].from, edges[*it].to)
		g.InvertDependency(edges[it].from, edges[it].to)
		deps = g.GetDependencies(edges[it].to)
		_, cycle = deps[edges[it].to]
	}

	return it, deps
}

func (_ *PraDet) FindDependencies(tests []string, oracle *runners.RunnerSet) (DependencyGraph, error) {
	g := NewDependencyGraph(tests)
	edges := []edge{}

	for i := 1; i < len(tests); i++ {
		for j := i; j < len(tests); j++ {
			edges = append(edges, edge{from: tests[j], to: tests[j-i]})
			g.AddDependency(tests[j], tests[j-i])
		}
	}
	log.Debug("starting dependency detection algorithm")

	it, deps := edgeSelectPraDet(g, edges, 0)
	for it > 0 {
		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, edges[it].to)

		runnerID, err := oracle.Reserve()
		if err != nil {
			return nil, fmt.Errorf("pradet could not reserve runner: %w", err)
		}

		results, err := oracle.Get(runnerID).Run(schedule)
		go oracle.Release(runnerID)
		if err != nil {
			return nil, fmt.Errorf("pradet could not run schedule: %w", err)
		}
		log.Debugf("run tests %v -> %v", schedule, results)

		g.RemoveDependency(edges[it].to, edges[it].from)

		for i, test := range schedule {
			if test == edges[it].from {
				if !results[i] {
					g.AddDependency(edges[it].from, edges[it].to)
				}
				edges = append(edges[:it], edges[it+1:]...)
				break
			} else if !results[i] {
				g.AddDependency(edges[it].from, edges[it].to)
				break
			}
		}

		// isManifest := false
		// for i, test := range schedule {
		// 	if test == edges[it].from && !results[i] {
		// 		isManifest = results[i]

		// 		// g.AddDependency(edges[it].from, edges[it].to)
		// 		// edges = append(edges[:it], edges[it+1:]...)
		// 		break
		// 	}
		// }

		// if isManifest

		// if FindFailed(results) != -1 {
		// log.Infof("keeping edge %d/%d: %s -> %s", it+1, len(edges), edges[it].from, edges[it].to)
		// }
		// else {
		// log.Infof("removing edge %d/%d: %s -> %s", it+1, len(edges), edges[it].from, edges[it].to)
		// }

		// edges = append(edges[:it], edges[it+1:]...)
		if it == len(edges) {
			it = 0
		}

		it, deps = edgeSelectPraDet(g, edges, 0)
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
