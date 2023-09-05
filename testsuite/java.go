package testsuite

import (
	"context"
	"strings"

	"github.com/docker/docker/client"
	"github.com/pako-23/gtdd/compose"
	log "github.com/sirupsen/logrus"
)

// TestSuite defines a Java test suite.
type JavaTestSuite struct {
	// The name of the Docker image for the test suite.
	Image string
}

// Builds the artifacts needed to run the Java test suite.
func (j *JavaTestSuite) Build(path string) error {
	return buildDockerImage(path, j.Image)
}

// ListTests returns the tests contained into a Java test suite, If there
// is an error, it is returned.
func (j *JavaTestSuite) ListTests() ([]string, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	ctx := context.Background()

	app := compose.App{Services: map[string]*compose.Service{
		"testsuite": {Cmd: []string{"--list-tests"}, Image: j.Image},
	}}
	instance, err := app.Start(&compose.StartConfig{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := instance.Delete(); err != nil {
			log.Error(err)
		}
	}()

	logs, err := getContainerLogs(ctx, cli, instance["testsuite"])
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.Trim(logs, "\n"), "\n"), nil
}

// ListTests returns the tests contained into a Java test suite, If there
// is an error, it is returned.
func (j *JavaTestSuite) Run(config *RunConfig) ([]bool, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	ctx := context.Background()

	suite := compose.App{Services: map[string]*compose.Service{
		"testsuite": {
			Cmd:   config.Tests,
			Image: j.Image,
			Env:   config.Env,
		},
	}}
	instance, err := suite.Start(config.StartConfig)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := instance.Delete(); err != nil {
			log.Error(err)
		}
	}()

	logs, err := getContainerLogs(ctx, cli, instance["testsuite"])
	if err != nil {
		return nil, err
	}

	result := []bool{}

	for _, line := range strings.Split(strings.Trim(logs, "\n"), "\n") {
		if len(line) > 0 {
			result = append(result, line[len(line)-1] == '1')
		}
	}

	return result, nil
}
