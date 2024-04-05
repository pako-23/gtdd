package testsuite

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pako-23/gtdd/internal/docker"
	log "github.com/sirupsen/logrus"
)

// TestSuite defines a Java test suite.
type JavaSeleniumTestSuite struct {
	// The name of the Docker image for the test suite.
	Image string
}

// Build produces the artifacts needed to run the Java test suite. It will
// create a Docker image on the host. If there is any error it is returned.
func (j *JavaSeleniumTestSuite) Build(path string) error {
	client, err := docker.NewClient()
	if err != err {
		return err
	}
	defer client.Close()

	return client.BuildImage(j.Image, path, "Dockerfile")
}

// ListTests returns the list of all tests declared into a Java test suite in
// the order in which they are run. If there is any error, it is returned.
func (j *JavaSeleniumTestSuite) ListTests() (tests []string, err error) {
	client, err := docker.NewClient()
	if err != err {
		return nil, err
	}
	defer client.Close()

	app := docker.App{
		"testsuite": {Command: []string{"--list-tests"}, Image: j.Image},
	}
	instance, err := client.Run(app, docker.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start Java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()

	logs, err := client.GetContainerLogs(instance["testsuite"])
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.Trim(logs, "\n"), "\n"), nil
}

// Run invokes the Java test suite with a given configuration and returns its
// results. The test results are represented as booleans. If the test
// is passed, the value is true; otherwise it is false. If there is
// any error, it is returned.
func (j *JavaSeleniumTestSuite) Run(config *RunConfig) (results []bool, err error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client to run Java test suite: %w", err)
	}

	suite := docker.App{
		config.Name: {
			Command:     config.Tests,
			Image:       j.Image,
			Environment: config.Env,
		},
	}

	instance, err := client.Run(suite, *config.StartConfig)
	if err != nil {
		return nil, fmt.Errorf("error in starting java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()
	log.Debugf("successfully started java test suite container %s", instance[config.Name])

	logs, err := client.GetContainerLogs(instance[config.Name])
	if err != nil {
		return nil, err
	}
	log.Debugf("successfully obtained logs from java test suite container %s", instance["testsuite"])
	log.Debugf("container logs: %s", logs)

	result := []bool{}
	r, _ := regexp.Compile("[a-zA-Z._0-9]+ (0|1)")
	lines := strings.Split(strings.Trim(logs, "\n"), "\n")
	for _, line := range lines[len(lines)-len(config.Tests):] {
		if r.MatchString(line) {
			result = append(result, line[len(line)-1] == '1')
		}
	}

	return result, nil
}
