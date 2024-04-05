package compose_runner

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pako-23/gtdd/internal/testsuite"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// The default number of runners into a set of runners.
const DefaultSetSize = 1

var ErrNoRunner = errors.New("no runner to reserve")

// RunnerSetConfig represents the configurations needed to create a RunnerSer.
type RunnerSetConfig struct {
	// The App to configure on the runners.
	App string
	// The driver used to configure the runners.
	Driver string
	// The number of runners to include in the set.
	Runners uint
	// The test suite to run into the runner.
	TestSuite testsuite.TestSuite
	// The environment variables to pass to the test suite when running it.
	TestSuiteEnv []string
}

// RunnerSet represents a group of runners used to run a test suites.
type RunnerSet struct {
	// The mapping from the name to the actual runner.
	runners map[string]*Runner
	// Tokens to reserve a runner.
	tokens chan string
}

// NewRunnerSet creates a new set of runner with the provided configuration.
// If there is an error in creating the set of runners, it is returned.
func NewRunnerSet(config *RunnerSetConfig) (*RunnerSet, error) {
	var n sync.WaitGroup

	set := RunnerSet{
		runners: map[string]*Runner{},
		tokens:  make(chan string, config.Runners),
	}

	for i := uint(0); i < config.Runners; i++ {
		runnerName := fmt.Sprintf("runner-%d", i)
		runner, err := NewRunner(&RunnerConfig{
			RunnerSetConfig: *config,
			Name:            runnerName,
		})
		if err != nil {
			if deleteErr := set.Delete(); deleteErr != nil {
				log.Error(deleteErr)
			}

			return nil, err
		}
		set.runners[runnerName] = runner

		n.Add(1)
		go func(runnerName string) {
			defer n.Done()

			set.Release(runnerName)
		}(runnerName)
	}
	n.Wait()
	log.Infof("successfully initialized %d runners", len(set.runners))

	return &set, nil
}

// Delete releases all the resources needed by the set of runners.
// If there is an error in the process, it is returned.
func (r *RunnerSet) Delete() error {
	var waitgroup errgroup.Group

	for range r.runners {
		waitgroup.Go(func() error {
			return r.Get(<-r.tokens).Delete()
		})
	}

	if err := waitgroup.Wait(); err != nil {
		return fmt.Errorf("failed to delete set of runners: %w", err)
	}
	close(r.tokens)

	return nil
}

// Reserve reserves one runner from the set and returns its identifier.
// When reserving the runner, the application on the runner is also reset.
// If there is any error in reserving the runner or resetting the application,
// it is returned.
func (r *RunnerSet) Reserve() (string, error) {
	if len(r.runners) == 0 {
		return "", ErrNoRunner
	}

	return <-r.tokens, nil
}

// Release deletes the reservation for a given runner. It requires the
// identifier of the reserved runner to release it.
func (r *RunnerSet) Release(runnerID string) {
	if runner, ok := r.runners[runnerID]; !ok {
		log.Errorf("failed to release runner %s: runner not found", runnerID)

		return
	} else if err := runner.ResetApplication(); err != nil {
		log.Errorf("failed to reset application on runner %s: %v", runnerID, err)
		if err = runner.Delete(); err != nil {
			log.Errorf("failed to delete runner %s: %v", runnerID, err)
		}
		delete(r.runners, runnerID)

		return
	}

	r.tokens <- runnerID
}

// Get returns the actual runner from a given identifier.
func (r *RunnerSet) Get(runnerID string) *Runner {
	return r.runners[runnerID]
}
