package testsuite

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pako-23/gtdd/compose"
	log "github.com/sirupsen/logrus"
)

type RunConfig struct {
	Env         []string
	Tests       []string
	StartConfig *compose.StartConfig
}

// TestSuite defines the interface for a generic test suite
type TestSuite interface {
	// Builds the artifacts necessary to run the test suite given the
	// path to test suite.
	Build(string) error
	// Returns the list of tests declared into the test suite.
	ListTests() ([]string, error)
	// Runs the test suite returning the results of the single tests.
	Run(*RunConfig) ([]bool, error)
}

// buildDockerImage builds a Docker image given the context and the image
// name. If there is any error in the build process, it is returned.
func buildDockerImage(dockerContext, imageName string) error {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx := context.Background()

	log.Debugf("Starting build for Docker image %s", imageName)
	tar, err := archive.TarWithOptions(dockerContext, &archive.TarOptions{})
	if err != nil {
		return err
	}

	res, err := cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Dockerfile:     "Dockerfile",
		Tags:           []string{imageName},
		Remove:         true,
		SuppressOutput: true,
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// TODO: Check when the image fails the build
	io.Copy(io.Discard, res.Body)

	log.Infof("Successfully built Docker image %s", imageName)
	return nil
}

// getContainerLogs returns the logs from a given container. If there is any
// error in retrieving the logs, it is returned
func getContainerLogs(ctx context.Context, cli *client.Client, containerId string) (string, error) {
	statusCh, errCh := cli.ContainerWait(ctx, containerId, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return "", err
	}
	defer out.Close()

	var stdout, stderr bytes.Buffer

	if _, err := stdcopy.StdCopy(&stdout, &stderr, out); err != nil {
		return "", err
	} else if stderr.Len() > 0 {
		return "", fmt.Errorf(stderr.String())
	}

	return stdout.String(), nil
}

// TestSuiteFactory creates the correct test suite from a given type.
// If the type is not recognized, an error is returned.
func TestSuiteFactory(testSuiteType string) (TestSuite, error) {
	switch testSuiteType {
	case "java":
		return &JavaTestSuite{Image: "testsuite"}, nil
	default:
		err := fmt.Errorf("Not a supported test-suite type: %s", testSuiteType)
		return nil, err
	}
}
