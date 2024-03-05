package testsuite

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/pako-23/gtdd/internal/docker"
	log "github.com/sirupsen/logrus"
)

// TestSuite defines a Java test suite.
type PlaywrightTestSuite struct {
	// The name of the Docker image for the test suite.
	Image string
}

// Build produces the artifacts needed to run the Java test suite. It will
// create a Docker image on the host. If there is any error it is returned.
func (j *PlaywrightTestSuite) Build(path string) error {
	return docker.BuildDockerImage(j.Image, path, "Dockerfile")
}

// ListTests returns the list of all tests declared into a Java test suite in
// the order in which they are run. If there is any error, it is returned.
func (j *PlaywrightTestSuite) ListTests() ([]string, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create client to list Java test suite tests: %w", err)
	}
	defer cli.Close()
	ctx := context.Background()

	app := docker.App{
		"testsuite": {Command: []string{"--list"}, Image: j.Image},
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

	fmt.Println(logs)

	return []string{}, nil
}

// Run invokes the Java test suite with a given configuration and returns its
// results. The test results are represented as booleans. If the test
// is passed, the value is true; otherwise it is false. If there is
// any error, it is returned.
func (j *PlaywrightTestSuite) Run(config *RunConfig) ([]bool, error) {
	return []bool{}, nil
}
