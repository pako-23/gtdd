package algorithms

import (
	"sort"

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/pako-23/gtdd/internal/runner"
)

func detectFailingTests(runners *runner.RunnerSet, schedules [][]string) (map[string]map[int]struct{}, error) {
	type results struct {
		results  []bool
		schedule int
		err      error
	}

	ch := make(chan results)
	notPassing := map[string]map[int]struct{}{}

	for i := range schedules {
		go func(index int) {
			out, err := runners.RunSchedule(schedules[index])
			if err != nil {
				ch <- results{err: err}

				return
			}
			log.Debugf("run tests %v -> %v", schedules[index], out.Results)

			ch <- results{schedule: index, results: out.Results, err: nil}
		}(i)
	}

	for i := 0; i < len(schedules); i++ {
		results := <-ch
		if results.err != nil {
			close(ch)
			return nil, results.err
		}

		for j, test := range schedules[results.schedule] {
			if results.results[j] {
				continue
			}

			if _, ok := notPassing[test]; !ok {
				notPassing[test] = map[int]struct{}{}
			}

			notPassing[test][results.schedule] = struct{}{}
		}
	}

	return notPassing, nil
}

func solvedSchedule(notPassingTests map[string]map[int]struct{}, passedSchedules map[int]struct{}, test string) bool {
	schedules, ok := notPassingTests[test]
	if !ok {
		return true
	}

	for s := range schedules {
		if _, ok := passedSchedules[s]; !ok {
			return false
		}
	}

	return true
}

type dependency struct {
	test string
	rank int
}

func findTargets(tests []string, g *DependencyGraph) []dependency {
	targets := make([]dependency, 0, len(tests))

	for i := 0; i < len(tests); i++ {
		targets = append(targets, dependency{
			test: tests[i],
			rank: len(g.getDependencies(tests[i])),
		})
	}

	sort.SliceStable(targets, func(i, j int) bool {
		return targets[i].rank < targets[j].rank
	})
	return targets
}

func solveNode(tests []string, runners *runner.RunnerSet, i int, test string, g *DependencyGraph) error {
	targets := findTargets(tests[:i], g)
	end := 0
	for i, target := range targets {
		log.Infof("recovery add edge %s -> %s", test, target.test)
		g.addDependency(test, target.test)
		deps := g.getDependencies(test)

		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, test)
		results, err := runners.RunSchedule(schedule)
		if err != nil {
			return err
		}
		log.Debugf("run tests %v -> %v", schedule, results.Results)

		if firstFailed := slices.Index(results.Results, false); firstFailed == -1 {
			end = i
			break
		}
	}

	for i, target := range targets {
		if i == end {
			break
		}
		g.removeDependency(test, target.test)
		deps := g.getDependencies(test)

		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, test)
		results, err := runners.RunSchedule(schedule)
		if err != nil {
			return err
		}
		log.Debugf("run tests %v -> %v", schedule, results.Results)

		if firstFailed := slices.Index(results.Results, false); firstFailed != -1 {
			g.addDependency(test, target.test)
		}
	}

	return nil
}

func recoveryPFAST(tests []string, runners *runner.RunnerSet, g *DependencyGraph) error {
	schedules := g.GetSchedules(tests)
	notPassingTests, err := detectFailingTests(runners, schedules)
	if err != nil {
		return err
	}

	log.Infof("failing tests %v", notPassingTests)

	passedSchedules := map[int]struct{}{}
	for i, test := range tests {
		log.Infof("recovery working on test %s", test)
		if solvedSchedule(notPassingTests, passedSchedules, test) {
			continue
		}

		if err := solveNode(tests, runners, i, test, g); err != nil {
			return err
		}

		deps := g.getDependencies(test)
		prefix := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				prefix = append(prefix, test)
			}
		}

		for s := range notPassingTests[test] {
			if _, ok := passedSchedules[s]; ok {
				continue
			}

			index := slices.Index(schedules[s], test)
			schedule := prefix
			schedule = append(schedule, schedules[s][index:]...)
			results, err := runners.RunSchedule(schedule)
			if err != nil {
				return err
			}
			log.Debugf("run tests %v -> %v", schedule, results.Results)

			if firstFailed := slices.Index(results.Results, false); firstFailed == -1 {
				passedSchedules[s] = struct{}{}
			}
		}
	}

	return nil
}

func PFAST(tests []string, r *runner.RunnerSet) (DependencyGraph, error) {
	type result struct {
		edge
		err error
	}

	type job struct {
		schedule []string
		toRemove int
		excluded int
	}

	results := make(chan result, r.Size())
	jobs := make(chan job, r.Size())
	done := make(chan struct{})

	g := NewDependencyGraph(tests)

	// start workers
	for i := 0; i < r.Size()+1; i++ {
		go func() {
			for job := range jobs {
				job.schedule = remove(job.schedule, job.toRemove)
				for {
					out, err := r.RunSchedule(job.schedule)
					if err != nil {
						results <- result{edge: edge{from: "", to: ""}, err: err}

						return
					}
					log.Debugf("run tests %v -> %v", job.schedule, out.Results)

					if firstFailed := slices.Index(out.Results, false); firstFailed == -1 {
						done <- struct{}{}
						break
					} else if firstFailed < job.excluded {
						continue
					} else if firstFailed != -1 {
						results <- result{
							edge: edge{
								from: job.schedule[firstFailed],
								to:   tests[job.excluded],
							},
							err: nil,
						}

						if len(job.schedule) != 1 {
							job.toRemove = firstFailed
							jobs <- job
						} else {
							done <- struct{}{}
						}
						break
					}
				}
			}
		}()
	}

	log.Debug("starting dependency detection algorithm")
	go func() {
		for i := 0; i < len(tests)-1; i++ {
			jobs <- job{schedule: tests, toRemove: i, excluded: i}
		}
	}()

	jobsNum := len(tests) - 1

	for jobsNum > 0 {
		select {
		case res := <-results:
			if res.err != nil {
				return nil, res.err
			}

			g.addDependency(res.from, res.to)
		case <-done:
			jobsNum--
		}
	}
	close(jobs)

	g.transitiveReduction()
	if err := recoveryPFAST(tests, r, &g); err != nil {
		return nil, err
	}

	log.Debug("finished dependency detection algorithm")
	g.transitiveReduction()

	return g, nil
}
