package runner

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// The default number of runners into a set of runners.
const DefaultSetSize = 1

var (
	ErrNoRunner           = errors.New("no runner to reserve")
	ErrWrongRunnerSetSize = errors.New("a runner set must have at least size 1")
)

// // RunnerSetConfig represents the configurations needed to create a RunnerSer.
// type RunnerSetConfig struct {
// 	// The App to configure on the runners.
// 	App string
// 	// The driver used to configure the runners.
// 	Driver string
// 	// The number of runners to include in the set.
// 	Runners uint
// 	// The test suite to run into the runner.
// 	TestSuite testsuite.TestSuite
// 	// The environment variables to pass to the test suite when running it.
// 	TestSuiteEnv []string
// }

type RunResults struct {
	Results     []bool
	RunningTime time.Duration
}

// RunnerSet represents a group of runners used to run a test suites.
type RunnerSet[T Runner] struct {
	// The mapping from the name to the actual runner.
	runners map[string]T
	// Tokens to reserve a runner.
	tokens chan string
	mu     sync.Mutex
}

// NewRunnerSet creates a new set of runner with the provided configuration.
// If there is an error in creating the set of runners, it is returned.
func NewRunnerSet[T Runner](size uint, builder RunnerBuilder[T], options ...RunnerOption[T]) (*RunnerSet[Runner], error) {
	var n sync.WaitGroup

	if size < 1 {
		return nil, ErrWrongRunnerSetSize
	}

	set := RunnerSet[Runner]{
		runners: map[string]Runner{},
		tokens:  make(chan string, size),
		mu:      sync.Mutex{},
	}

	for i := uint(0); i < size; i++ {
		runnerName := fmt.Sprintf("runner-%d", i)
		runner, err := builder(runnerName, options...)

		if err != nil {
			if deleteErr := set.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}

			return nil, err
		}
		set.runners[runnerName] = runner

		n.Add(1)
		go func(runnerName string, runner Runner) {
			defer n.Done()

			set.release(runnerName, runner)
		}(runnerName, runner)
	}
	n.Wait()
	log.Infof("successfully initialized %d runners", len(set.runners))

	return &set, nil
}

// Delete releases all the resources needed by the set of runners.
// If there is an error in the process, it is returned.
func (r *RunnerSet[T]) Delete() error {
	var waitgroup errgroup.Group

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, runner := range r.runners {
		waitgroup.Go(func(runner Runner) func() error {
			return func() error {
				return runner.Delete()
			}
		}(runner))
	}

	if err := waitgroup.Wait(); err != nil {
		return fmt.Errorf("failed to delete set of runners: %w", err)
	}
	close(r.tokens)
	r.runners = nil

	return nil
}

// Reserve reserves one runner from the set and returns its identifier.
// When reserving the runner, the application on the runner is also reset.
// If there is any error in reserving the runner or resetting the application,
// it is returned.
// func (r *RunnerSet[T]) Reserve() (string, error) {
// 	if len(r.runners) == 0 {
// 		return "", ErrNoRunner
// 	}

// 	return <-r.tokens, nil
// }

// Release deletes the reservation for a given runner. It requires the
// identifier of the reserved runner to release it.
// func (r *RunnerSet[T]) Release(runnerID string) {
// 	if runner, ok := r.runners[runnerID]; !ok {
// 		log.Errorf("failed to release runner %s: runner not found", runnerID)

// 		return
// 	} else if err := runner.ResetApplication(); err != nil {
// 		log.Errorf("failed to reset application on runner %s: %v", runnerID, err)
// 		if err = runner.Delete(); err != nil {
// 			log.Errorf("failed to delete runner %s: %v", runnerID, err)
// 		}
// 		delete(r.runners, runnerID)

// 		return
// 	}

// 	r.tokens <- runnerID
// }

func (r *RunnerSet[T]) release(runnerID string, runner Runner) {
	if err := runner.ResetApplication(); err != nil {
		log.Errorf("failed to reset application on runner %s: %v", runnerID, err)
		if err = runner.Delete(); err != nil {
			log.Errorf("failed to delete runner %s: %v", runnerID, err)
		}
		r.mu.Lock()
		delete(r.runners, runnerID)
		r.mu.Unlock()

		return
	}

	r.tokens <- runnerID
}

// Get returns the actual runner from a given identifier.
// func (r *RunnerSet[T]) Get(runnerID string) T {
// 	return r.runners[runnerID]
// }

// func (r *RunnerSet[T]) Reserve(schedule []string) ([]bool, error) {
// 	r.mu.Lock()
// 	if len(r.runners) == 0 {
// 		r.mu.Unlock()
// 		return nil, ErrNoRunner
// 	}

// 	runnerID := <-r.tokens
// 	runner := r.runners[runnerID]
// 	r.mu.Unlock()

// 	result, err := runner.Run(schedule)

// 	go r.release(runnerID, runner)

// 	return result, err
// }

func (r *RunnerSet[T]) RunSchedule(schedule []string) (RunResults, error) {
	r.mu.Lock()
	if len(r.runners) == 0 {
		r.mu.Unlock()
		return RunResults{}, ErrNoRunner
	}

	runnerID := <-r.tokens
	runner := r.runners[runnerID]
	r.mu.Unlock()

	start := time.Now()
	result, err := runner.Run(schedule)
	duration := time.Since(start)

	go r.release(runnerID, runner)

	return RunResults{
		Results:     result,
		RunningTime: duration,
	}, err
}
