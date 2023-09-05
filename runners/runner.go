package runners

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/pako-23/gtdd/compose"
	"github.com/pako-23/gtdd/testsuite"
	log "github.com/sirupsen/logrus"
)

// RunnerConfig contains the configurations needed to create a runner.
type RunnerConfig struct {
	RunnerSetConfig
	Name string
}

// Runner is an environment where the tests are run.
type Runner struct {
	// The container for the driver to run the tests.
	driver compose.AppInstance
	// The running containers which are part of the application.
	app compose.AppInstance
	// The name of the runner.
	name string
	// The network to which the application containers are attached.
	network       string
	appDefinition *compose.App
	testSuite     testsuite.TestSuite
	testSuiteEnv  []string
}

// NewRunner creates a new runner with the provided configuration. If there
// is an error in creating the runner, it is returned.
func NewRunner(config *RunnerConfig) (*Runner, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	ctx := context.Background()

	net, err := cli.NetworkCreate(ctx, config.Name, types.NetworkCreate{})
	if err != nil {
		return nil, err
	}
	log.Debugf("Successfully created network %s with ID %s", config.Name, net.ID)

	runner := &Runner{
		appDefinition: config.App,
		name:          config.Name,
		network:       net.ID,
		testSuite:     config.TestSuite,
	}

	if config.Driver != nil {
		driver, err := config.Driver.Start(&compose.StartConfig{
			Context:  runner.name,
			Networks: []string{net.ID},
		})
		if err != nil {
			if netErr := cli.NetworkRemove(ctx, net.ID); netErr != nil {
				log.Error(netErr)
			}
			return nil, err
		}
		runner.driver = driver
	}
	runner.testSuiteEnv = runner.translateEnv(config.TestSuiteEnv)
	log.Debugf("Successfully initialized runner %s", config.Name)
	return runner, nil
}

// translateEnv turns the environment variables into the runner specific
// environment variables.
func (r *Runner) translateEnv(vars []string) []string {
	newEnv := make([]string, len(vars))

	hosts := make([]string, 0, len(r.appDefinition.Services)+len(r.driver))
	for k := range r.appDefinition.Services {
		hosts = append(hosts, k)
	}
	for k := range r.driver {
		hosts = append(hosts, k)
	}

	for i, v := range vars {
		before, after, _ := strings.Cut(v, "=")

		for _, host := range hosts {
			re := regexp.MustCompile(fmt.Sprintf("\\b%s\\b", host))
			after = re.ReplaceAllString(after, fmt.Sprintf("%s-%s", host, r.name))
		}
		newEnv[i] = fmt.Sprintf("%s=%s", before, after)
	}
	return newEnv
}

// ResetApplication deletes the containers related to the currently running
// application and sets up the containers to run a provided application.
// If there is an error in the process, it is returned.
func (r *Runner) ResetApplication() error {
	if err := r.app.Delete(); err != nil {
		return err
	}

	instance, err := r.appDefinition.Start(&compose.StartConfig{
		Context:  r.name,
		Networks: []string{r.network},
	})
	if err != nil {
		return err
	}
	r.app = instance

	return nil
}

// Delete releases all the resources needed by the runner.
// If there is an error in the process, it is returned.
func (r *Runner) Delete() error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx := context.Background()

	if err := r.driver.Delete(); err != nil {
		return err
	}

	if err := r.app.Delete(); err != nil {
		return err
	}

	if err := cli.NetworkRemove(ctx, r.network); err != nil {
		return err
	}
	log.Debugf("Successfully deleted network with ID %s", r.network)

	return nil
}

// Run starts and App inside the runner and returns the instance.
// If there is an error in the process, it is returned.
func (r *Runner) Run(tests []string) ([]bool, error) {
	return r.testSuite.Run(&testsuite.RunConfig{
		Env:         r.testSuiteEnv,
		Tests:       tests,
		StartConfig: &compose.StartConfig{Networks: []string{r.network}},
	})
}
