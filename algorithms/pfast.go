package algorithms

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

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

	firstFailed := FindFailed(results)
	if firstFailed == -1 {
		return
	}

	if firstFailed < excludedTest {
		n.Add(1)

		// log.Infof("failed smaller tests, excluded test: %s, tests: %v, failed test: %s", tests[excludedTest], tests, tests[firstFailed])
		go iterationPFAST(ctx, excludedTest, firstFailed, previousSchedule, ch)
	} else {
		// log.Infof("sending over channel: %v, test results  %v -> %v", d, schedule, results)
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
		// schedules := g.GetSchedules(tests)

		close(ch)
	}()

	for result := range ch {
		log.Debugf("channel receive: %v", result)
		if result.err != nil {
			return nil, result.err
		}

		// log.Infof("adding edge: %s -> %s", result.from, result.to)

		g.AddDependency(result.from, result.to)
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
