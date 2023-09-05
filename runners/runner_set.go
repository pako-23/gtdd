package runners

import (
	"fmt"

	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// The configuration to create an set of runners
type RunnerSetConfig struct {
	// The App to configure on the runners.
	App *compose.App
	// The driver used to configure the runners.
	Driver *compose.App
	// The number of runners to include in the set.
	Runners uint
	// The testsuite to run into the runner.
	TestSuite testsuite.TestSuite
	//
	TestSuiteEnv []string
}

// RunnerSet defines a set of runners.
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
		return nil, err
	}

	for i := uint(0); i < config.Runners; i++ {
		runnerName := fmt.Sprintf("runner-%d", i)
		set.tokens <- runnerName
		runner, err := NewRunner(&RunnerConfig{
			RunnerSetConfig: *config,
			Name:            runnerName,
		})
		if err != nil {
			set.Delete()
			return nil, err
		}
		set.runners[runnerName] = runner
	}
	log.Infof("Successfully initialized %d runners", config.Runners)

	return &set, nil
}

// Delete releases all the resources needed by the set of runners.
// If there is an error in the process, it is returned.
func (r *RunnerSet) Delete() error {
	close(r.tokens)

	var g errgroup.Group

	for _, runner := range r.runners {
		g.Go(runner.Delete)
	}

	return g.Wait()
}

func (r *RunnerSet) Reserve() (string, error) {
	runnerId := <-r.tokens

	runner := r.runners[runnerId]

	if err := runner.ResetApplication(); err != nil {
		r.Release(runnerId)
		return "", err
	}

	return runnerId, nil
}

func (r *RunnerSet) Release(runnerId string) {
	r.tokens <- runnerId
}

func (r *RunnerSet) Get(runnerId string) *Runner {
	return r.runners[runnerId]
}
