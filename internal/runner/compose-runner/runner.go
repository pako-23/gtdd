package compose_runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pako-23/gtdd/internal/docker"
	"github.com/pako-23/gtdd/internal/runner"
	"github.com/pako-23/gtdd/internal/testsuite"
	log "github.com/sirupsen/logrus"
)

// Runner represents an environment where a test suite can be run.
type ComposeRunner struct {
	// The running containers for the application against which the test
	// suite is being run.
	app docker.AppInstance
	// The definition of the App against which the test suite is being run.
	appDefinition docker.App
	// The running containers for the drivers needed to run the test suite.
	// An example could be the WebDriver to run a Selenium test suite.
	driver docker.AppInstance
	// A name associated with the runner.
	id string
	// The ID of the Docker network in which all the Docker containers needed
	// to run the test suite are running.
	network string
	// The test suite that should be run inside this runner.
	testSuite *testsuite.TestSuite
	// The environment variables that should be passed to the container running
	// the test suite.
	translatedEnv []string

	env []string

	client *docker.Client
}

// NewRunner creates a new runner based on the runner configuration.
// If there is an error, it is returned.
func ComposeRunnerBuilder(id string, options ...runner.RunnerOption[*ComposeRunner]) (*ComposeRunner, error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client to create runner %s: %w", id, err)
	}

	net, err := client.NetworkCreate(id)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create network: %w", err)
	}
	log.Debugf("[runner=%s] successfully created network with ID %s", id, net)

	runner := &ComposeRunner{
		client:  client,
		network: net,
		app:     docker.AppInstance{},
		id:      id,
	}

	for _, option := range options {
		if err := option(runner); err != nil {
			// TODO: handle errors
			return nil, err
		}

	}

	runner.translatedEnv = runner.translateEnv(runner.env)

	return runner, nil
}

func WithAppDefinition(path string) func(*ComposeRunner) error {
	return func(runner *ComposeRunner) error {
		app, err := runner.client.NewApp(path)
		if err != nil {
			return err
		}

		runner.appDefinition = app
		return nil
	}

}

func WithDriverDefinition(path string) func(*ComposeRunner) error {
	return func(runner *ComposeRunner) error {
		definition, err := runner.client.NewApp(path)
		if err != nil {
			return err
		}

		driver, err := runner.client.Run(definition, docker.RunOptions{
			Prefix:   runner.Id(),
			Networks: []string{runner.network}})
		if err != nil {
			return err
		}

		runner.driver = driver

		return nil
	}
}

func WithEnv(env []string) func(*ComposeRunner) error {
	return func(runner *ComposeRunner) error {
		runner.env = env
		return nil
	}
}

func WithTestSuite(suite *testsuite.TestSuite) func(*ComposeRunner) error {
	return func(runner *ComposeRunner) error {
		runner.testSuite = suite
		return nil
	}
}

// translateEnv translates each hostname in the value of each environment
// variable based on the container names created by the runner. The resulting
// environment variables are returned.
func (c *ComposeRunner) translateEnv(variables []string) []string {
	newEnv := make([]string, len(variables))

	hosts := make([]string, 0, len(c.appDefinition)+len(c.driver))
	for k := range c.appDefinition {
		hosts = append(hosts, k)
	}
	for k := range c.driver {
		hosts = append(hosts, k)
	}

	for index, variable := range variables {
		before, after, _ := strings.Cut(variable, "=")

		for _, host := range hosts {
			re := regexp.MustCompile(fmt.Sprintf("\\b%s\\b", host))
			after = re.ReplaceAllString(after, fmt.Sprintf("%s-%s", c.Id(), host))
		}

		newEnv[index] = fmt.Sprintf("%s=%s", before, after)
	}
	return newEnv
}

// ResetApplication deletes the containers related to the currently running
// application and sets up the containers to run a provided application.
// If there is an error in the process, it is returned.
func (c *ComposeRunner) ResetApplication() error {
	if err := c.client.Delete(c.app); err != nil {
		return fmt.Errorf("app deletion failed in app reset:  %w", err)
	}

	instance, err := c.client.Run(c.appDefinition, docker.RunOptions{
		Prefix:   c.Id(),
		Networks: []string{c.network},
	})
	if err != nil {
		return fmt.Errorf("app start-up failed in app reset: %w", err)
	}
	c.app = instance
	log.Debugf("[runner=%s] successfully reset app", c.Id())

	return nil
}

// Delete releases all the resources allocated for the runner. If there is an
// error in the process, it is returned.
func (c *ComposeRunner) Delete() error {

	if err := c.client.Delete(c.driver); err != nil {
		return fmt.Errorf("driver deletion failed when deleting runner %s: %w", c.Id(), err)
	}
	log.Debugf("[runner=%s] successfully deleted driver", c.Id())

	if err := c.client.Delete(c.app); err != nil {
		return fmt.Errorf("app deletion failed when deleting runner %s: %w", c.Id(), err)
	}
	log.Debugf("[runner=%s] successfully deleted app", c.Id())

	if err := c.client.NetworkRemove(c.network); err != nil {
		return fmt.Errorf("network deletion failed when deleting runner %s: %w", c.Id(), err)
	}
	log.Debugf("[runner=%s] successfully deleted network", c.Id())
	_ = c.client.Close()

	return nil
}

// Run runs a test schedule on this runner. The test results are represented
// as booleans. If the test is passed, the value is true; otherwise it is
// false. If there is any error, it is returned.
func (c *ComposeRunner) Run(tests []string) ([]bool, error) {
	results, err := c.testSuite.Run(&testsuite.RunConfig{
		Name:        fmt.Sprintf("%s-testsuite", c.Id()),
		Env:         c.translatedEnv,
		Tests:       tests,
		StartConfig: &docker.RunOptions{Networks: []string{c.network}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run test suite on runner %s: %w", c.Id(), err)
	}
	return results, nil
}

func (c *ComposeRunner) Id() string {
	return c.id
}
