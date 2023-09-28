package algorithms

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pako-23/gtdd/runners"
)

// exLinearContext represents all the context data to execute one iteration of
// the ex-linear strategy to detect dependencies between tests.
type exLinearContext struct {
	// The schedule from the previous iteration of the algorithm.
	previousSchedule []string
	// The test excluded from the first iteration of the algorithm.
	excludedTest string
	// The index of the test that failed from the previous iteration.
	failedTest int
	// The runners on which the test suites are run.
	runners *runners.RunnerSet
}

// exLinearIteration performs one iteration of the ex-linear strategy to detect
// dependencies between the tests of a test suite. The strategy works
// as follows:
//
//   - Remove one test from the original test suite.
//   - Run the resulting schedule.
//   - If some test fails add one edge in the graph of tests dependencies
//     from the failed test and the test initially excluded. Then, proceed with
//     a new iteration where the failed test is also excluded. If no test
//     failed, do nothing.
func exLinearIteration(ctx *exLinearContext, n *sync.WaitGroup, ch chan<- edgeChannelData) {
	defer n.Done()
	schedule := remove(ctx.previousSchedule, ctx.failedTest)
	runnerID, err := ctx.runners.Reserve()
	if err != nil {
		ch <- edgeChannelData{edge: edge{from: "", to: ""}, err: err}

		return
	}
	defer ctx.runners.Release(runnerID)

	time.Sleep(StartUpTime)
	results, err := ctx.runners.Get(runnerID).Run(schedule)
	if err != nil {
		ch <- edgeChannelData{edge: edge{from: "", to: ""}, err: err}

		return
	}

	log.Debugf("run tests %v -> %v", schedule, results)

	firstFailed := FindFailed(results)
	if firstFailed == -1 {
		ch <- edgeChannelData{edge: edge{from: "", to: ""}, err: nil}
	} else {
		ch <- edgeChannelData{
			edge: edge{
				from: schedule[firstFailed],
				to:   ctx.excludedTest,
			},
			err: err,
		}

		if len(schedule) == 1 {
			return
		}

		n.Add(1)
		go exLinearIteration(&exLinearContext{
			previousSchedule: schedule,
			excludedTest:     ctx.excludedTest,
			failedTest:       firstFailed,
			runners:          ctx.runners,
		}, n, ch)
	}
}

// ExLinear implements the ex-linear strategy to detect dependencies between
// the tests into a given test suite. If there is any error, it is returned.
func ExLinear(tests []string, r *runners.RunnerSet) (DependencyGraph, error) {
	ch := make(chan edgeChannelData)
	n := sync.WaitGroup{}
	g := NewDependencyGraph(tests)

	log.Debug("starting dependency detection algorithm")

	for i := 0; i < len(tests)-1; i++ {
		n.Add(1)

		go exLinearIteration(&exLinearContext{
			previousSchedule: tests,
			excludedTest:     tests[i],
			failedTest:       i,
			runners:          r,
		}, &n, ch)
	}

	go func() {
		n.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.err != nil {
			return nil, result.err
		} else if result.from != "" && result.to != "" {
			g.AddDependency(result.from, result.to)
		}
	}

	log.Debug("finished dependency detection algorithm")

	return g, nil
}
