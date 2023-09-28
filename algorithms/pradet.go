package algorithms

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pako-23/gtdd/runners"
)

func Pradet(tests []string, oracle *runners.RunnerSet) (DependencyGraph, error) {
	g := NewDependencyGraph(tests)
	edges := []edge{}
	triedEdges := 0

	for i := range tests {
		for j := i + 1; j < len(tests); j++ {
			edges = append(edges, edge{from: tests[j], to: tests[i]})
			g.AddDependency(tests[j], tests[i])
		}
	}

	log.Debug("starting dependency detection algorithm")
	it := 0
	for len(edges) > 0 {
		runnerID, err := oracle.Reserve()
		if err != nil {
			return nil, fmt.Errorf("pradet could not reserve runner: %w", err)
		}

		g.InvertDependency(edges[it].from, edges[it].to)
		triedEdges = 1
		deps := g.GetDependencies(edges[it].to)
		_, cycle := deps[edges[it].to]

		for cycle {
			if triedEdges >= len(edges) {
				it = -1
				break
			}

			g.InvertDependency(edges[it].to, edges[it].from)
			it += 1
			if it >= len(edges) {
				it = 0
			}
			triedEdges += 1
			g.InvertDependency(edges[it].from, edges[it].to)
			deps = g.GetDependencies(edges[it].to)
			_, cycle = deps[edges[it].to]
		}

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

		time.Sleep(StartUpTime)
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
		if it >= len(edges) {
			it = 0
		}
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
