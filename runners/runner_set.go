package runners

import (
	"fmt"

	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// The default number of runners into a set of runners.
const DefaultSetSize = 1

// RunnerSetConfig represents the configurations needed to create a RunnerSer.
type RunnerSetConfig struct {
	// The App to configure on the runners.
	App *compose.App
	// The driver used to configure the runners.
	Driver *compose.App
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
	set := RunnerSet{
		runners: map[string]*Runner{},
		tokens:  make(chan string, config.Runners),
	}
	if err := config.Driver.Pull(); err != nil {
		return nil, fmt.Errorf("failed to pull driver artifacts when creating set of runners: %w", err)
	}
	log.Debugf("successfully pulled images for the test driver")

	for i := uint(0); i < config.Runners; i++ {
		runnerName := fmt.Sprintf("runner-%d", i)
		set.tokens <- runnerName
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
	}
	log.Infof("successfully initialized %d runners", config.Runners)

	return &set, nil
}

// Delete releases all the resources needed by the set of runners.
// If there is an error in the process, it is returned.
func (r *RunnerSet) Delete() error {
	close(r.tokens)

	var waitgroup errgroup.Group

	for _, runner := range r.runners {
		waitgroup.Go(runner.Delete)
	}

	if err := waitgroup.Wait(); err != nil {
		return fmt.Errorf("failed to delete set of runners: %w", err)
	}

	return nil
}

// Reserve reserves one runner from the set and returns its identifier.
// When reserving the runner, the application on the runner is also reset.
// If there is any error in reserving the runner or resetting the application,
// it is returned.
func (r *RunnerSet) Reserve() (string, error) {
	runnerID := <-r.tokens

	runner := r.runners[runnerID]

	if err := runner.ResetApplication(); err != nil {
		r.Release(runnerID)

		return "", err
	}

	return runnerID, nil
}

// Release deletes the reservation for a given runner. It requires the
// identifier of the reserved runner to release it.
func (r *RunnerSet) Release(runnerID string) {
	r.tokens <- runnerID
}

// Get returns the actual runner from a given identifier.
func (r *RunnerSet) Get(runnerID string) *Runner {
	return r.runners[runnerID]
}
