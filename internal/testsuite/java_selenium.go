package testsuite

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	cgo "github.com/compose-spec/compose-go/types"

	"github.com/docker/docker/client"
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
	return docker.BuildDockerImage(j.Image, path, "Dockerfile")
}

// ListTests returns the list of all tests declared into a Java test suite in
// the order in which they are run. If there is any error, it is returned.
func (j *JavaSeleniumTestSuite) ListTests() ([]string, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to list Java test suite tests: %w", err)
	}
	defer cli.Close()
	ctx := context.Background()

	app := docker.App{
		"testsuite": {Command: []string{"--list-tests"}, Image: j.Image},
	}
	instance, err := app.Start(&docker.StartConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to start Java test suite container: %w", err)
	}
	defer func() {
		if err := instance.Delete(); err != nil {
			log.Error(err)
		}
	}()

	logs, err := docker.GetContainerLogs(ctx, cli, instance["testsuite"])
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.Trim(logs, "\n"), "\n"), nil
}

// Run invokes the Java test suite with a given configuration and returns its
// results. The test results are represented as booleans. If the test
// is passed, the value is true; otherwise it is false. If there is
// any error, it is returned.
func (j *JavaSeleniumTestSuite) Run(config *RunConfig) ([]bool, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to run Java test suite: %w", err)
	}
	defer cli.Close()
	ctx := context.Background()

	env := cgo.MappingWithEquals{}
	for _, variable := range config.Env {
		env[variable] = nil
	}

	suite := docker.App{
		"testsuite": {
			Command:     config.Tests,
			Image:       j.Image,
			Environment: env,
		},
	}
	instance, err := suite.Start(config.StartConfig)
	if err != nil {
		return nil, fmt.Errorf("error in starting java test suite container: %w", err)
	}
	defer func() {
		if err := instance.Delete(); err != nil {
			log.Error(err)
		}
	}()
	log.Debugf("successfully started java test suite container %s", instance["testsuite"])

	logs, err := docker.GetContainerLogs(ctx, cli, instance["testsuite"])
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
