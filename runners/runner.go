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

// Runner represents an environment where a test suite can be run.
type Runner struct {
	// The running containers for the application against which the test
	// suite is being run.
	app compose.AppInstance
	// The definition of the App against which the test suite is being run.
	appDefinition *compose.App
	// The running containers for the drivers needed to run the test suite.
	// An example could be the WebDriver to run a Selenium test suite.
	driver compose.AppInstance
	// A name associated with the runner.
	name string
	// The ID of the Docker network in which all the Docker containers needed
	// to run the test suite are running.
	network string
	// The test suite that should be run inside this runner.
	testSuite testsuite.TestSuite
	// The environment variables that should be passed to the container running
	// the test suite.
	testSuiteEnv []string
}

// RunnerConfig represents the configurations needed to create a runner.
type RunnerConfig struct {
	// The configuration coming from the initialization of a group of runners.
	RunnerSetConfig
	// The name to give to the runner.
	Name string
}

// NewRunner creates a new runner based on the runner configuration.
// If there is an error, it is returned.
func NewRunner(config *RunnerConfig) (*Runner, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to create runner %s: %w", config.Name, err)
	}
	defer cli.Close()
	ctx := context.Background()

	net, err := cli.NetworkCreate(ctx, config.Name, types.NetworkCreate{})
	if err != nil {
		return nil, fmt.Errorf("network creation failed for runner %s: %w", config.Name, err)
	}
	log.Debugf("successfully created network %s with ID %s", config.Name, net.ID)

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
	log.Debugf("successfully initialized runner %s", config.Name)
	return runner, nil
}

// translateEnv translates each hostname in the value of each environment
// variable based on the container names created by the runner. The resulting
// environment variables are returned.
func (r *Runner) translateEnv(variables []string) []string {
	newEnv := make([]string, len(variables))

	hosts := make([]string, 0, len(r.appDefinition.Services)+len(r.driver))
	for k := range r.appDefinition.Services {
		hosts = append(hosts, k)
	}
	for k := range r.driver {
		hosts = append(hosts, k)
	}

	for index, variable := range variables {
		before, after, _ := strings.Cut(variable, "=")

		for _, host := range hosts {
			re := regexp.MustCompile(fmt.Sprintf("\\b%s\\b", host))
			after = re.ReplaceAllString(after, fmt.Sprintf("%s-%s", host, r.name))
		}
		newEnv[index] = fmt.Sprintf("%s=%s", before, after)
	}
	return newEnv
}

// ResetApplication deletes the containers related to the currently running
// application and sets up the containers to run a provided application.
// If there is an error in the process, it is returned.
func (r *Runner) ResetApplication() error {
	if err := r.app.Delete(); err != nil {
		return fmt.Errorf("app deletion failed when resetting app on runner %s: %w", r.name, err)
	}

	instance, err := r.appDefinition.Start(&compose.StartConfig{
		Context:  r.name,
		Networks: []string{r.network},
	})
	if err != nil {
		return fmt.Errorf("app start-up failed when resetting app on runner %s: %w", r.name, err)
	}
	r.app = instance
	log.Debugf("successfully reset app on runner %s", r.name)

	return nil
}

// Delete releases all the resources allocated for the runner. If there is an
// error in the process, it is returned.
func (r *Runner) Delete() error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create client to delete runner %s: %w", r.name, err)
	}
	defer cli.Close()
	ctx := context.Background()

	if err := r.driver.Delete(); err != nil {
		return fmt.Errorf("driver deletion failed when deleting runner %s: %w", r.name, err)
	}
	log.Debugf("successfully deleted driver on runner %s", r.name)

	if err := r.app.Delete(); err != nil {
		return fmt.Errorf("app deletion failed when deleting runner %s: %w", r.name, err)
	}
	log.Debugf("successfully deleted app on runner %s", r.name)

	if err := cli.NetworkRemove(ctx, r.network); err != nil {
		return fmt.Errorf("network deletion failed when deleting runner %s: %w", r.name, err)
	}
	log.Debugf("successfully deleted network on runner %s", r.name)

	return nil
}

// Run runs a test schedule on this runner. The test results are represented
// as booleans. If the test is passed, the value is true; otherwise it is
// false. If there is any error, it is returned.
func (r *Runner) Run(tests []string) ([]bool, error) {
	results, err := r.testSuite.Run(&testsuite.RunConfig{
		Env:         r.testSuiteEnv,
		Tests:       tests,
		StartConfig: &compose.StartConfig{Networks: []string{r.network}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run test suite on runner %s: %w", r.name, err)
	}
	return results, nil
}
