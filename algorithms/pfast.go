package algorithms

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/pako-23/gtdd/runners"
)

type PFAST struct{}

// iterationPFAST performs one iteration of the pfast strategy to detect
// dependencies between the tests of a test suite. The strategy works
// as follows:
//
//   - Remove one test from the original test suite.
//   - Run the resulting schedule.
//   - If some test fails add one edge in the graph of tests dependencies
//     from the failed test and the test initially excluded. Then, proceed with
//     a new iteration where the failed test is also excluded. If no test
//     failed, do nothing.
func iterationPFAST(ctx context.Context, excludedTest, failedTest int, previousSchedule []string, ch chan<- edgeChannelData) {
	var (
		n       = ctx.Value("wait-group").(*sync.WaitGroup)
		runners = ctx.Value("runners").(*runners.RunnerSet)
		tests   = ctx.Value("tests").([]string)
	)

	defer n.Done()
	schedule := remove(previousSchedule, failedTest)
	runnerID, err := runners.Reserve()
	if err != nil {
		ch <- edgeChannelData{edge: edge{from: "", to: ""}, err: err}

		return
	}
	defer func() { go runners.Release(runnerID) }()

	results, err := runners.Get(runnerID).Run(schedule)
	if err != nil {
		ch <- edgeChannelData{edge: edge{from: "", to: ""}, err: err}

		return
	}
	log.Debugf("run tests %v -> %v", schedule, results)

	if firstFailed := slices.Index(results, false); firstFailed == -1 {
		return
	} else if firstFailed < excludedTest {
		n.Add(1)
		go iterationPFAST(ctx, excludedTest, failedTest, previousSchedule, ch)
	} else if firstFailed != -1 {
		ch <- edgeChannelData{
			edge: edge{
				from: schedule[firstFailed],
				to:   tests[excludedTest],
			},
			err: nil,
		}

		if len(schedule) == 1 {
			return
		}

		n.Add(1)
		go iterationPFAST(ctx, excludedTest, firstFailed, schedule, ch)
	}
}

func findPossibleTargets(tests []string, g *DependencyGraph) map[string]int {
	targets := map[string]int{}
	visited := map[string]struct{}{}

	for i := len(tests) - 1; i >= 0; i-- {
		if _, ok := visited[tests[i]]; ok {
			continue
		}

		deps := g.GetDependencies(tests[i])
		targets[tests[i]] = len(deps)
		visited[tests[i]] = struct{}{}

		for dep := range deps {
			visited[dep] = struct{}{}
		}
	}

	return targets
}

func selectTarget(targets map[string]int) string {
	res := ""
	max := 0

	for target, value := range targets {
		if value > max {
			res = target
			max = value
		}
	}

	return res
}

func detectFailingTests(ctx context.Context, schedules [][]string) (map[string]map[int]struct{}, error) {
	type results struct {
		results  []bool
		schedule int
		err      error
	}

	runners := ctx.Value("runners").(*runners.RunnerSet)
	ch := make(chan results)
	notPassing := map[string]map[int]struct{}{}

	for i := range schedules {
		go func(index int) {
			runnerID, err := runners.Reserve()
			if err != nil {
				ch <- results{err: err}

				return
			}

			out, err := runners.Get(runnerID).Run(schedules[index])
			go runners.Release(runnerID)
			if err != nil {
				ch <- results{err: err}

				return
			}
			log.Debugf("run tests %v -> %v", schedules[index], out)

			ch <- results{schedule: index, results: out, err: nil}
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

func cleanAddedEdges(ctx context.Context, i int, test string, g *DependencyGraph) error {
	var (
		runners = ctx.Value("runners").(*runners.RunnerSet)
		tests   = ctx.Value("tests").([]string)
	)

	targets := findPossibleTargets(tests[:i+1], g)
	for target := range targets {
		g.AddDependency(test, target)
	}

	for target := selectTarget(targets); target != ""; target = selectTarget(targets) {
		g.RemoveDependency(test, target)
		deps := g.GetDependencies(test)

		schedule := []string{}
		for _, test := range tests {
			if _, ok := deps[test]; ok {
				schedule = append(schedule, test)
			}
		}
		schedule = append(schedule, test)

		runnerID, err := runners.Reserve()
		if err != nil {
			return err
		}

		results, err := runners.Get(runnerID).Run(schedule)
		go runners.Release(runnerID)
		if err != nil {
			return err
		}
		log.Debugf("run tests %v -> %v", schedule, results)

		if firstFailed := slices.Index(results, false); firstFailed == -1 {
			delete(targets, target)
		} else if firstFailed == len(schedule)-1 {
			delete(targets, target)
			g.AddDependency(test, target)
		}
	}
	return nil
}

func recoveryPFAST(ctx context.Context, g *DependencyGraph) error {
	var (
		runners = ctx.Value("runners").(*runners.RunnerSet)
		tests   = ctx.Value("tests").([]string)
	)

	schedules := g.GetSchedules(tests)
	notPassingTests, err := detectFailingTests(ctx, schedules)
	if err != nil {
		return err
	}

	log.Infof("failing tests %v", notPassingTests)

	passedSchedules := map[int]struct{}{}

	for i, test := range tests {
		if solvedSchedule(notPassingTests, passedSchedules, test) {
			continue
		}

		if err := cleanAddedEdges(ctx, i, test, g); err != nil {
			return err
		}

		deps := g.GetDependencies(test)
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
			schedule = append(schedule, schedules[s][index+1:]...)

			runnerID, err := runners.Reserve()
			if err != nil {
				return err
			}

			results, err := runners.Get(runnerID).Run(schedule)
			go runners.Release(runnerID)
			if err != nil {
				return err
			}
			log.Debugf("run tests %v -> %v", schedule, results)

			if firstFailed := slices.Index(results, false); firstFailed == -1 {
				passedSchedules[s] = struct{}{}
			}
		}
	}

	return nil
}

// PFAST implements the pfast strategy to detect dependencies between
// the tests into a given test suite. If there is any error, it is returned.
func (_ *PFAST) FindDependencies(tests []string, r *runners.RunnerSet) (DependencyGraph, error) {
	ch := make(chan edgeChannelData)
	n := sync.WaitGroup{}
	g := NewDependencyGraph(tests)

	log.Debug("starting dependency detection algorithm")

	ctx := context.WithValue(
		context.WithValue(
			context.WithValue(
				context.Background(), "runners", r,
			),
			"wait-group", &n,
		), "tests", tests,
	)

	for i := 0; i < len(tests)-1; i++ {
		n.Add(1)

		go iterationPFAST(ctx, i, i, tests, ch)
	}

	go func() {
		n.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.err != nil {
			return nil, result.err
		}
		g.AddDependency(result.from, result.to)
	}

	log.Info("Start recovery")
	if err := recoveryPFAST(ctx, &g); err != nil {
		return nil, err
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
